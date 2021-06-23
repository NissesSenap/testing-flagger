/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"github.com/fluxcd/pkg/runtime/logger"
	"github.com/fluxcd/pkg/runtime/metrics"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/fluxcd/pkg/runtime/pprof"
	"github.com/fluxcd/pkg/runtime/probes"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	crtlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	testingv1alpha1 "flagger.app/testing/api/v1alpha1"
	"flagger.app/testing/controllers"
	"flagger.app/testing/pkg/loadtester"
	"github.com/sethvargo/go-limiter/memorystore"
	prommetrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	//+kubebuilder:scaffold:imports
)

var (
	eventsAddr        = ":9090"
	scheme            = runtime.NewScheme()
	setupLog          = ctrl.Log.WithName("setup")
	rateLimitInterval = 5 * time.Minute
	timeout           = 1 * time.Hour
	logOptions        logger.Options
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(testingv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	log := logger.NewLogger(logOptions)
	ctrl.SetLogger(log)
	//ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	metricsRecorder := metrics.NewRecorder()
	crtlmetrics.Registry.MustRegister(metricsRecorder.Collectors()...)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "708ca791.flagger.app",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	probes.SetupChecks(mgr, setupLog)
	pprof.SetupHandlers(mgr, setupLog)

	if err = (&controllers.TestReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Test")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()
	store, err := memorystore.New(&memorystore.Config{
		Interval: rateLimitInterval,
	})
	if err != nil {
		setupLog.Error(err, "unable to create memorystore")
	}

	taskRunner := loadtester.NewTaskRunner(log, timeout)

	setupLog.Info("starting event server", "addr", eventsAddr)
	eventMdlw := middleware.New(middleware.Config{
		Recorder: prommetrics.NewRecorder(prommetrics.Config{
			Prefix:   "gotk_event",
			Registry: crtlmetrics.Registry,
		}),
	})

	gateStorage := loadtester.NewGateStorage("in-memory")
	eventServer := loadtester.NewEventServer(eventsAddr, log, mgr.GetClient())
	go eventServer.ListenAndServe(ctx.Done(), eventMdlw, store, taskRunner, gateStorage)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
