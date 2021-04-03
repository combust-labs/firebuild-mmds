package bootstrap

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/combust-labs/firebuild-mmds/mmds"
	"github.com/combust-labs/firebuild-shared/build/commands"
	"github.com/combust-labs/firebuild-shared/build/rootfs"
	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
)

type Bootstrapper interface {
	Execute() error
	WithCommandRunner(CommandRunner) Bootstrapper
	WithResourceDeployer(ResourceDeployer) Bootstrapper
}

type defaultBootstrapper struct {
	commandRunner    CommandRunner
	bootstrapData    *mmds.MMDSBootstrap
	logger           hclog.Logger
	resourceDeployer ResourceDeployer
}

func NewDefaultBoostrapper(logger hclog.Logger, bootstrapData *mmds.MMDSBootstrap) Bootstrapper {
	return &defaultBootstrapper{
		commandRunner:    &noopCommandRunner{logger: logger.Named("noop-runner")},
		bootstrapData:    bootstrapData,
		logger:           logger,
		resourceDeployer: &noopResourceDeployer{logger: logger.Named("noo-deployer")},
	}
}

// DoBootstrap executes the bootstrap sequence on the machine.
func (b *defaultBootstrapper) Execute() error {
	clientTLSConfig, err := getTLSConfig(b.bootstrapData)
	if err != nil {
		b.logger.Error("failed creating client TLS config", "reason", err)
		return err
	}

	clientConfig := &rootfs.GRPCClientConfig{
		HostPort:       b.bootstrapData.HostPort,
		TLSConfig:      clientTLSConfig,
		MaxRecvMsgSize: rootfs.DefaultMaxRecvMsgSize,
	}

	client, err := rootfs.NewClient(b.logger.Named("grpc-client"), clientConfig)
	if err != nil {
		b.logger.Error("failed constructing gRPC client", "reason", err)
		return err
	}

	chanFinished := make(chan struct{}, 1)
	go func() {
		timer := time.NewTimer(b.bootstrapData.SafePingInterval())
		for {
			select {
			case <-timer.C:
				b.logger.Debug("pinging server")
				if err := client.Ping(); err != nil {
					b.logger.Error("ping returned an error", "reason", err)
					return
				}
				timer.Reset(time.Second * 5)
			case <-chanFinished:
				timer.Stop()
				b.logger.Debug("ping stopped, program finished")
				return
			}
		}
	}()

	if err := client.Commands(); err != nil {
		b.logger.Error("failed fetching bootstrap commands over gRPC", "reason", err)
		return err
	}

	for {

		serializableCommand := client.NextCommand()
		if serializableCommand == nil {
			break // finished
		}

		switch vCommand := serializableCommand.(type) {
		case commands.Run:
			if err := b.commandRunner.Execute(vCommand, client); err != nil {
				b.logger.Error("bootstrap failed, executing RUN command failed", "reason", err)
				close(chanFinished)
				client.Abort(err)
				return err
			}
		case commands.Add:
			if err := b.resourceDeployer.Add(vCommand, client); err != nil {
				b.logger.Error("bootstrap failed, executing ADD command failed", "reason", err)
				close(chanFinished)
				client.Abort(err)
				return err
			}
		case commands.Copy:
			if err := b.resourceDeployer.Copy(vCommand, client); err != nil {
				b.logger.Error("bootstrap failed, executing COPY command failed", "reason", err)
				close(chanFinished)
				client.Abort(err)
				return err
			}
		}

	}

	close(chanFinished)

	return client.Success()
}

func (b *defaultBootstrapper) WithCommandRunner(input CommandRunner) Bootstrapper {
	b.commandRunner = input
	return b
}
func (b *defaultBootstrapper) WithResourceDeployer(input ResourceDeployer) Bootstrapper {
	b.resourceDeployer = input
	return b
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
