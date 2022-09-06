package traffic_director

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type configInput struct {
	xdsServerUri           string
	gcpProjectNumber       int64
	vpcNetworkName         string
	ip                     string
	zone                   string
	ignoreResourceDeletion bool
	includeV3Features      bool
	includePSMSecurity     bool
	secretsDir             string
	metadataLabels         map[string]string
	deploymentInfo         map[string]string
	configMesh             string
}

func Bootstrap(u *url.URL) error {
	bootStrapFilename := os.Getenv("GRPC_XDS_BOOTSTRAP")
	zlog.Info("looked for GRPC_XDS_BOOTSTRAP", zap.String("filename", bootStrapFilename))

	if bootStrapFilename != "" {
		return fmt.Errorf("GRPC_XDS_BOOTSTRAP environment var must be set when using traffic director")
	}

	vpcNetworkName := u.Query().Get("vpc_network")
	if vpcNetworkName == "" {
		return fmt.Errorf("vpc_network must be set when bootstraping traffic director")
	}

	zlog.Info("generating bootstrap file", zap.String("filename", bootStrapFilename))
	err := generateBootstrapFile("trafficdirector.googleapis.com:443", vpcNetworkName, bootStrapFilename)
	if err != nil {
		panic(fmt.Sprintf("failed to generate bootstrap file: %v", err))
	}

	return nil
}

func generateBootstrapFile(xdsServerUri string, vpcNetworkName string, outputFilePath string) error {
	if xdsServerUri == "" {
		return fmt.Errorf("xdsServerUri must be set")
	}

	if vpcNetworkName == "" {
		return fmt.Errorf("vpcNetworkName must be set")
	}

	gcpProjectNumber, err := getProjectId()
	if err != nil {
		return fmt.Errorf("could not discover GCP project number: %w", err)
	}
	nodeMetadata := make(map[string]string)
	ip, err := getHostIp()
	if err != nil {
		return fmt.Errorf("could not discover host IP: %w", err)
	}
	// Retrieve zone from the metadata server only if not specified in args.
	zone, err := getZone()
	if err != nil {
		return fmt.Errorf("could not discover GCP zone: %w", err)
	}

	// Generate deployment info from metadata server or from command-line
	// arguments, with the latter taking preference.
	var deploymentInfo map[string]string
	dType := getDeploymentType()
	switch dType {
	case deploymentTypeGKE:
		cluster := getClusterName()
		pod := getPodName()
		deploymentInfo = map[string]string{
			"GKE-CLUSTER":   cluster,
			"GCP-ZONE":      zone,
			"INSTANCE-IP":   ip,
			"GKE-POD":       pod,
			"GKE-NAMESPACE": "",
		}
	case deploymentTypeGCE:
		vmName := getVMName()
		deploymentInfo = map[string]string{
			"GCE-VM":      vmName,
			"GCP-ZONE":    zone,
			"INSTANCE-IP": ip,
		}
	}

	input := configInput{
		xdsServerUri:           xdsServerUri,
		gcpProjectNumber:       gcpProjectNumber,
		vpcNetworkName:         vpcNetworkName,
		ip:                     ip,
		zone:                   zone,
		ignoreResourceDeletion: false,
		includeV3Features:      true,
		includePSMSecurity:     true,
		secretsDir:             "/var/run/secrets/workload-spiffe-credentials",
		metadataLabels:         nodeMetadata,
		deploymentInfo:         deploymentInfo,
		configMesh:             "",
	}

	if err := validate(input); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	config, err := generate(input)
	if err != nil {
		return fmt.Errorf("generating config: %w", err)
	}
	output, err := os.Create(outputFilePath)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	_, err = output.Write(config)
	if err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	_, err = output.Write([]byte("\n"))
	if err != nil {
		return fmt.Errorf("writing \\n to file: %w", err)
	}
	err = output.Close()
	if err != nil {
		return fmt.Errorf("closing config file: %w", err)
	}
	return nil
}
func generate(in configInput) ([]byte, error) {
	c := &config{
		XdsServers: []server{
			{
				ServerUri: in.xdsServerUri,
				ChannelCreds: []creds{
					{Type: "google_default"},
				},
			},
		},
		Node: &node{
			Id:      uuid.New().String() + "~" + in.ip,
			Cluster: "cluster", // unused by TD
			Locality: &locality{
				Zone: in.zone,
			},
			Metadata: map[string]interface{}{
				"TRAFFICDIRECTOR_NETWORK_NAME":       in.vpcNetworkName,
				"TRAFFICDIRECTOR_GCP_PROJECT_NUMBER": strconv.FormatInt(in.gcpProjectNumber, 10),
			},
		},
	}

	for k, v := range in.metadataLabels {
		c.Node.Metadata[k] = v
	}
	if in.includeV3Features {
		// xDS v2 implementation in TD expects the projectNumber and networkName in
		// the metadata field while the v3 implementation expects these in the id
		// field.
		networkIdentifier := in.vpcNetworkName
		if in.configMesh != "" {
			networkIdentifier = fmt.Sprintf("mesh:%s", in.configMesh)
		}

		c.Node.Id = fmt.Sprintf("projects/%d/networks/%s/nodes/%s", in.gcpProjectNumber, networkIdentifier, uuid.New().String())
		// xDS v2 implementation in TD expects the IP address to be encoded in the
		// id field while the v3 implementation expects this in the metadata.
		c.Node.Metadata["INSTANCE_IP"] = in.ip
		c.XdsServers[0].ServerFeatures = append(c.XdsServers[0].ServerFeatures, "xds_v3")
	}
	if in.includePSMSecurity {
		c.CertificateProviders = map[string]certificateProviderConfig{
			"google_cloud_private_spiffe": {
				PluginName: "file_watcher",
				Config: privateSPIFFEConfig{
					CertificateFile:   path.Join(in.secretsDir, "certificates.pem"),
					PrivateKeyFile:    path.Join(in.secretsDir, "private_key.pem"),
					CACertificateFile: path.Join(in.secretsDir, "ca_certificates.pem"),
					// The file_watcher plugin will parse this a Duration proto, but it is totally
					// fine to just emit a string here.
					RefreshInterval: "600s",
				},
			},
		}
		c.ServerListenerResourceNameTemplate = "grpc/server?xds.resource.listening_address=%s"
	}
	if in.ignoreResourceDeletion {
		c.XdsServers[0].ServerFeatures = append(c.XdsServers[0].ServerFeatures, "ignore_resource_deletion")
	}
	if in.deploymentInfo != nil {
		c.Node.Metadata["TRAFFIC_DIRECTOR_CLIENT_ENVIRONMENT"] = in.deploymentInfo
	}

	return json.MarshalIndent(c, "", "  ")
}

func getHostIp() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return "", err
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("no addresses found for hostname: %s", hostname)
	}
	return addrs[0], nil
}

func getZone() (string, error) {
	qualifiedZone, err := getFromMetadata("http://metadata.google.internal/computeMetadata/v1/instance/zone")
	if err != nil {
		return "", fmt.Errorf("could not discover instance zone: %w", err)
	}
	i := bytes.LastIndexByte(qualifiedZone, '/')
	if i == -1 {
		return "", fmt.Errorf("could not parse zone from metadata server: %s", qualifiedZone)
	}
	return string(qualifiedZone[i+1:]), nil
}

func getProjectId() (int64, error) {
	projectIdBytes, err := getFromMetadata("http://metadata.google.internal/computeMetadata/v1/project/numeric-project-id")
	if err != nil {
		return 0, fmt.Errorf("could not discover project id: %w", err)
	}
	projectId, err := strconv.ParseInt(string(projectIdBytes), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("could not parse project id from metadata server: %w", err)
	}
	return projectId, nil
}

func getClusterName() string {
	cluster, err := getFromMetadata("http://metadata.google.internal/computeMetadata/v1/instance/attributes/cluster-name")
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not discover GKE cluster name: %v", err)
		return ""
	}
	return string(cluster)
}

// For overriding in unit tests.
var readHostNameFile = func() ([]byte, error) {
	return ioutil.ReadFile("/etc/hostname")
}

func getPodName() string {
	pod, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not discover GKE pod name: %v", err)
	}
	return pod
}

func getVMName() string {
	vm, err := getFromMetadata("http://metadata.google.internal/computeMetadata/v1/instance/name")
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not discover GCE VM name: %v", err)
		return ""
	}
	return string(vm)
}

func getFromMetadata(urlStr string) ([]byte, error) {
	parsedUrl, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	req := &http.Request{
		Method: "GET",
		URL:    parsedUrl,
		Header: http.Header{
			"Metadata-Flavor": {"Google"},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed communicating with metadata server: %w", err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed reading from metadata server: %w", err)
	}
	return body, nil
}

func validate(in configInput) error {
	re := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]{0,63}$`)
	if in.configMesh != "" && !re.MatchString(in.configMesh) {
		return fmt.Errorf("config-mesh may only contain letters, numbers, and '-'. It must begin with a letter and must not exceed 64 characters in length")
	}

	return nil
}

type config struct {
	XdsServers                         []server                             `json:"xds_servers,omitempty"`
	Node                               *node                                `json:"node,omitempty"`
	CertificateProviders               map[string]certificateProviderConfig `json:"certificate_providers,omitempty"`
	ServerListenerResourceNameTemplate string                               `json:"server_listener_resource_name_template,omitempty"`
}

type server struct {
	ServerUri      string   `json:"server_uri,omitempty"`
	ChannelCreds   []creds  `json:"channel_creds,omitempty"`
	ServerFeatures []string `json:"server_features,omitempty"`
}

type creds struct {
	Type   string      `json:"type,omitempty"`
	Config interface{} `json:"config,omitempty"`
}

type node struct {
	Id           string                 `json:"id,omitempty"`
	Cluster      string                 `json:"cluster,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Locality     *locality              `json:"locality,omitempty"`
	BuildVersion string                 `json:"build_version,omitempty"`
}

type locality struct {
	Region  string `json:"region,omitempty"`
	Zone    string `json:"zone,omitempty"`
	SubZone string `json:"sub_zone,omitempty"`
}

type certificateProviderConfig struct {
	PluginName string      `json:"plugin_name,omitempty"`
	Config     interface{} `json:"config,omitempty"`
}

type privateSPIFFEConfig struct {
	CertificateFile   string `json:"certificate_file,omitempty"`
	PrivateKeyFile    string `json:"private_key_file,omitempty"`
	CACertificateFile string `json:"ca_certificate_file,omitempty"`
	RefreshInterval   string `json:"refresh_interval,omitempty"`
}
