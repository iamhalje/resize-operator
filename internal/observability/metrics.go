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

package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

type RunStats struct {
	PodsListed               int
	PodsEligible             int
	PodsSkippedCooldown      int
	PodsMaxPodsSkipped       int
	PodsPendingStabilization int
	PodsBudgetSkipped        int
	PodsMetricsNotFound      int
	PodsMetricsErrorsOther   int
	PodsQoSSkipped           int
	PodsConflictSkipped      int
	PodsNoChange             int
	PodsDryRunFailed         int
	PodsApplyFailed          int
	PodsResized              int

	MetricsOK bool

	DeltaCPURequestMilli   int64
	DeltaMemoryRequestByte int64
}

var (
	reconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resize_operator_reconcile_total",
			Help: "Total number of reconciliations per policy.",
		},
		[]string{"policy"},
	)
	reconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "resize_operator_reconcile_duration_seconds",
			Help:    "Reconciliation duration per policy.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"policy"},
	)
	lastRunTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "resize_operator_last_run_timestamp_seconds",
			Help: "Unix timestamp of the last completed reconcile per policy.",
		},
		[]string{"policy"},
	)
	lastRunMetricsOK = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "resize_operator_last_run_metrics_ok",
			Help: "1 if metrics-server was reachable during the last run; otherwise 0.",
		},
		[]string{"policy"},
	)
	lastRunDeltaCPURequestMilli = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "resize_operator_last_run_delta_cpu_request_millicores",
			Help: "Sum of CPU request delta (millicores) applied during the last run.",
		},
		[]string{"policy"},
	)
	lastRunDeltaMemoryRequestBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "resize_operator_last_run_delta_memory_request_bytes",
			Help: "Sum of memory request delta (bytes) applied during the last run.",
		},
		[]string{"policy"},
	)
	podsResizedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resize_operator_pods_resized_total",
			Help: "Total number of pods successfully resized (apply succeeded).",
		},
		[]string{"policy"},
	)
	podsSkippedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resize_operator_pods_skipped_total",
			Help: "Total number of pods skipped, by reason.",
		},
		[]string{"policy", "reason"},
	)
	podsFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resize_operator_pods_failed_total",
			Help: "Total number of pods that failed during resize, by phase.",
		},
		[]string{"policy", "phase"},
	)
	apiCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resize_operator_api_calls_total",
			Help: "Total API calls made by the operator, by call type.",
		},
		[]string{"policy", "call"},
	)
	requestsClampedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resize_operator_requests_clamped_total",
			Help: "Total number of times desired request was clamped to an existing limit.",
		},
		[]string{"policy", "resource"},
	)
	podsResizedByNamespaceTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resize_operator_pods_resized_namespace_total",
			Help: "Total number of pods successfully resized (apply succeeded), by namespace.",
		},
		[]string{"policy", "namespace"},
	)
	requestsBoundedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resize_operator_requests_bounded_total",
			Help: "Total number of times desired request was clamped to configured bounds.",
		},
		[]string{"policy", "resource", "bound"},
	)
	requestsBoundedByNamespaceTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resize_operator_requests_bounded_namespace_total",
			Help: "Total number of times desired request was clamped to configured bounds, by namespace.",
		},
		[]string{"policy", "namespace", "resource", "bound"},
	)
	lastRunDeltaCPURequestMilliByNamespace = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "resize_operator_last_run_delta_cpu_request_millicores_namespace",
			Help: "Sum of CPU request delta (millicores) applied during the last run, by namespace.",
		},
		[]string{"policy", "namespace"},
	)
	lastRunDeltaMemoryRequestBytesByNamespace = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "resize_operator_last_run_delta_memory_request_bytes_namespace",
			Help: "Sum of memory request delta (bytes) applied during the last run, by namespace.",
		},
		[]string{"policy", "namespace"},
	)
	deltaCPURequestMilliTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resize_operator_delta_cpu_request_millicores_total",
			Help: "Total CPU request changes applied since process start (millicores), split by direction.",
		},
		[]string{"policy", "direction"},
	)
	deltaMemoryRequestBytesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resize_operator_delta_memory_request_bytes_total",
			Help: "Total memory request changes applied since process start (bytes), split by direction.",
		},
		[]string{"policy", "direction"},
	)
	deltaCPURequestMilliByNamespaceTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resize_operator_delta_cpu_request_millicores_namespace_total",
			Help: "Total CPU request changes applied since process start (millicores), by namespace and direction.",
		},
		[]string{"policy", "namespace", "direction"},
	)
	deltaMemoryRequestBytesByNamespaceTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resize_operator_delta_memory_request_bytes_namespace_total",
			Help: "Total memory request changes applied since process start (bytes), by namespace and direction.",
		},
		[]string{"policy", "namespace", "direction"},
	)
)

func init() {
	ctrlmetrics.Registry.MustRegister(
		reconcileTotal,
		reconcileDuration,
		lastRunTimestamp,
		lastRunMetricsOK,
		lastRunDeltaCPURequestMilli,
		lastRunDeltaMemoryRequestBytes,
		podsResizedTotal,
		podsSkippedTotal,
		podsFailedTotal,
		apiCallsTotal,
		requestsClampedTotal,
		requestsBoundedTotal,
		requestsBoundedByNamespaceTotal,
		podsResizedByNamespaceTotal,
		lastRunDeltaCPURequestMilliByNamespace,
		lastRunDeltaMemoryRequestBytesByNamespace,
		deltaCPURequestMilliTotal,
		deltaMemoryRequestBytesTotal,
		deltaCPURequestMilliByNamespaceTotal,
		deltaMemoryRequestBytesByNamespaceTotal,
	)
}

func InitPolicyMetrics(policy string) {
	if policy == "" {
		policy = "unknown"
	}

	reconcileTotal.WithLabelValues(policy)
	reconcileDuration.WithLabelValues(policy)
	lastRunTimestamp.WithLabelValues(policy)
	lastRunMetricsOK.WithLabelValues(policy)
	lastRunDeltaCPURequestMilli.WithLabelValues(policy)
	lastRunDeltaMemoryRequestBytes.WithLabelValues(policy)

	podsResizedTotal.WithLabelValues(policy)
	podsFailedTotal.WithLabelValues(policy, "dryrun")
	podsFailedTotal.WithLabelValues(policy, "apply")

	for _, r := range []string{
		"nochange",
		"cooldown",
		"maxpods",
		"stabilization",
		"budget",
		"metrics_not_found",
		"metrics_error",
		"qos",
		"conflict",
	} {
		podsSkippedTotal.WithLabelValues(policy, r)
	}

	for _, res := range []string{"cpu", "memory"} {
		requestsClampedTotal.WithLabelValues(policy, res)
		requestsBoundedTotal.WithLabelValues(policy, res, "min")
		requestsBoundedTotal.WithLabelValues(policy, res, "max")
	}

	for _, dir := range []string{"up", "down"} {
		deltaCPURequestMilliTotal.WithLabelValues(policy, dir)
		deltaMemoryRequestBytesTotal.WithLabelValues(policy, dir)
	}
}

func ObserveRun(policy string, started time.Time, s RunStats) {
	if policy == "" {
		policy = "unknown"
	}
	reconcileTotal.WithLabelValues(policy).Inc()
	reconcileDuration.WithLabelValues(policy).Observe(time.Since(started).Seconds())
	lastRunTimestamp.WithLabelValues(policy).Set(float64(time.Now().Unix()))
	if s.PodsMetricsErrorsOther > 0 {
		lastRunMetricsOK.WithLabelValues(policy).Set(0)
	} else {
		lastRunMetricsOK.WithLabelValues(policy).Set(1)
	}
	lastRunDeltaCPURequestMilli.WithLabelValues(policy).Set(float64(s.DeltaCPURequestMilli))
	lastRunDeltaMemoryRequestBytes.WithLabelValues(policy).Set(float64(s.DeltaMemoryRequestByte))
	AddDeltaTotals(policy, s.DeltaCPURequestMilli, s.DeltaMemoryRequestByte)

	if s.PodsResized > 0 {
		podsResizedTotal.WithLabelValues(policy).Add(float64(s.PodsResized))
	}
	if s.PodsDryRunFailed > 0 {
		podsFailedTotal.WithLabelValues(policy, "dryrun").Add(float64(s.PodsDryRunFailed))
	}
	if s.PodsApplyFailed > 0 {
		podsFailedTotal.WithLabelValues(policy, "apply").Add(float64(s.PodsApplyFailed))
	}

	if s.PodsNoChange > 0 {
		podsSkippedTotal.WithLabelValues(policy, "nochange").Add(float64(s.PodsNoChange))
	}
	if s.PodsSkippedCooldown > 0 {
		podsSkippedTotal.WithLabelValues(policy, "cooldown").Add(float64(s.PodsSkippedCooldown))
	}
	if s.PodsPendingStabilization > 0 {
		podsSkippedTotal.WithLabelValues(policy, "stabilization").Add(float64(s.PodsPendingStabilization))
	}
	if s.PodsBudgetSkipped > 0 {
		podsSkippedTotal.WithLabelValues(policy, "budget").Add(float64(s.PodsBudgetSkipped))
	}
	if s.PodsMetricsNotFound > 0 {
		podsSkippedTotal.WithLabelValues(policy, "metrics_not_found").Add(float64(s.PodsMetricsNotFound))
	}
	if s.PodsMetricsErrorsOther > 0 {
		podsSkippedTotal.WithLabelValues(policy, "metrics_error").Add(float64(s.PodsMetricsErrorsOther))
	}
	if s.PodsQoSSkipped > 0 {
		podsSkippedTotal.WithLabelValues(policy, "qos").Add(float64(s.PodsQoSSkipped))
	}
	if s.PodsConflictSkipped > 0 {
		podsSkippedTotal.WithLabelValues(policy, "conflict").Add(float64(s.PodsConflictSkipped))
	}
	if s.PodsMaxPodsSkipped > 0 {
		podsSkippedTotal.WithLabelValues(policy, "maxpods").Add(float64(s.PodsMaxPodsSkipped))
	}
}

func IncAPICall(policy, call string) {
	if policy == "" {
		policy = "unknown"
	}
	if call == "" {
		call = "unknown"
	}
	apiCallsTotal.WithLabelValues(policy, call).Inc()
}

func AddClamped(policy, resource string, n int) {
	if n <= 0 {
		return
	}
	if policy == "" {
		policy = "unknown"
	}
	if resource == "" {
		resource = "unknown"
	}
	requestsClampedTotal.WithLabelValues(policy, resource).Add(float64(n))
}

func ObserveNamespace(policy, namespace string, resized int, deltaCPUMilli, deltaMemBytes int64) {
	if policy == "" {
		policy = "unknown"
	}
	if namespace == "" {
		namespace = "unknown"
	}
	if resized > 0 {
		podsResizedByNamespaceTotal.WithLabelValues(policy, namespace).Add(float64(resized))
	}
	lastRunDeltaCPURequestMilliByNamespace.WithLabelValues(policy, namespace).Set(float64(deltaCPUMilli))
	lastRunDeltaMemoryRequestBytesByNamespace.WithLabelValues(policy, namespace).Set(float64(deltaMemBytes))
	AddDeltaTotalsNamespace(policy, namespace, deltaCPUMilli, deltaMemBytes)
}

func AddDeltaTotals(policy string, deltaCPUMilli, deltaMemBytes int64) {
	if policy == "" {
		policy = "unknown"
	}
	if deltaCPUMilli > 0 {
		deltaCPURequestMilliTotal.WithLabelValues(policy, "up").Add(float64(deltaCPUMilli))
	} else if deltaCPUMilli < 0 {
		deltaCPURequestMilliTotal.WithLabelValues(policy, "down").Add(float64(-deltaCPUMilli))
	}
	if deltaMemBytes > 0 {
		deltaMemoryRequestBytesTotal.WithLabelValues(policy, "up").Add(float64(deltaMemBytes))
	} else if deltaMemBytes < 0 {
		deltaMemoryRequestBytesTotal.WithLabelValues(policy, "down").Add(float64(-deltaMemBytes))
	}
}

func AddDeltaTotalsNamespace(policy, namespace string, deltaCPUMilli, deltaMemBytes int64) {
	if policy == "" {
		policy = "unknown"
	}
	if namespace == "" {
		namespace = "unknown"
	}
	if deltaCPUMilli > 0 {
		deltaCPURequestMilliByNamespaceTotal.WithLabelValues(policy, namespace, "up").Add(float64(deltaCPUMilli))
	} else if deltaCPUMilli < 0 {
		deltaCPURequestMilliByNamespaceTotal.WithLabelValues(policy, namespace, "down").Add(float64(-deltaCPUMilli))
	}
	if deltaMemBytes > 0 {
		deltaMemoryRequestBytesByNamespaceTotal.WithLabelValues(policy, namespace, "up").Add(float64(deltaMemBytes))
	} else if deltaMemBytes < 0 {
		deltaMemoryRequestBytesByNamespaceTotal.WithLabelValues(policy, namespace, "down").Add(float64(-deltaMemBytes))
	}
}

func AddBounded(policy, resource, bound string, n int) {
	if n <= 0 {
		return
	}
	if policy == "" {
		policy = "unknown"
	}
	if resource == "" {
		resource = "unknown"
	}
	if bound == "" {
		bound = "unknown"
	}
	requestsBoundedTotal.WithLabelValues(policy, resource, bound).Add(float64(n))
}

func AddBoundedNamespace(policy, namespace, resource, bound string, n int) {
	if n <= 0 {
		return
	}
	if policy == "" {
		policy = "unknown"
	}
	if namespace == "" {
		namespace = "unknown"
	}
	if resource == "" {
		resource = "unknown"
	}
	if bound == "" {
		bound = "unknown"
	}
	requestsBoundedByNamespaceTotal.WithLabelValues(policy, namespace, resource, bound).Add(float64(n))
}
