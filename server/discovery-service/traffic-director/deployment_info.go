package traffic_director

import (
	"net/http"
	"net/url"
	"time"
)

type deploymentType int

const (
	deploymentTypeUnknown deploymentType = iota
	deploymentTypeGKE
	deploymentTypeGCE
)

// getDeploymentType tries to talk the metadata trafficDirectorServer at
// http://metadata.google.internal and uses a response header with key "TrafficDirectorServer"
// to determine the deployment type.
func getDeploymentType() deploymentType {
	parsedUrl, err := url.Parse("http://metadata.google.internal")
	if err != nil {
		return deploymentTypeUnknown
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req := &http.Request{
		Method: "GET",
		URL:    parsedUrl,
		Header: http.Header{"Metadata-Flavor": {"Google"}},
	}
	resp, err := client.Do(req)
	if err != nil {
		return deploymentTypeUnknown
	}
	resp.Body.Close()

	// Read the "TrafficDirectorServer" header to determine the deployment type.
	vals := resp.Header.Values("TrafficDirectorServer")
	for _, val := range vals {
		switch val {
		case "GKE Metadata TrafficDirectorServer":
			return deploymentTypeGKE
		case "Metadata TrafficDirectorServer for VM":
			return deploymentTypeGCE
		}
	}
	return deploymentTypeUnknown
}
