package store

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	"os"

	"gitlab.com/learnt/api/pkg/logger"
)

// ConfigureCert returns tls configured struct
func ConfigureCert(file *os.File, insecure bool) (config *tls.Config, err error) {
	if insecure {
		config = &tls.Config{
			InsecureSkipVerify: true,
		}
		return
	}
	roots := x509.NewCertPool()
	logger.Get().Info("|⣿⣿⣿⣿|⣿⣿⣿⣿")

	var bytes []byte
	if bytes, err = ioutil.ReadFile(file.Name()); err != nil {
		return
	}

	if roots.AppendCertsFromPEM(bytes) == false {
		logger.Get().Errorf("failed to append certs ... %#v", err)
	}
	config = &tls.Config{
		InsecureSkipVerify: true,
		RootCAs:            roots,
	}
	return
}
