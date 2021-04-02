package bootstrap

import (
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/combust-labs/firebuild-embedded-ca/ca"
	"github.com/combust-labs/firebuild-mmds/mmds"
	"github.com/hashicorp/go-hclog"
)

func TestBootstrap(t *testing.T) {

	t.Fatal("not implemented")

}

func TestGetTLSConfig(t *testing.T) {

	logger := hclog.Default()
	logger.SetLevel(hclog.Debug)

	embeddedCAConfig := &ca.EmbeddedCAConfig{
		Addresses:     []string{"test-app"},
		CertsValidFor: time.Hour,
		KeySize:       1024,
	}

	embeddedCA, err := ca.NewDefaultEmbeddedCAWithLogger(embeddedCAConfig, logger.Named("embedded-ca"))
	if err != nil {
		t.Fatal("failed constructing embedded CA", err)
	}

	clientCertData, err := embeddedCA.NewClientCert()
	if err != nil {
		t.Fatal("failed creating test client certitifcate", err)
	}

	bootstrapConfig := &mmds.MMDSBootstrap{
		HostPort:    "127.0.0.1:0",
		CaChain:     strings.Join(embeddedCA.CAPEMChain(), "\n"),
		Certificate: string(clientCertData.CertificatePEM()),
		Key:         string(clientCertData.KeyPEM()),
		ServerName:  "irrelevant",
	}

	_, tlsConfigErr := getTLSConfig(bootstrapConfig)
	if tlsConfigErr != nil {
		t.Fatal("expected TLS config, got error", tlsConfigErr)
	}

}

func mustPutTestResource(t *testing.T, path string, contents []byte) {
	if err := os.MkdirAll(filepath.Dir(path), fs.ModePerm); err != nil {
		t.Fatal("failed creating parent directory for the resource, got error", err)
	}
	if err := ioutil.WriteFile(path, contents, fs.ModePerm); err != nil {
		t.Fatal("expected resource to be written, got error", err)
	}
}

const testDockerfileMultiStage = `FROM alpine:3.13 as builder

FROM alpine:3.13
ARG PARAM1=value
ENV ENVPARAM1=envparam1
RUN mkdir -p /dir
ADD resource1 /target/resource1
COPY resource2 /target/resource2
COPY --from=builder /etc/test /etc/test
RUN cp /dir/${ENVPARAM1} \
	&& call --arg=${PARAM1}`
