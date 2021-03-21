package injectors

import (
	"fmt"
	"os"

	"github.com/combust-labs/firebuild-mmds/mmds"
	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
)

// InjectHostname injects the hostname into /etc/hostname file.
func InjectHostname(logger hclog.Logger, mmdsData *mmds.MMDSData, etcHostnameFile string) error {

	if len(mmdsData.LocalHostname) == 0 {
		logger.Debug("no local hostname, nothing to do")
		return nil // nothing to do
	}

	sourceStat, err := checkIfExistsAndIsRegular(etcHostnameFile)
	if err != nil {
		logger.Error("hostname file requirements failed", "on-disk-path", etcHostnameFile, "reason", err)
		return err
	}

	logger.Debug("hostname file ok, going to chmod for writing")

	// I need to chmod it such that I can write it:
	if chmodErr := os.Chmod(etcHostnameFile, 0660); chmodErr != nil {
		logger.Error("failed chmod hostname file for writing", "reason", chmodErr)
		return chmodErr
	}

	defer func() {
		// Chmod it to what it was before:
		logger.Debug("resetting mode perimissions for hostname file")
		if chmodErr := os.Chmod(etcHostnameFile, sourceStat.Mode().Perm()); chmodErr != nil {
			logger.Error("failed resetting chmod hostname file AFTER writing", "reason", chmodErr)
		}
	}()

	logger.Debug("opening hostname file for writing", "current-file-size", sourceStat.Size())

	writableFile, fileErr := os.OpenFile(etcHostnameFile, os.O_RDWR, 0660)
	if fileErr != nil {
		return fmt.Errorf("failed opening the hostname '%s' file for writing: %+v", etcHostnameFile, fileErr)
	}
	defer func() {
		logger.Debug("closing hostname file after writing")
		if err := writableFile.Close(); err != nil {
			logger.Error("failed closing hostname file AFTER writing", "reason", err)
		}
	}()

	written, writeErr := writableFile.WriteString(mmdsData.LocalHostname)
	if writeErr != nil {
		logger.Error("failed writing hostname to file", "reason", writeErr)
		return errors.Wrap(writeErr, "hostname file write failed: see error")
	}
	if written != len(mmdsData.LocalHostname) {
		logger.Error("hostname file bytes written != hostname length", "written", written, "required", len(mmdsData.LocalHostname))
		return errors.New("hostname file write failed: written != length")
	}

	return nil
}
