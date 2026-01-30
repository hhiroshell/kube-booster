package main

import (
	"flag"
	"fmt"
	"os"

	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/hhiroshell/kube-booster/pkg/controller"
	"github.com/hhiroshell/kube-booster/pkg/warmup"
	webhookpkg "github.com/hhiroshell/kube-booster/pkg/webhook"
)

var (
	version  = "dev"
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var webhookPort int
	var certDir string
	var showVersion bool
	var enableWebhook bool
	var enableController bool
	var nodeName string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.IntVar(&webhookPort, "webhook-port", 9443, "The port the webhook server binds to.")
	flag.StringVar(&certDir, "cert-dir", "/tmp/k8s-webhook-server/serving-certs", "The directory containing the webhook TLS certificates.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&enableWebhook, "enable-webhook", true, "Enable webhook server")
	flag.BoolVar(&enableController, "enable-controller", true, "Enable pod controller")
	flag.StringVar(&nodeName, "node-name", "", "Node name for node-local controller mode (enables node filtering)")

	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if showVersion {
		fmt.Printf("kube-booster controller version: %s\n", version)
		os.Exit(0)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Log node-local mode if configured
	if nodeName != "" {
		setupLog.Info("running in node-local mode", "nodeName", nodeName)
	}

	setupLog.Info("starting kube-booster controller",
		"version", version,
		"enableWebhook", enableWebhook,
		"enableController", enableController,
		"nodeName", nodeName,
		"webhook-port", webhookPort,
		"metrics-addr", metricsAddr,
	)

	// Build manager options
	mgrOptions := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "kube-booster.io",
	}

	// Only configure webhook server if webhook is enabled
	if enableWebhook {
		mgrOptions.WebhookServer = webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			CertDir: certDir,
		})
	}

	// Configure node-local cache if node name is specified
	if nodeName != "" {
		mgrOptions.Cache = cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Pod{}: {
					Field: fields.OneTermEqualSelector("spec.nodeName", nodeName),
				},
			},
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOptions)
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	// Create warmup executor
	warmupExecutor := warmup.NewVegetaExecutor(ctrl.Log.WithName("warmup"))

	// Setup pod controller (only if enabled)
	if enableController {
		if err = (&controller.PodReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			WarmupExecutor: warmupExecutor,
			Recorder:       mgr.GetEventRecorder("kube-booster-controller"),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Pod")
			os.Exit(1)
		}
		setupLog.Info("controller enabled")
	}

	// Setup webhook (only if enabled)
	if enableWebhook {
		mgr.GetWebhookServer().Register("/mutate-v1-pod", &webhook.Admission{
			Handler: webhookpkg.NewPodMutator(mgr.GetClient(), mgr.GetScheme()),
		})
		setupLog.Info("registered webhook", "path", "/mutate-v1-pod")
	}

	// Add health check endpoints
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
