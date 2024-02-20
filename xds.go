package dgrpc

import (
	"os"
	"strings"
)

// IsXDSRemoteAddr returns true if the remote address is an xDS address
// (i.e. starts with "xds://").
func IsXDSRemoteAddr(remoteAddr string) bool {
	return strings.HasPrefix(remoteAddr, "xds://")
}

// GetXDSBootstrapFilename returns the filename of the xDS bootstrap file
// which is currently simply returning the value of the 'GRPC_XDS_BOOTSTRAP'
// environment variable.
//
// We might add more logic to this function in the future.
func GetXDSBootstrapFilename() string {
	return os.Getenv("GRPC_XDS_BOOTSTRAP")
}
