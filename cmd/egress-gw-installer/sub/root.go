package sub

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	egressgw "github.com/ysksuzuki/egress-gw-cni-plugin"
)

const (
	defaultCniBinDir    = "/host/opt/cni/bin"
	defaultEgressGWPath = "/usr/local/egress-gw/egress-gw-cni"
)

var rootCmd = &cobra.Command{
	Use:     "egress-gw-installer",
	Short:   "install egress-gw CNI binary and configuration files",
	Long:    `egress-gw-installer setup egress-gw on each node by installing CNI binary and config files.`,
	Version: egressgw.Version(),
	RunE: func(cmd *cobra.Command, _ []string) error {
		cniBinDir := viper.GetString("CNI_BIN_DIR")
		egressGWPath := viper.GetString("EGRESS_GW_PATH")

		err := installEgressGW(egressGWPath, cniBinDir)
		if err != nil {
			return err
		}

		// Because kubelet scans /etc/cni/net.d for CNI config files every 5 seconds,
		// the installer need to sleep at least 5 seconds before finish.
		// ref: https://github.com/kubernetes/kubernetes/blob/3d9c6eb9e6e1759683d2c6cda726aad8a0e85604/pkg/kubelet/kubelet.go#L1416
		time.Sleep(6 * time.Second)
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.BindEnv("CNI_BIN_DIR")
	viper.BindEnv("EGRESS_GW_PATH")

	viper.SetDefault("CNI_BIN_DIR", defaultCniBinDir)
	viper.SetDefault("EGRESS_GW_PATH", defaultEgressGWPath)
}
