package discovery_service

import (
	"fmt"
	"net/url"

	trafficdirector "github.com/streamingfast/dgrpc/discovery-service/traffic-director"
)

func Bootstrap(u *url.URL) error {
	switch u.Scheme {
	case "traffic-director":
		return trafficdirector.Bootstrap(u)
	default:
		return fmt.Errorf("unsupported service discovery scheme %q", u.Scheme)
	}
}
