package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/combust-labs/firebuild-mmds/configs"
	"github.com/combust-labs/firebuild-mmds/mmds"
	"github.com/spf13/cobra"
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
}

var (
	config = new(commandConfig)
	logCfg = configs.NewLogginConfig()
)

func initFlags() {
	rootCmd.Flags().StringVar(&config.MMDSIP, "guest-mmds-ip", "169.254.169.254", "Guest IP address of the MMDS service")
	rootCmd.Flags().StringVar(&config.MMDSIP, "metadata-path", "latest/meta-data", "Path to the metadata root")
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

	httpRequest, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/%s", config.MMDSIP, config.MetadataPath), nil)
	if err != nil {
		rootLogger.Error("error when creating a http request", "reason", err.Error())
		return 1
	}
	httpRequest.Header.Add("accept", "application/json")
	httpResponse, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		rootLogger.Error("error executing MMDS request", "reason", err.Error())
		return 1
	}
	defer httpResponse.Body.Close()
	mmdsData := &mmds.MMDSData{}
	if err := json.NewDecoder(httpResponse.Body).Decode(mmdsData); err != nil {
		rootLogger.Error("error deserializing MMDS data", "reason", err.Error())
		return 1
	}

	bytes, _ := json.MarshalIndent(mmdsData, "", "  ")

	fmt.Println(string(bytes))

	return 0
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
