package bootstrap

import (
	"github.com/combust-labs/firebuild-shared/build/commands"
	"github.com/combust-labs/firebuild-shared/build/rootfs"
	"github.com/hashicorp/go-hclog"
)

type ResourceDeployer interface {
	Add(commands.Add, rootfs.ClientProvider) error
	Copy(commands.Copy, rootfs.ClientProvider) error
}

type noopResourceDeployer struct {
	logger hclog.Logger
}

func (n *noopResourceDeployer) Add(cmd commands.Add, grpcClient rootfs.ClientProvider) error {
	n.logger.Debug("executing ADD command", "command", cmd)
	return nil
}
func (n *noopResourceDeployer) Copy(cmd commands.Copy, grpcClient rootfs.ClientProvider) error {
	n.logger.Debug("executing COPY command", "command", cmd)
	return nil
}
