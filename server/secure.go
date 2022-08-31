package server

import (
	"crypto/tls"
	"fmt"

	"github.com/streamingfast/dgrpc/insecure"
)

type SecureTLSConfig interface {
	asTLSConfig() *tls.Config
}

type secureTLSConfigWrapper tls.Config

func (c *secureTLSConfigWrapper) asTLSConfig() *tls.Config {
	return (*tls.Config)(c)
}

// SecuredByX509KeyPair creates a SecureTLSConfig by loading the provided public/private
// X509 pair (`.pem` files format).
func SecuredByX509KeyPair(publicCertFile, privateKeyFile string) (SecureTLSConfig, error) {
	serverCert, err := tls.LoadX509KeyPair(publicCertFile, privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load X509 key pair files: %w", err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert,
	}

	return (*secureTLSConfigWrapper)(config), nil
}

// SecuredByBuiltInSelfSignedCertificate creates a SecureTLSConfig that uses the
// built-in hard-coded certificate found in package `insecure`
// (path `github.com/streamingfast/dgrpc/insecure`).
//
// This certificate is self-signed and distributed publicly over the internet, so
// it's not a safe certificate, can be seen as compromised.
//
// **Never** uses that in a production environment.
func SecuredByBuiltInSelfSignedCertificate() SecureTLSConfig {
	return (*secureTLSConfigWrapper)(&tls.Config{
		Certificates: []tls.Certificate{insecure.Cert},
		ClientCAs:    insecure.CertPool,
		ClientAuth:   tls.VerifyClientCertIfGiven,
	})
}
