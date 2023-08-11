package sub

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	egressgw "github.com/ysksuzuki/egress-gw-cni-plugin"
	"github.com/ysksuzuki/egress-gw-cni-plugin/pkg/constants"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var config struct {
	metricsAddr      string
	healthAddr       string
	podTableId       int
	podRulePrio      int
	exportTableId    int
	protocolId       int
	socketPath       string
	compatCalico     bool
	egressPort       int
	registerFromMain bool
	zapOpts          zap.Options
}

var rootCmd = &cobra.Command{
	Use:   "egress-gw-agent",
	Short: "gRPC server running on each node",
	Long: `egress-gw-agent is a gRPC server running on each node.

It listens on a UNIX domain socket and accepts requests from
CNI plugin.`,
	Version: egressgw.Version(),
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return subMain()
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
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&config.metricsAddr, "metrics-addr", ":9384", "bind address of metrics endpoint")
	pf.StringVar(&config.healthAddr, "health-addr", ":9385", "bind address of health/readiness probes")
	pf.IntVar(&config.protocolId, "protocol-id", 30, "route author ID")
	pf.StringVar(&config.socketPath, "socket", constants.DefaultSocketPath, "UNIX domain socket path")
	pf.IntVar(&config.egressPort, "egress-port", 5555, "UDP port number for egress NAT")

	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	config.zapOpts.BindFlags(goflags)

	pf.AddGoFlagSet(goflags)
}
