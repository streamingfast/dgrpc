package dgrpc

import (
	"github.com/sercand/kuberesolver/v5"
	"google.golang.org/grpc/resolver"
)

func RegisterKubernetesResolver() {
	resolver.Register(kuberesolver.NewBuilder(nil, "kubernetes"))
}
