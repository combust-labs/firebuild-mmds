package injectors

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/combust-labs/firebuild-mmds/mmds"
	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
)

// InjectEnvironment injects an environment into an /etc/profile.d/... file.
func InjectEnvironment(logger hclog.Logger, mmdsData *mmds.MMDSData, envFile string) error {
	if mmdsData.Env == nil {
		logger.Debug("no env, nothing to do")
		return nil // nothing to do
	}

	if len(mmdsData.Env) == 0 {
		logger.Debug("env empty, nothing to do")
		return nil // nothing to do
	}

	// make sure a parent directory exists:
	dirExists, err := pathExists(filepath.Dir(envFile))
	if err != nil {
		logger.Error("failed checking if env file parent directory exists", "reason", err)
		return err
	}
	if !dirExists {
		logger.Debug("creating env file parent directory", "env-file", envFile)
		if err := os.MkdirAll(filepath.Dir(envFile), 0755); err != nil { // the default permission for this directory
			return errors.Wrap(err, "failed creating parent env directory")
		}
	}

	logger.Debug("writing env file", "parent-existed", dirExists)

	writableFile, openErr := os.OpenFile(envFile, os.O_CREATE|os.O_RDWR, 0755)
	if openErr != nil {
		logger.Error("failed opening env file for writing", "reason", openErr)
		return errors.Wrap(openErr, "failed opening env file for writing")
	}
	defer writableFile.Close()

	for k, v := range mmdsData.Env {
		line := fmt.Sprintf("export %s=\"%s\"\n", k, strings.ReplaceAll(v, "\"", "\\\""))
		written, writeErr := writableFile.WriteString(line)
		if err != nil {
			return errors.Wrap(writeErr, "env file write failed: see error")
		}
		if written != len(line) {
			logger.Error("env file bytes written != line length", "kv", k+"::"+v, "written", written, "required", len(line))
			return errors.New("env file write failed: written != length")
		}
	}

	return nil
}
