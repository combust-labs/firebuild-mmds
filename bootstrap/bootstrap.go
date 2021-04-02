package bootstrap

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/combust-labs/firebuild-mmds/mmds"
	"github.com/combust-labs/firebuild-shared/build/commands"
	"github.com/combust-labs/firebuild-shared/build/rootfs"
	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
)

// DoBootstrap executes the bootstrap sequence on the machine.
func DoBootstrap(logger hclog.Logger, bootstrapData *mmds.MMDSBootstrap) error {
	clientTLSConfig, err := getTLSConfig(bootstrapData)
	if err != nil {
		logger.Error("failed creating client TLS config", "reason", err)
		return err
	}

	clientConfig := &rootfs.GRPCClientConfig{
		HostPort:       bootstrapData.HostPort,
		TLSConfig:      clientTLSConfig,
		MaxRecvMsgSize: rootfs.DefaultMaxRecvMsgSize,
	}

	client, err := rootfs.NewClient(logger.Named("grpc-client"), clientConfig)
	if err != nil {
		logger.Error("failed constructing gRPC client", "reason", err)
		return err
	}

	if err := client.Commands(); err != nil {
		logger.Error("failed fetching bootstrap commands over gRPC", "reason", err)
		return err
	}

	for {
		serializableCommand := client.NextCommand()
		if serializableCommand == nil {
			break // finished
		}

		switch vCommand := serializableCommand.(type) {
		case commands.Run:
			logger.Info("RUN command", "paylod", vCommand)
		case commands.Add:
			logger.Info("ADD command", "paylod", vCommand)
		case commands.Copy:
			logger.Info("COPY command", "paylod", vCommand)
		}
	}

	// TODO: this is where the command execution goes

	return fmt.Errorf("not implemented")
}

func getTLSConfig(bootstrapData *mmds.MMDSBootstrap) (*tls.Config, error) {
	roots := x509.NewCertPool()
	input := []byte(bootstrapData.Certificate)
	for {
		block, remaning := pem.Decode(input)
		if block == nil {
			break
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing certificate")
		}
		roots.AddCert(cert)
		input = remaning
	}

	ok := roots.AppendCertsFromPEM([]byte(bootstrapData.CaChain))
	if !ok {
		return nil, fmt.Errorf("failed appending root to the cert pool")
	}

	block, _ := pem.Decode([]byte(bootstrapData.Certificate))
	if block == nil {
		return nil, fmt.Errorf("failed to parse certificate PEM")
	}

	tlsCert, err := tls.X509KeyPair([]byte(bootstrapData.Certificate), []byte(bootstrapData.Key))
	if err != nil {
		return nil, errors.Wrap(err, "failed loading TLS certificate")
	}

	return &tls.Config{
		ServerName:   bootstrapData.ServerName,
		RootCAs:      roots,
		Certificates: []tls.Certificate{tlsCert},
	}, nil
}
