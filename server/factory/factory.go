package factory

import (
	"strings"

	"github.com/streamingfast/dgrpc/server"
	traffic_director "github.com/streamingfast/dgrpc/server/discovery-service/traffic-director"
	"github.com/streamingfast/dgrpc/server/standard"
	"go.uber.org/zap"
)

func ServerFromOptions(opts ...server.Option) server.Server {
	options := server.NewOptions()
	for _, opt := range opts {
		opt(options)
	}

	if options.ServiceDiscoveryURL != nil {
		u := options.ServiceDiscoveryURL
		switch u.Scheme {
		case "traffic-director":
			clientOnly := strings.ToUpper(u.Query().Get("client_only")) == "TRUE"
			if !clientOnly {
				zlog.Info("launching traffic director base server", zap.Stringer("url", u))
				return traffic_director.NewServer(options)
			}
			zlog.Info("traffic director url is for client only")
		}
	}

	zlog.Info("standard server created")
	return standard.NewServer(options)
}
