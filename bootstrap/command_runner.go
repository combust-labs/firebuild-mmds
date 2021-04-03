package bootstrap

import (
	"fmt"
	"io/ioutil"
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

	environment, commandToExecute, cleanupFunc := constructExecutableCommand(n.logger, cmdEnv, cmd.Command)
	defer cleanupFunc()

	// TODO: https://github.com/combust-labs/firebuild/issues/2

	cmdargs := cmd.Shell.Commands
	//cmdargs = append(cmdargs, fmt.Sprintf("'%s'", strings.ReplaceAll(envString+cmdEnv.Expand(cmd.Command), "'", "'\\''")))
	cmdargs = append(cmdargs, commandToExecute)

	shellCmd := exec.Command(cmdargs[0], cmdargs[1:]...)
	shellCmd.Dir = cmd.Workdir.Value
	shellCmd.Env = environment
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

// returns environment, command to execute and a cleanup function
func constructExecutableCommand(logger hclog.Logger, cmdEnv env.BuildEnv, inputCommand string) ([]string, string, func()) {
	environment := os.Environ()
	for k, v := range cmdEnv.Snapshot() {
		environment = append(environment, fmt.Sprintf("%s=\"%s\"", k, v))
	}
	envFileContents := []string{}
	for _, envItem := range environment {
		envFileContents = append(envFileContents, fmt.Sprintf("export %s", envItem))
	}

	expandedCommand := cmdEnv.Expand(inputCommand)

	tempFile, tempFileErr := ioutil.TempFile("", "")

	if tempFileErr != nil {

		// if we are not allowed to write to the file, try executing the command inline:
		logger.Warn("failed creating temporary command file, passing command inline", "reason", tempFileErr)
		return environment, strings.Join(envFileContents, "; ") + expandedCommand, func() {}

	} else {

		deferredFunc := func() {
			fileNameToCleanup := tempFile.Name()
			if err := tempFile.Close(); err != nil {
				logger.Warn("failed closing command temporary file", "reason", err)
			}
			if err := os.Remove(fileNameToCleanup); err != nil {
				logger.Warn("failed removing command temporary file", "reason", err)
			}
		}

		// we have the file created but we need it executable:
		if err := tempFile.Chmod(0777); err != nil {
			// if we can't make it executable, we have to try executing the command inline:
			logger.Debug("failed chmoding command file, passing command inline", "reason", err)
			return environment, strings.Join(envFileContents, "; ") + expandedCommand, deferredFunc
		}

		// we preferably want to execute this content separated by new lines:
		fileContent := strings.Join(append(envFileContents, expandedCommand), "\n")
		if _, err := tempFile.WriteString(fileContent); err != nil {
			// but if we couldn't write to the file, we have to try best effort with an inline command:
			logger.Warn("failed writing temporary environment file, passing command inline", "reason", err)
			return environment, strings.Join(envFileContents, "; ") + expandedCommand, deferredFunc
		} else {
			// we're good, we have written to file so we can use the file as our command:
			logger.Debug("using executable file command", "script-file", tempFile.Name())
			return environment, fmt.Sprintf(". %s; ", tempFile.Name()), deferredFunc
		}

	}

}
