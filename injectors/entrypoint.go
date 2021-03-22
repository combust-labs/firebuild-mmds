package injectors

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/combust-labs/firebuild-mmds/mmds"
	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
)

// InjectEnvironment injects an environment into an /etc/profile.d/... file.
func InjectEntrypoint(logger hclog.Logger, mmdsData *mmds.MMDSData, entrypointRunnerPath, envFile string) error {

	entrypointInfo, jsonErr := mmds.NewMMDSRootfsEntrypointInfoFromJSON(mmdsData.EntrypointJSON)
	if jsonErr != nil {
		logger.Warn("entrypoint information could not be deserializing; exit early", "reason", jsonErr)
		return jsonErr
	}

	if len(entrypointInfo.Entrypoint) == 0 {
		logger.Debug("no entrypoint, nothing to do")
		return nil // nothing to do
	}

	// make sure a parent directory exists:
	dirExists, err := pathExists(filepath.Dir(entrypointRunnerPath))
	if err != nil {
		logger.Error("failed checking if entrypoint runner file parent directory exists", "reason", err)
		return err
	}
	if !dirExists {
		logger.Debug("creating entrypoint runner file parent directory", "entrypoint-runner", entrypointRunnerPath)
		if err := os.MkdirAll(filepath.Dir(entrypointRunnerPath), 0755); err != nil { // the default permission for this directory
			return errors.Wrap(err, "failed creating parent entrypoint runner directory")
		}
	}

	logger.Debug("writing entrypoint runner file", "parent-existed", dirExists)

	writableFile, openErr := os.OpenFile(entrypointRunnerPath, os.O_CREATE|os.O_RDWR, 0755)
	if openErr != nil {
		logger.Error("failed opening entrypoint runner file for writing", "reason", openErr)
		return errors.Wrap(openErr, "failed opening entrypoint runner file for writing")
	}
	defer writableFile.Close()
	shell, env, command := entrypointInfo.ToShellCommand()
	stringToWrite := fmt.Sprintf("#!/bin/sh\n\n%s '%sif [ -f \"%s\" ]; then . \"%s\"; fi; %s'\n", shell, env, envFile, envFile, command)

	written, writeErr := writableFile.WriteString(stringToWrite)
	if err != nil {
		return errors.Wrap(writeErr, "env file write failed: see error")
	}
	if written != len(stringToWrite) {
		logger.Error("env file bytes written != data length", "written", written, "required", len(stringToWrite))
		return errors.New("env file write failed: written != length")
	}

	return nil
}
