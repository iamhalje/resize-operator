/*
Copyright 2026 Dmitry Ponomaryov

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

	_ "time/tzdata"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	// kubebuilder create api
	resizev1alpha1 "github.com/iamhalje/resize-operator/api/v1alpha1"
	"github.com/iamhalje/resize-operator/internal/controller"
	"github.com/iamhalje/resize-operator/internal/metrics"
	"github.com/iamhalje/resize-operator/internal/resize"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// kubebuilder create api
	utilruntime.Must(resizev1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func applyConfigTuning(cfg *rest.Config, qps float64, burst int) {
	if cfg == nil {
		return
	}
	if qps > 0 {
		cfg.QPS = float32(qps)
	}
	if burst > 0 {
		cfg.Burst = burst
	}
}

func main() {
	var enableLeaderElection bool
	var probeAddr string
	var metricsAddr string
	var timeZone string
	var kubeAPIQPS float64
	var kubeAPIBurst int
	var metricsTimeout time.Duration
	var resizeTimeout time.Duration
	var patchTimeout time.Duration
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&timeZone, "time-zone", "", "Time zone name for annotations (e.g. Asia/Yekaterinburg). Default is UTC.")
	flag.Float64Var(&kubeAPIQPS, "kube-api-qps", 0, "Kubernetes API client QPS (0 = default).")
	flag.IntVar(&kubeAPIBurst, "kube-api-burst", 0, "Kubernetes API client burst (0 = default).")
	flag.DurationVar(&metricsTimeout, "metrics-timeout", 5*time.Second, "Timeout for metrics-server list calls.")
	flag.DurationVar(&resizeTimeout, "resize-timeout", 10*time.Second, "Timeout for pods/resize dry-run/apply calls.")
	flag.DurationVar(&patchTimeout, "patch-timeout", 5*time.Second, "Timeout for pod annotation patch calls.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	cfg := ctrl.GetConfigOrDie()
	applyConfigTuning(cfg, kubeAPIQPS, kubeAPIBurst)

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "57410f0d.halje.ru",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}

	kubeClientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		setupLog.Error(err, "Failed to create kubernetes clientset")
		os.Exit(1)
	}
	metricsCS, err := metricsclientset.NewForConfig(cfg)
	if err != nil {
		setupLog.Error(err, "Failed to create metrics clientset")
		os.Exit(1)
	}

	loc := time.UTC
	if timeZone != "" {
		l, err := time.LoadLocation(timeZone)
		if err != nil {
			setupLog.Error(err, "Failed to load time zone; falling back to UTC", "timeZone", timeZone)
		} else {
			loc = l
		}
	}
	setupLog.Info("Time zone for annotations",
		"timeZone", timeZone,
		"resolved", loc.String(),
		"now", time.Now().In(loc).Format(time.RFC3339Nano),
	)

	// kubebuilder create api
	if err := (&controller.InPlacePodResizeReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
		Scheme:    mgr.GetScheme(),
		Metrics:   metrics.New(metricsCS),
		Resizer:   resize.NewProber(kubeClientset.Discovery(), kubeClientset),
		//lint:ignore SA1019
		Recorder:       mgr.GetEventRecorderFor("resize-operator"),
		TimeLocation:   loc,
		MetricsTimeout: metricsTimeout,
		ResizeTimeout:  resizeTimeout,
		PatchTimeout:   patchTimeout,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "InPlacePodResize")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Failed to run manager")
		os.Exit(1)
	}
}
