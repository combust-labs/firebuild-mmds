package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/combust-labs/firebuild-mmds/configs"
	"github.com/combust-labs/firebuild-mmds/mmds"
	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var defaultHosts = map[string]string{
	"127.0.0.1": "localhost",
	"::1":       "localhost ip6-localhost ip6-loopback",
	"fe00::0":   "ip6-localnet",
	"ff00::0":   "ip6-mcastprefix",
	"ff02::1":   "ip6-allnodes",
	"ff02::2":   "ip6-allrouters",
}

const (
	defaultGuestMMDSIP                   = "169.254.169.254"
	defaultMetadataPath                  = "latest/meta-data"
	defaultPathAuthorizedKeysPatternFile = "/home/%s/.ssh/authorized_keys"
	defaultPathEnvFile                   = "/etc/profile.d/run-env.sh"
	defaultPathHostnameFile              = "/etc/hostname"
	defaultPathHostsFile                 = "/etc/hosts"
)

var rootCmd = &cobra.Command{
	Use:   "vminit",
	Short: "vminit",
	Long:  ``,
	Run:   run,
}

type commandConfig struct {
	MMDSIP       string
	MetadataPath string

	PathAuthorizedKeysPatternFile string
	PathEnvFile                   string
	PathHostnameFile              string
	PathHostsFile                 string
}

var (
	config = new(commandConfig)
	logCfg = configs.NewLogginConfig()
)

func initFlags() {
	rootCmd.Flags().StringVar(&config.MMDSIP, "guest-mmds-ip", defaultGuestMMDSIP, "Guest IP address of the MMDS service")
	rootCmd.Flags().StringVar(&config.MetadataPath, "metadata-path", defaultMetadataPath, "Path to the metadata root")

	rootCmd.Flags().StringVar(&config.PathAuthorizedKeysPatternFile, "path-authorized-keys-pattern", defaultPathAuthorizedKeysPatternFile, "Path to the metadata root")
	rootCmd.Flags().StringVar(&config.PathEnvFile, "path-env-file", defaultPathEnvFile, "Path to the metadata root")
	rootCmd.Flags().StringVar(&config.PathHostnameFile, "path-hostname-file", defaultPathHostnameFile, "Path to the metadata root")
	rootCmd.Flags().StringVar(&config.PathHostsFile, "path-hosts-file", defaultPathHostsFile, "Path to the metadata root")

	rootCmd.Flags().AddFlagSet(logCfg.FlagSet())
}

func init() {
	initFlags()
}

func run(cobraCommand *cobra.Command, _ []string) {
	os.Exit(processCommand())
}

func processCommand() int {

	rootLogger := logCfg.NewLogger("vminit")

	mmdsData, err := fetchMMDSMetadata(rootLogger)
	if err != nil {
		// already logged
		return 1
	}

	if err := injectSSHKeys(rootLogger, mmdsData, config.PathAuthorizedKeysPatternFile); err != nil {
		rootLogger.Error("error injecting ssh keys from MMDS data", "reason", err.Error())
		return 1
	}

	if err := injectEnvironment(rootLogger, mmdsData, config.PathEnvFile); err != nil {
		rootLogger.Error("error injecting environment from MMDS data", "reason", err.Error())
		return 1
	}

	if err := injectHostname(rootLogger, mmdsData, config.PathHostnameFile); err != nil {
		rootLogger.Error("error injecting local hostname from MMDS data", "reason", err.Error())
		return 1
	}

	if err := injectHosts(rootLogger, mmdsData, config.PathHostsFile); err != nil {
		rootLogger.Error("error injecting hosts from MMDS data", "reason", err.Error())
		return 1
	}

	return 0
}

func fetchMMDSMetadata(logger hclog.Logger) (*mmds.MMDSData, error) {
	httpRequest, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/%s", config.MMDSIP, config.MetadataPath), nil)
	if err != nil {
		logger.Error("error when creating a http request", "reason", err.Error())
		return nil, err
	}
	httpRequest.Header.Add("accept", "application/json")
	httpResponse, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		logger.Error("error executing MMDS request", "reason", err.Error())
		return nil, err
	}
	defer httpResponse.Body.Close()
	mmdsData := &mmds.MMDSData{}
	if err := json.NewDecoder(httpResponse.Body).Decode(mmdsData); err != nil {
		logger.Error("error deserializing MMDS data", "reason", err.Error())
		return nil, err
	}
	return mmdsData, nil
}

func injectEnvironment(logger hclog.Logger, mmdsData *mmds.MMDSData, envFile string) error {
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
		if err := os.Mkdir(filepath.Dir(envFile), 0755); err != nil { // the default permission for this directory
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

func injectHostname(logger hclog.Logger, mmdsData *mmds.MMDSData, etcHostnameFile string) error {

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

func injectHosts(logger hclog.Logger, mmdsData *mmds.MMDSData, etcHostsFile string) error {

	hosts := map[string]string{}
	for k, v := range defaultHosts {
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

func injectSSHKeys(logger hclog.Logger, mmdsData *mmds.MMDSData, authKeysFullPathPattern string) error {

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

// -- filesystem utils:

func checkIfExistsAndIsRegular(path string) (fs.FileInfo, error) {
	stat, statErr := os.Stat(path)
	if statErr != nil {
		return nil, statErr // don't wrap OS errors:
	}
	if !stat.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: '%s'", path)
	}
	return stat, nil
}

func pathExists(path string) (bool, error) {
	_, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return false, nil
		}
		return false, statErr
	}
	// something exists:
	return true, nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
