package injectors

import (
	"fmt"
	"os"

	"github.com/combust-labs/firebuild-mmds/mmds"
	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
)

// InjectHosts injects data into /etc/hosts file
func InjectHosts(logger hclog.Logger, mmdsData *mmds.MMDSData, defaults map[string]string, etcHostsFile string) error {

	hosts := map[string]string{}
	for k, v := range defaults {
		if k == "127.0.0.1" || k == "::1" {
			if len(mmdsData.Network.Interfaces) == 0 && mmdsData.LocalHostname != "" {
				// if there is no interface and hostname is given,
				// make 127.0.0.1 reply to the hostname
				v = v + " " + mmdsData.LocalHostname
			}
		}
		hosts[k] = v
	}
	if mmdsData.LocalHostname != "" {
		// if there is an interface and we have a hostname, make the hostname reply to the VMM IP:
		for _, v := range mmdsData.Network.Interfaces {
			hosts[v.IP] = mmdsData.LocalHostname
		}
	}

	sourceStat, err := checkIfExistsAndIsRegular(etcHostsFile)
	if err != nil {
		logger.Error("hosts file requirements failed", "on-disk-path", etcHostsFile, "reason", err)
		return err
	}

	logger.Debug("hosts file ok, going to chmod for writing")

	// I need to chmod it such that I can write it:
	if chmodErr := os.Chmod(etcHostsFile, 0660); chmodErr != nil {
		logger.Error("failed chmod hosts file for writing", "reason", chmodErr)
		return chmodErr
	}

	defer func() {
		// Chmod it to what it was before:
		logger.Debug("resetting mode perimissions for hosts file")
		if chmodErr := os.Chmod(etcHostsFile, sourceStat.Mode().Perm()); chmodErr != nil {
			logger.Error("failed resetting chmod hosts file AFTER writing", "reason", chmodErr)
		}
	}()

	logger.Debug("opening hosts file for writing", "current-file-size", sourceStat.Size())

	writableFile, fileErr := os.OpenFile(etcHostsFile, os.O_RDWR, 0660)
	if fileErr != nil {
		return fmt.Errorf("failed opening the hosts '%s' file for writing: %+v", etcHostsFile, fileErr)
	}
	defer func() {
		logger.Debug("closing hosts file after writing")
		if err := writableFile.Close(); err != nil {
			logger.Error("failed closing hosts file AFTER writing", "reason", err)
		}
	}()

	if err := writableFile.Truncate(0); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed truncating hosts file '%s'", etcHostsFile))
	}

	for k, v := range hosts {
		hostsLine := k + "\t" + v
		hostsLine = hostsLine + "\n"
		written, writeErr := writableFile.WriteString(hostsLine)
		if writeErr != nil {
			logger.Error("failed writing hosts to file", "reason", writeErr)
			return errors.Wrap(writeErr, "hosts file write failed: see error")
		}
		if written != len(hostsLine) {
			logger.Error("hosts file bytes written != hosts length", "kv", k+"::"+v, "written", written, "required", len(hostsLine))
			return errors.New("hosts file write failed: written != length")
		}
	}

	return nil
}
