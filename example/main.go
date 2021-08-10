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

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/streamingfast/derr"
	"github.com/dfuse-io/dgrpc"
	pbhealth "github.com/dfuse-io/dgrpc/example/pb/grpc/health/v1"
	"github.com/streamingfast/dtracing"
	"github.com/dfuse-io/logging"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
)

func setupTracing() {
	err := dtracing.SetupTracing("example", trace.ProbabilitySampler(1/8.0))
	derr.Check("unable to setup tracing correctly", err)
}
var zlog *zap.Logger

func setupLogger() {
	logging.Register("github.com/dfuse-io/dgrpc/example/", &zlog)
	logging.Set(logging.MustCreateLoggerWithServiceName("drgpc-example"))
}

func main() {
	setupLogger()
	setupTracing()

	srv := dgrpc.NewServer()
	h := &healthy{}

	pbhealth.RegisterHealthServer(srv, h)
	lis, err := net.Listen("tcp", ":9001")
	if err != nil {
		derr.Check("failed listening grpc", err)
	}
	go srv.Serve(lis)

	conn, err := dgrpc.NewInternalClient("betterdns:///127.0.0.2:9001")
	if err != nil {
		zlog.Error("cannot get grpc client", zap.Error(err))
		zlog.Sync()
		os.Exit(1)
	}

	cli := pbhealth.NewHealthClient(conn)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		time.Sleep(1 * time.Second)
		resp, err := cli.Check(ctx, &pbhealth.HealthCheckRequest{})
		fmt.Println(resp, err)
	}

}

type healthy struct{}

func (h *healthy) Check(ctx context.Context, in *pbhealth.HealthCheckRequest) (*pbhealth.HealthCheckResponse, error) {
	return &pbhealth.HealthCheckResponse{
		Status: pbhealth.HealthCheckResponse_SERVING,
	}, nil
}
