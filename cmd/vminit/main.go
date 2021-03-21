package main

import (
	"fmt"
	"os"

	"github.com/combust-labs/firebuild-mmds/configs"
	"github.com/combust-labs/firebuild-mmds/injectors"
	"github.com/combust-labs/firebuild-mmds/mmds"
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

	mmdsData, err := mmds.GuestFetchMMDSMetadata(rootLogger, fmt.Sprintf("http://%s/%s", config.MMDSIP, config.MetadataPath))
	if err != nil {
		// already logged
		return 1
	}

	if err := injectors.InjectSSHKeys(rootLogger, mmdsData, config.PathAuthorizedKeysPatternFile); err != nil {
		rootLogger.Error("error injecting ssh keys from MMDS data", "reason", err.Error())
		return 1
	}

	if err := injectors.InjectEnvironment(rootLogger, mmdsData, config.PathEnvFile); err != nil {
		rootLogger.Error("error injecting environment from MMDS data", "reason", err.Error())
		return 1
	}

	if err := injectors.InjectHostname(rootLogger, mmdsData, config.PathHostnameFile); err != nil {
		rootLogger.Error("error injecting local hostname from MMDS data", "reason", err.Error())
		return 1
	}

	if err := injectors.InjectHosts(rootLogger, mmdsData, defaultHosts, config.PathHostsFile); err != nil {
		rootLogger.Error("error injecting hosts from MMDS data", "reason", err.Error())
		return 1
	}

	return 0
}

// -- filesystem utils:

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
