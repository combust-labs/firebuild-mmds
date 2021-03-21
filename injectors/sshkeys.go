package injectors

import (
	"fmt"
	"os"

	"github.com/combust-labs/firebuild-mmds/mmds"
	"github.com/hashicorp/go-hclog"
)

// InjectSSHKeys injects user SSH keys into respective authorized)keys file.
func InjectSSHKeys(logger hclog.Logger, mmdsData *mmds.MMDSData, authKeysFullPathPattern string) error {

	if len(mmdsData.Users) == 0 {
		logger.Debug("no users, nothing to do")
		return nil // nothing to do
	}

	for username, userinfo := range mmdsData.Users {

		authKeysFullPath := fmt.Sprintf(authKeysFullPathPattern, username)

		logger.Debug("authorized_keys file to use", "path", authKeysFullPath)
		logger.Debug("checking the authorized_keys file")

		sourceStat, err := checkIfExistsAndIsRegular(authKeysFullPath)
		if err != nil {
			logger.Error("authorized_keys file requirements failed", "on-disk-path", authKeysFullPath, "reason", err)
			return err
		}

		logger.Debug("authorized_keys file ok, going to chmod for writing")

		// I need to chmod it such that I can write it:
		if chmodErr := os.Chmod(authKeysFullPath, 0660); chmodErr != nil {
			logger.Error("failed chmod authorized_keys file for writing", "reason", chmodErr)
			return chmodErr
		}

		defer func() {
			// Chmod it to what it was before:
			logger.Debug("resetting mode perimissions for authorized_keys file")
			if chmodErr := os.Chmod(authKeysFullPath, sourceStat.Mode().Perm()); chmodErr != nil {
				logger.Error("failed resetting chmod authorized_keys file AFTER writing", "reason", chmodErr)
			}
		}()

		logger.Debug("opening authorized_keys file for writing", "current-file-size", sourceStat.Size())

		writableFile, fileErr := os.OpenFile(authKeysFullPath, os.O_RDWR, 0660)
		if fileErr != nil {
			return fmt.Errorf("failed opening the authorized_keys '%s' file for writing: %+v", authKeysFullPath, fileErr)
		}
		defer func() {
			logger.Debug("closing authorized_keys file after writing")
			if err := writableFile.Close(); err != nil {
				logger.Error("failed closing authorized_keys file AFTER writing", "reason", err)
			}
		}()

		// make sure we have a new line:
		if sourceStat.Size() > 0 {
			logger.Debug("content found in authorized_keys file, appening new line")
			if _, err := writableFile.Write([]byte("\n")); err != nil {
				logger.Error("failed writing new line authorized_keys file", "reason", err)
				return err
			}
		}

		written, err := writableFile.Write([]byte(userinfo.SSHKeys))
		if err != nil {
			logger.Error("failed writing marshaled key to authorized_keys file", "reason", err)
			return err
		}
		expectedToWrite := len(userinfo.SSHKeys)
		if written != expectedToWrite {
			logger.Error("written != len", "written", written, "len", expectedToWrite)
		}

	}
	return nil
}
