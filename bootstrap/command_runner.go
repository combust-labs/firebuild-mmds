package bootstrap

import (
	"github.com/combust-labs/firebuild-shared/build/commands"
	"github.com/combust-labs/firebuild-shared/build/rootfs"
	"github.com/hashicorp/go-hclog"
)

type CommandRunner interface {
	Execute(commands.Run, rootfs.ClientProvider) error
}

type noopCommandRunner struct {
	logger hclog.Logger
}

func (n *noopCommandRunner) Execute(cmd commands.Run, grpcClient rootfs.ClientProvider) error {
	n.logger.Debug("executing RUN command", "command", cmd)
	return nil
}
