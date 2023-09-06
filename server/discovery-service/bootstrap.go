package discovery_service

import (
	"fmt"
	"os"

	"net/url"

	trafficdirector "github.com/streamingfast/dgrpc/server/discovery-service/traffic-director"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

var zlog, _ = logging.PackageLogger("dgrpc", "github.com/streamingfast/dgrpc/discovery-service")

// Bootstrap generates an xds configuration file to allow the grpc server to connect to a service mesh
// using the discovery service URL provided. Supported schemes are:
// traffic-director:// - Google Cloud Platform specific. Ex: traffic-director://xds?vpc_network=vpc-global&use_xds_creds=true
// xds:// - use configuration file provided by env var GRPC_XDS_BOOTSTRAP, Ex: xds://xds?use_xds_creds=true
func Bootstrap(u *url.URL) error {
	switch u.Scheme {
	case "traffic-director":
		return trafficdirector.Bootstrap(u)
	case "xds":
		bootStrapFilename := os.Getenv("GRPC_XDS_BOOTSTRAP")
		zlog.Info("looked for GRPC_XDS_BOOTSTRAP", zap.String("filename", bootStrapFilename))
		if bootStrapFilename == "" {
			return fmt.Errorf("GRPC_XDS_BOOTSTRAP environment var must be set when using traffic director")
		}
		return nil

	default:
		return fmt.Errorf("unsupported service discovery scheme %q", u.Scheme)
	}
}
