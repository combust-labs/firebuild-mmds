package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/combust-labs/firebuild-shared/build/commands"
	"github.com/combust-labs/firebuild-shared/build/rootfs"
	"github.com/combust-labs/firebuild-shared/env"
	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
)

type CommandRunner interface {
	Execute(commands.Run, rootfs.ClientProvider) error
}

type noopCommandRunner struct {
	logger hclog.Logger
}

func (n *noopCommandRunner) Execute(cmd commands.Run, grpcClient rootfs.ClientProvider) error {

	cmdEnv := env.NewBuildEnv()
	for k, v := range cmd.Args {
		cmdEnv.Put(k, v)
	}
	for k, v := range cmd.Env {
		cmdEnv.Put(k, v)
	}

	// We're running the commands by wrapping the command in the shell call so sshSession.Setenv might not do what we intend.
	// Also, we don't really know which shell are we running because it comes as an argument to us
	// so we can't, for example, assume bourne shell -a...
	envString := ""
	for k, v := range cmd.Env {
		envString = fmt.Sprintf("export %s%s=\"%s\"; ", envString, k, v)
	}

	executableCommand := fmt.Sprintf("mkdir -p %s && cd %s && %s '%s'\n",
		cmd.Workdir.Value,
		cmd.Workdir.Value,
		strings.Join(cmd.Shell.Commands, " "),
		strings.ReplaceAll(envString+cmdEnv.Expand(cmd.Command), "'", "'\\''"))

	n.logger.Debug("executing RUN command", "command", executableCommand)

	return nil
}

type shellCommandRunner struct {
	defaultUser commands.User
	logger      hclog.Logger
}

func NewShellCommandRunner(logger hclog.Logger) CommandRunner {
	return &shellCommandRunner{
		defaultUser: commands.DefaultUser(),
		logger:      logger,
	}
}

func (n *shellCommandRunner) Execute(cmd commands.Run, grpcClient rootfs.ClientProvider) error {

	logValues := []interface{}{
		"workdir", cmd.Workdir.Value,
		"user", cmd.User.Value,
		"shell", cmd.Shell.Commands,
	}
	if n.logger.IsTrace() {
		logValues = append(logValues, []interface{}{"raw-command", cmd}...)
	}

	n.logger.Debug("executing command", logValues...)

	cmdEnv := env.NewBuildEnv()
	for k, v := range cmd.Args {
		cmdEnv.Put(k, v)
	}
	for k, v := range cmd.Env {
		cmdEnv.Put(k, v)
	}

	// TODO: https://github.com/combust-labs/firebuild/issues/2

	cmdargs := cmd.Shell.Commands
	cmdargs = append(cmdargs, cmdEnv.Expand(cmd.Command))

	shellCmd := exec.Command(cmdargs[0], cmdargs[1:]...)
	shellCmd.Dir = cmd.Workdir.Value
	shellCmd.Env = func() []string {
		result := os.Environ()
		for k, v := range cmdEnv.Snapshot() {
			result = append(result, fmt.Sprintf("%s=%s", k, v))
		}
		return result
	}()
	shellCmd.Stderr = &shellCommandWriter{
		writerFunc: func(p []byte) error {
			n.logger.Trace("writing stderr", "data", string(p))
			return grpcClient.StdErr([]string{string(p)})
		},
	}
	shellCmd.Stdout = &shellCommandWriter{
		writerFunc: func(p []byte) error {
			n.logger.Trace("writing stdout", "data", string(p))
			return grpcClient.StdOut([]string{string(p)})
		},
	}

	// Start the command
	if err := shellCmd.Start(); err != nil {
		n.logger.Error("failed starting command", "reason", err)
		return err
	}

	if err := shellCmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {

			// The program has exited with an exit code != 0
			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			n.logger.Error("command finished with error", "reason", exiterr)
			return errors.Wrapf(exiterr, "command exited with code: %d, message %q", exiterr.ExitCode(), exiterr.String())
		} else {
			n.logger.Error("wait returned a non exec.ExitError error", "reason", err)
			return err
		}
	}

	n.logger.Debug("command finished successfully")

	return nil
}

type shellCommandWriter struct {
	writerFunc func([]byte) error
}

func (e *shellCommandWriter) Write(p []byte) (n int, err error) {
	if err := e.writerFunc(p); err != nil {
		return 0, err
	}
	return len(p), nil
}
