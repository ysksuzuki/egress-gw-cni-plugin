package sub

import (
	"github.com/ysksuzuki/egress-gw-cni-plugin/runners"
	"net"
	"os"
	"time"

	"github.com/go-logr/zapr"
	egressv1beta1 "github.com/ysksuzuki/egress-gw-cni-plugin/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	gracefulTimeout = 20 * time.Second
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(egressv1beta1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

func subMain() error {
	// egress needs a raw zap logger for grpc_zip.
	zapLogger := zap.NewRaw(zap.UseFlagOptions(&config.zapOpts))
	defer zapLogger.Sync()

	grpcLogger := zapLogger.Named("grpc")
	ctrl.SetLogger(zapr.NewLogger(zapLogger))

	timeout := gracefulTimeout
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		LeaderElection:          false,
		MetricsBindAddress:      config.metricsAddr,
		GracefulShutdownTimeout: &timeout,
		HealthProbeBindAddress:  config.healthAddr,
	})
	if err != nil {
		return err
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		return err
	}

	os.Remove(config.socketPath)
	l, err := net.Listen("unix", config.socketPath)
	if err != nil {
		return err
	}
	server := runners.NewEgressGwAgent(l, grpcLogger)
	if err := mgr.Add(server); err != nil {
		return err
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}

	return nil
}
