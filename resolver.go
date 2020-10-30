// Copyright 2019 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dgrpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/resolver"
)

var (
	errMissingAddr = errors.New("missing address")
	defaultPort    = "9000"
)

func init() {
	registerBetterDNSResolver("betterdns")
}

func registerBetterDNSResolver(scheme string) {
	r := &Resolver{
		scheme: scheme,
		rn:     make(chan struct{}, 1),
	}
	resolver.Register(r)
}

// Resolver works by launching a ConnStateManager function when Build() is called.
type Resolver struct {
	scheme        string
	cancelContext context.CancelFunc
	rn            chan struct{}
}

func parseTarget(target string) (host, port string, err error) {
	if target == "" {
		return "", "", errMissingAddr
	}
	if ip := net.ParseIP(target); ip != nil {
		return target, defaultPort, nil
	}
	if host, port, err := net.SplitHostPort(target); err == nil {
		if host == "" {
			host = "localhost"
		}
		if port == "" {
			port = defaultPort
		}
		return host, port, nil
	}
	if host, port, err := net.SplitHostPort(target + ":" + defaultPort); err == nil {
		return host, port, nil
	}
	return "", "", fmt.Errorf("invalid target address %v", target)
}

func sameAddresses(a1, a2 []resolver.Address) bool {
	a1str := []string{}
	a2str := []string{}
	for _, a := range a1 {
		a1str = append(a1str, a.Addr)
	}
	for _, a := range a2 {
		a2str = append(a2str, a.Addr)
	}
	sort.Strings(a1str)
	sort.Strings(a2str)

	if len(a1str) == len(a2str) {
		for i, v := range a1str {
			if v != a2str[i] {
				return false
			}
		}
	} else {
		return false
	}
	return true

}

func manageConnections(ctx context.Context, target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions, rn chan struct{}) {
	firstTime := true
	lastAddresses := []resolver.Address{}
	var forceUpdate bool

	for {
		if !firstTime {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second * 5):
			case <-rn:
				forceUpdate = true
			}
		}
		firstTime = false

		addresses := []resolver.Address{}
		host, port, err := parseTarget(target.Endpoint)
		if err != nil {
			zlog.Error("cannot parse target endpoint, invalid format", zap.String("target_endpoint", target.Endpoint))
			panic(err)
		}

		ips, err := net.LookupIP(host)
		if err != nil {
			zlog.Warn("cannot resolve grpc endpoint", zap.String("target_endpoint", target.Endpoint))
			time.Sleep(time.Second * 5)
			continue
		}
		for _, ip := range ips {
			addresses = append(addresses, resolver.Address{
				Addr: ip.String() + ":" + port,
				Type: resolver.Backend,
			})
		}

		if !sameAddresses(addresses, lastAddresses) || forceUpdate {
			zlog.Debug("updating resolver state", zap.Any("addresses", addresses))
			cc.UpdateState(resolver.State{
				Addresses: addresses,
			})
			lastAddresses = addresses
			forceUpdate = false
		}
	}
}

func (r *Resolver) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancelContext = cancel

	go manageConnections(ctx, target, cc, opts, r.rn)
	return r, nil
}

func (r *Resolver) Scheme() string {
	return r.scheme
}

// ResolveNow is a noop for Resolver.
func (r *Resolver) ResolveNow(resolver.ResolveNowOptions) {
	select {
	case r.rn <- struct{}{}:
	default:
	}
}

// Close will close the context
func (r *Resolver) Close() {
	r.cancelContext()
}
