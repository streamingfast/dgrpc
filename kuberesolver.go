package dgrpc

import (
	"os"

	"github.com/sercand/kuberesolver/v5"
	"google.golang.org/grpc/resolver"
)

func init() {
	if v := os.Getenv("GRPC_REGISTER_KUBERNETES_RESOLVER"); v != "false" {
		RegisterKubernetesResolver("kubernetes")
	}
}

// RegisterKubernetesResolver registers the "kubernetes" resolver with the given scheme. Refers
// to github.com/sercand/kuberesolver/v5 for more information about the `kubernetes:///` gRPC
// resolver.
//
// **Important** The 'dgrpc' library already registers the kubernetes resolver with the 'kubernetes' scheme.
// so it's not necessary to call this function unless you want to use a different scheme. You can define
// the variable `GRPC_REGISTER_KUBERNETES_RESOLVER` to `false` to prevent the automatic registration.
// if needed.
func RegisterKubernetesResolver(customScheme string) {
	resolver.Register(kuberesolver.NewBuilder(nil, customScheme))
}
