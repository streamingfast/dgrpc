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

	logger := options.Logger
	if logger == nil {
		logger = zlog
	}

	if options.ServiceDiscoveryURL != nil {
		u := options.ServiceDiscoveryURL
		switch u.Scheme {
		case "traffic-director", "xds":
			clientOnly := strings.ToUpper(u.Query().Get("client_only")) == "TRUE"
			if !clientOnly {
				logger.Info("launching traffic director base server", zap.Stringer("url", u))
				return traffic_director.NewServer(options)
			}

			logger.Info("traffic director url is for client only")
		}
	}

	logger.Info("standard server created")
	return standard.NewServer(options)
}
