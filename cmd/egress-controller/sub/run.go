package sub

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	egressv1beta1 "github.com/ysksuzuki/egress-gw-cni-plugin/api/v1beta1"
	"github.com/ysksuzuki/egress-gw-cni-plugin/controllers"
	"github.com/ysksuzuki/egress-gw-cni-plugin/pkg/constants"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
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
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&config.zapOpts)))

	host, portStr, err := net.SplitHostPort(config.webhookAddr)
	if err != nil {
		return fmt.Errorf("invalid webhook address: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid webhook address: %w", err)
	}

	timeout := gracefulTimeout
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		LeaderElection:          true,
		LeaderElectionID:        "egress-leader",
		LeaderElectionNamespace: "kube-system", // egress-gw should run in kube-system
		MetricsBindAddress:      config.metricsAddr,
		GracefulShutdownTimeout: &timeout,
		HealthProbeBindAddress:  config.healthAddr,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    host,
			Port:    port,
			CertDir: config.certDir}),
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

	// register controllers

	podNS := os.Getenv(constants.EnvPodNamespace)
	podName := os.Getenv(constants.EnvPodName)
	img, err := controllers.GetImage(mgr.GetAPIReader(), client.ObjectKey{Namespace: podNS, Name: podName})
	if err != nil {
		return err
	}
	egressctrl := controllers.EgressReconciler{
		Client: mgr.GetClient(),
		Scheme: scheme,
		Image:  img,
		Port:   config.egressPort,
	}
	if err := egressctrl.SetupWithManager(mgr); err != nil {
		return err
	}

	if err := controllers.SetupCRBReconciler(mgr); err != nil {
		return err
	}

	// TODO register webhooks

	setupLog.Info("starting manager")
	ctx := ctrl.SetupSignalHandler()
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}

	return nil
}
