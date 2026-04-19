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

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	resizev1alpha1 "github.com/iamhalje/resize-operator/api/v1alpha1"
	"github.com/iamhalje/resize-operator/internal/nsselector"
	"github.com/iamhalje/resize-operator/internal/observability"
	"github.com/iamhalje/resize-operator/internal/policy"
	"github.com/iamhalje/resize-operator/internal/resize"
)

// +kubebuilder:rbac:groups=resize.halje.ru,resources=inplacepodresizes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=resize.halje.ru,resources=inplacepodresizes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=resize.halje.ru,resources=inplacepodresizes/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups="",resources=pods/resize,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=metrics.k8s.io,resources=pods,verbs=get;list;watch

func (r *InPlacePodResizeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	started := now()
	formatTime := func(t time.Time) string {
		if r != nil && r.TimeLocation != nil {
			return t.In(r.TimeLocation).Format(time.RFC3339Nano)
		}
		return t.UTC().Format(time.RFC3339Nano)
	}

	var ipr resizev1alpha1.InPlacePodResize
	if err := r.Get(ctx, req.NamespacedName, &ipr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	observability.InitPolicyMetrics(ipr.Name)

	log.Info("Reconcile InPlacePodResize", "name", ipr.Name, "generation", ipr.Generation, "enabled", ipr.Spec.Enabled)

	metricsTimeout := r.MetricsTimeout
	if metricsTimeout <= 0 {
		metricsTimeout = 5 * time.Second
	}
	resizeTimeout := r.ResizeTimeout
	if resizeTimeout <= 0 {
		resizeTimeout = 10 * time.Second
	}
	patchTimeout := r.PatchTimeout
	if patchTimeout <= 0 {
		patchTimeout = 5 * time.Second
	}

	interval := ipr.Spec.Timing.Interval.Duration
	if interval <= 0 {
		interval = time.Minute
	}
	cooldown := ipr.Spec.Timing.Cooldown.Duration
	stab := ipr.Spec.Timing.StabilizationWindow.Duration
	maxPods := ipr.Spec.Timing.MaxPodsPerRun
	if maxPods <= 0 {
		maxPods = 20
	}
	capTTL := ipr.Spec.Timing.CapabilityCheckInterval.Duration
	if capTTL <= 0 {
		capTTL = time.Hour
	}

	statusPatch := client.MergeFrom(ipr.DeepCopy())
	setCond := func(cond metav1.Condition) {
		meta.SetStatusCondition(&ipr.Status.Conditions, cond)
	}
	var requeueAfter time.Duration
	defer func() {
		ipr.Status.ObservedGeneration = ipr.Generation
		ipr.Status.LastRunTime = &metav1.Time{Time: now()}
		if err := r.Status().Patch(ctx, &ipr, statusPatch); err != nil {
			log.Error(err, "Patch status failed")
		}
	}()

	if !ipr.Spec.Enabled {
		setCond(metav1.Condition{
			Type:               conditionActive,
			Status:             metav1.ConditionFalse,
			Reason:             "Disabled",
			Message:            "policy is disabled",
			ObservedGeneration: ipr.Generation,
			LastTransitionTime: metav1.NewTime(now()),
		})
		requeueAfter = interval
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	setCond(metav1.Condition{
		Type:               conditionActive,
		Status:             metav1.ConditionTrue,
		Reason:             "Enabled",
		Message:            "policy is enabled",
		ObservedGeneration: ipr.Generation,
		LastTransitionTime: metav1.NewTime(now()),
	})

	var cap resize.Capability
	if r.Resizer != nil {
		cap = r.Resizer.Supported(ctx, capTTL, now())
		observability.ObserveCapability(ipr.Name, string(cap.State))
		ipr.Status.LastCapabilityCheckTime = &metav1.Time{Time: cap.Checked}
		switch cap.State {
		case resize.SupportSupported:
			setCond(metav1.Condition{
				Type:               conditionInPlaceResizeSupport,
				Status:             metav1.ConditionTrue,
				Reason:             cap.Reason,
				Message:            cap.Message,
				ObservedGeneration: ipr.Generation,
				LastTransitionTime: metav1.NewTime(now()),
			})
		case resize.SupportUnsupported:
			setCond(metav1.Condition{
				Type:               conditionInPlaceResizeSupport,
				Status:             metav1.ConditionFalse,
				Reason:             cap.Reason,
				Message:            cap.Message,
				ObservedGeneration: ipr.Generation,
				LastTransitionTime: metav1.NewTime(now()),
			})
		default:
			setCond(metav1.Condition{
				Type:               conditionInPlaceResizeSupport,
				Status:             metav1.ConditionUnknown,
				Reason:             cap.Reason,
				Message:            cap.Message,
				ObservedGeneration: ipr.Generation,
				LastTransitionTime: metav1.NewTime(now()),
			})
		}
	}

	if cap.State == resize.SupportUnsupported {
		if r.Recorder != nil {
			r.Recorder.Eventf(&ipr, corev1.EventTypeWarning, "ResizeUnsupported", "In-place resize unsupported: %s", cap.Message)
		}
		log.Info("In-place resize unsupported; skipping evaluation",
			"state", cap.State, "reason", cap.Reason, "message", cap.Message, "requeueAfter", interval.String())
		requeueAfter = interval
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	nsMatchType := nsselector.MatchType(ipr.Spec.NamespaceSelector.MatchType)
	selector, err := nsselector.Compile(nsMatchType, ipr.Spec.NamespaceSelector.Include, ipr.Spec.NamespaceSelector.Exclude)
	if err != nil {
		setCond(metav1.Condition{
			Type:               conditionActive,
			Status:             metav1.ConditionFalse,
			Reason:             "InvalidNamespaceSelector",
			Message:            err.Error(),
			ObservedGeneration: ipr.Generation,
			LastTransitionTime: metav1.NewTime(now()),
		})
		requeueAfter = interval
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	if len(ipr.Spec.NamespaceSelector.Include) == 0 && ipr.Spec.PodSelector == nil {
		setCond(metav1.Condition{
			Type:               conditionActive,
			Status:             metav1.ConditionFalse,
			Reason:             "UnsafeConfig",
			Message:            "namespaceSelector.include is empty; set podSelector to limit pods",
			ObservedGeneration: ipr.Generation,
			LastTransitionTime: metav1.NewTime(now()),
		})
		requeueAfter = interval
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	var podLabelSel client.MatchingLabelsSelector
	podLabelSelectorString := ""
	if ipr.Spec.PodSelector != nil {
		ls, err := metav1.LabelSelectorAsSelector(ipr.Spec.PodSelector)
		if err != nil {
			setCond(metav1.Condition{
				Type:               conditionActive,
				Status:             metav1.ConditionFalse,
				Reason:             "InvalidPodSelector",
				Message:            err.Error(),
				ObservedGeneration: ipr.Generation,
				LastTransitionTime: metav1.NewTime(now()),
			})
			requeueAfter = interval
			return ctrl.Result{RequeueAfter: interval}, nil
		}
		podLabelSel = client.MatchingLabelsSelector{Selector: ls}
		podLabelSelectorString = ls.String()
	}

	var (
		namespacesMatched        int
		podsListed               int
		podsEligible             int
		podsSkippedCooldown      int
		podsMaxPodsSkipped       int
		podsMetricsNotFound      int
		podsMetricsErrorsOther   int
		podsQoSSkipped           int
		podsConflictSkipped      int
		podsNoChange             int
		podsPendingStabilization int
		podsBudgetSkipped        int
		podsApplyFailed          int
		podsResized              int
		boundedCPUMinTotal       int
		boundedCPUMaxTotal       int
		boundedMemMinTotal       int
		boundedMemMaxTotal       int
	)

	type nsAgg struct {
		resized   int
		deltaCPUm int64
		deltaMemB int64
	}
	nsAggs := make(map[string]*nsAgg)

	totalResized := 0
	var totalDelta policy.Delta
	metricsOK := false
	metricsErrMessage := ""
	stopAll := false

	maxConcurrentPods := ipr.Spec.Timing.MaxConcurrentPods
	if maxConcurrentPods <= 0 {
		maxConcurrentPods = 10
	}
	if maxConcurrentPods > maxPods {
		maxConcurrentPods = maxPods
	}

	// This reduces list_pods from O(namespaces) to O(1).
	candidates := make([]*corev1.Pod, 0, 256)
	{
		var pods corev1.PodList
		listOpts := []client.ListOption{}
		if ipr.Spec.PodSelector != nil {
			listOpts = append(listOpts, podLabelSel)
		}
		observability.IncAPICall(ipr.Name, "list_pods_clusterwide")
		if err := r.List(ctx, &pods, listOpts...); err != nil {
			log.Error(err, "List pods failed (cluster-wide)")
			stopAll = true
		} else {
			podsListed = len(pods.Items)
		}
		for i := range pods.Items {
			pod := &pods.Items[i]
			if !selector.Match(pod.Namespace) {
				continue
			}
			namespacesMatched++ // count distinct matched namespaces below
			if !isEligiblePod(pod) {
				continue
			}
			podsEligible++
			if shouldSkipByCooldown(pod, cooldown, now()) {
				podsSkippedCooldown++
				continue
			}
			candidates = append(candidates, pod)
		}

		if !stopAll {
			seen := make(map[string]struct{}, 16)
			namespacesMatched = 0
			for i := range pods.Items {
				ns := pods.Items[i].Namespace
				if selector.Match(ns) {
					if _, ok := seen[ns]; !ok {
						seen[ns] = struct{}{}
						namespacesMatched++
					}
				}
			}
		}
	}

	type budgetManager struct {
		mu        sync.Mutex
		committed policy.Delta
		reserved  policy.Delta
		maxCPUm   int64
		maxMemB   int64
	}
	newBudgetManager := func() *budgetManager {
		var b budgetManager
		if ipr.Spec.Timing.MaxTotalCPUIncrease != nil {
			b.maxCPUm = ipr.Spec.Timing.MaxTotalCPUIncrease.MilliValue()
		}
		if ipr.Spec.Timing.MaxTotalMemoryIncrease != nil {
			b.maxMemB = ipr.Spec.Timing.MaxTotalMemoryIncrease.Value()
		}
		return &b
	}
	tryReserveBudget := func(b *budgetManager, add policy.Delta) bool {
		if b == nil {
			return true
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		nextCPU := b.committed.CPURequestMilli + b.reserved.CPURequestMilli + add.CPURequestMilli
		nextMem := b.committed.MemoryRequestByte + b.reserved.MemoryRequestByte + add.MemoryRequestByte
		if b.maxCPUm > 0 && nextCPU > b.maxCPUm {
			return false
		}
		if b.maxMemB > 0 && nextMem > b.maxMemB {
			return false
		}
		b.reserved.CPURequestMilli += add.CPURequestMilli
		b.reserved.MemoryRequestByte += add.MemoryRequestByte
		return true
	}
	releaseBudget := func(b *budgetManager, add policy.Delta) {
		if b == nil {
			return
		}
		b.mu.Lock()
		b.reserved.CPURequestMilli -= add.CPURequestMilli
		b.reserved.MemoryRequestByte -= add.MemoryRequestByte
		b.mu.Unlock()
	}
	commitBudget := func(b *budgetManager, add policy.Delta) {
		if b == nil {
			return
		}
		b.mu.Lock()
		b.reserved.CPURequestMilli -= add.CPURequestMilli
		b.reserved.MemoryRequestByte -= add.MemoryRequestByte
		b.committed.CPURequestMilli += add.CPURequestMilli
		b.committed.MemoryRequestByte += add.MemoryRequestByte
		b.mu.Unlock()
	}
	committedDelta := func(b *budgetManager) policy.Delta {
		if b == nil {
			return policy.Delta{}
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.committed
	}

	bm := newBudgetManager()
	slots := make(chan struct{}, maxPods)
	for i := int32(0); i < maxPods; i++ {
		slots <- struct{}{}
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex
	fatalOnce := sync.Once{}

	type podKey struct{ namespace, name string }
	allMetrics := make(map[podKey]*metricsv1beta1.PodMetrics)
	var metricsErr error
	var metricsListed bool

	if !stopAll && len(candidates) > 0 {
		observability.IncAPICall(ipr.Name, "list_pod_metrics_clusterwide")
		mctx, mcancel := context.WithTimeout(runCtx, metricsTimeout)
		list, err := r.Metrics.ListPodMetrics(mctx, "", podLabelSelectorString)
		mcancel()
		if err != nil {
			metricsErr = err
		} else {
			metricsListed = true
			for i := range list.Items {
				pm := list.Items[i].DeepCopy()
				allMetrics[podKey{pm.Namespace, pm.Name}] = pm
			}
		}
	}

	getPodMetrics := func(namespace, podName string) (*metricsv1beta1.PodMetrics, error) {
		if metricsErr != nil {
			return nil, metricsErr
		}
		if !metricsListed {
			return nil, nil
		}
		return allMetrics[podKey{namespace, podName}], nil
	}

	getFreshPod := func(namespace, name string) (*corev1.Pod, error) {
		var out corev1.Pod
		key := client.ObjectKey{Namespace: namespace, Name: name}
		reader := client.Reader(r.Client)
		if r != nil && r.APIReader != nil {
			reader = r.APIReader
		}
		if err := reader.Get(runCtx, key, &out); err != nil {
			return nil, err
		}
		return &out, nil
	}

	processOne := func(pod *corev1.Pod) {
		if pod == nil {
			return
		}
		if runCtx.Err() != nil {
			return
		}

		slotReserved := false
		select {
		case <-slots:
			slotReserved = true
		default:
			mu.Lock()
			podsMaxPodsSkipped++
			mu.Unlock()
			return
		}
		applied := false
		defer func() {
			if slotReserved && !applied {
				slots <- struct{}{}
			}
		}()

		pm, err := getPodMetrics(pod.Namespace, pod.Name)
		if err != nil {
			mu.Lock()
			podsMetricsErrorsOther++
			if metricsErrMessage == "" {
				metricsErrMessage = err.Error()
			}
			mu.Unlock()
			log.Error(err, "Get pod metrics failed", "pod", pod.Name, "namespace", pod.Namespace)
			cancel()
			return
		}
		if pm == nil {
			mu.Lock()
			podsMetricsNotFound++
			mu.Unlock()
			log.V(1).Info("Pod metrics not found yet; skipping", "pod", pod.Name, "namespace", pod.Namespace)
			return
		}
		mu.Lock()
		metricsOK = true
		mu.Unlock()

		res, err := policy.ComputeDesired(pod, pm, ipr.Spec)
		if err != nil {
			var q *policy.QoSChangeError
			if errors.As(err, &q) {
				mu.Lock()
				podsQoSSkipped++
				mu.Unlock()
				log.V(1).Info("Skipping pod because QoS would change", "pod", pod.Name, "namespace", pod.Namespace, "from", q.From, "to", q.To)
				return
			}
			log.Error(err, "Compute desired resources failed", "pod", pod.Name, "namespace", pod.Namespace)
			return
		}
		observability.AddClamped(ipr.Name, "cpu", res.ClampedCPU)
		observability.AddClamped(ipr.Name, "memory", res.ClampedMem)
		observability.AddBounded(ipr.Name, "cpu", "min", res.BoundedCPUMin)
		observability.AddBounded(ipr.Name, "cpu", "max", res.BoundedCPUMax)
		observability.AddBounded(ipr.Name, "memory", "min", res.BoundedMemMin)
		observability.AddBounded(ipr.Name, "memory", "max", res.BoundedMemMax)
		observability.AddBoundedNamespace(ipr.Name, pod.Namespace, "cpu", "min", res.BoundedCPUMin)
		observability.AddBoundedNamespace(ipr.Name, pod.Namespace, "cpu", "max", res.BoundedCPUMax)
		observability.AddBoundedNamespace(ipr.Name, pod.Namespace, "memory", "min", res.BoundedMemMin)
		observability.AddBoundedNamespace(ipr.Name, pod.Namespace, "memory", "max", res.BoundedMemMax)
		mu.Lock()
		boundedCPUMinTotal += res.BoundedCPUMin
		boundedCPUMaxTotal += res.BoundedCPUMax
		boundedMemMinTotal += res.BoundedMemMin
		boundedMemMaxTotal += res.BoundedMemMax
		mu.Unlock()
		if !res.Changed {
			mu.Lock()
			podsNoChange++
			mu.Unlock()
			return
		}
		if res.Delta.CPURequestMilli == 0 && res.Delta.MemoryRequestByte == 0 {
			mu.Lock()
			podsNoChange++
			mu.Unlock()
			log.V(1).Info("Skipping resize because request delta is zero", "pod", pod.Name, "namespace", pod.Namespace)
			return
		}

		desiredHash := res.Hash
		t := now()
		if !stabilizationAllows(pod, desiredHash, stab, t) {
			pctx, pcancel := context.WithTimeout(runCtx, patchTimeout)
			if err := r.ensurePending(pctx, pod, desiredHash, t); err != nil {
				log.Error(err, "Patch pending annotations failed", "pod", pod.Name, "namespace", pod.Namespace)
			}
			pcancel()
			mu.Lock()
			podsPendingStabilization++
			mu.Unlock()
			return
		}

		if !tryReserveBudget(bm, res.Delta) {
			mu.Lock()
			podsBudgetSkipped++
			mu.Unlock()
			return
		}
		budgetReserved := true
		defer func() {
			if budgetReserved && !applied {
				releaseBudget(bm, res.Delta)
			}
		}()

		if r.Resizer == nil || cap.State != resize.SupportSupported {
			mu.Lock()
			podsApplyFailed++
			mu.Unlock()
			return
		}

		observability.IncAPICall(ipr.Name, "update_resize_apply")
		{
			actx, acancel := context.WithTimeout(runCtx, resizeTimeout)
			_, err := r.Resizer.UpdateResize(actx, pod, res.Desired)
			acancel()
			if err != nil {
				if apierrors.IsConflict(err) {
					if fresh, gerr := getFreshPod(pod.Namespace, pod.Name); gerr == nil && fresh != nil {
						desired2 := res.Desired.DeepCopy()
						desired2.ResourceVersion = fresh.ResourceVersion
						desired2.UID = fresh.UID
						actx2, acancel2 := context.WithTimeout(runCtx, resizeTimeout)
						_, err2 := r.Resizer.UpdateResize(actx2, fresh, desired2)
						acancel2()
						if err2 == nil {
							res.Desired = desired2
							goto applyOK
						}
					}
					mu.Lock()
					podsConflictSkipped++
					mu.Unlock()
					log.V(1).Info("Resize apply conflict; will retry next reconcile", "pod", pod.Name, "namespace", pod.Namespace)
					return
				}
				mu.Lock()
				podsApplyFailed++
				mu.Unlock()
				log.Error(err, "Resize apply failed", "pod", pod.Name, "namespace", pod.Namespace)

				var u *resize.UnsupportedError
				var f *resize.ForbiddenError
				if errors.As(err, &u) || errors.As(err, &f) {
					fatalOnce.Do(func() {
						c := resize.Capability{
							State:   resize.SupportUnsupported,
							Reason:  "ApplyFailed",
							Message: err.Error(),
							Checked: now(),
						}
						if r.Resizer != nil {
							r.Resizer.Mark(c)
						}
						observability.ObserveCapability(ipr.Name, string(c.State))
						observability.IncCapabilityRuntimeProbeFailure(ipr.Name, "apply")
						ipr.Status.LastCapabilityCheckTime = &metav1.Time{Time: c.Checked}
						setCond(metav1.Condition{
							Type:               conditionInPlaceResizeSupport,
							Status:             metav1.ConditionFalse,
							Reason:             c.Reason,
							Message:            c.Message,
							ObservedGeneration: ipr.Generation,
							LastTransitionTime: metav1.NewTime(now()),
						})
						stopAll = true
						cancel()
					})
				}
				return
			}
		}
	applyOK:

		commitBudget(bm, res.Delta)
		applied = true

		mu.Lock()
		podsResized++
		totalResized++
		agg := nsAggs[pod.Namespace]
		if agg == nil {
			agg = &nsAgg{}
			nsAggs[pod.Namespace] = agg
		}
		agg.resized++
		agg.deltaCPUm += res.Delta.CPURequestMilli
		agg.deltaMemB += res.Delta.MemoryRequestByte
		mu.Unlock()

		observability.IncAPICall(ipr.Name, "patch_pod_annotations")
		pctx, pcancel := context.WithTimeout(runCtx, patchTimeout)
		if err := r.patchPodAnnotations(pctx, pod, map[string]string{
			annotationLastResizeTime:  formatTime(t),
			annotationLastAppliedHash: desiredHash,
			annotationPendingHash:     "",
			annotationPendingSince:    "",
		}); err != nil {
			log.Error(err, "Patch resize annotations failed", "pod", pod.Name, "namespace", pod.Namespace)
		}
		pcancel()
		if r.Recorder != nil {
			deltaCPU := resource.NewMilliQuantity(res.Delta.CPURequestMilli, resource.DecimalSI).String()
			deltaMem := resource.NewQuantity(res.Delta.MemoryRequestByte, resource.BinarySI).String()
			r.Recorder.Eventf(pod, corev1.EventTypeNormal, "Resized", "In-place resize applied: deltaCPU=%s deltaMem=%s", deltaCPU, deltaMem)
		}
	}

	if !stopAll && len(candidates) > 0 && maxConcurrentPods > 0 {
		workCh := make(chan *corev1.Pod)
		var wg sync.WaitGroup
		for i := int32(0); i < maxConcurrentPods; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for pod := range workCh {
					if runCtx.Err() != nil {
						continue
					}
					processOne(pod)
				}
			}()
		}

	enqueue:
		for _, pod := range candidates {
			select {
			case <-runCtx.Done():
				break enqueue
			case workCh <- pod:
			}
		}
		close(workCh)
		wg.Wait()
	}

	totalDelta = committedDelta(bm)

	elapsed := now().Sub(started)
	requeueAfter = interval
	if interval > 0 && elapsed > 0 {
		if elapsed >= interval {
			requeueAfter = 0
		} else {
			requeueAfter = interval - elapsed
		}
	}

	log.Info("Reconcile summary",
		"namespacesMatched", namespacesMatched,
		"podsListed", podsListed,
		"podsEligible", podsEligible,
		"podsSkippedCooldown", podsSkippedCooldown,
		"podsMaxPodsSkipped", podsMaxPodsSkipped,
		"podsQoSSkipped", podsQoSSkipped,
		"podsConflictSkipped", podsConflictSkipped,
		"podsNoChange", podsNoChange,
		"podsPendingStabilization", podsPendingStabilization,
		"podsBudgetSkipped", podsBudgetSkipped,
		"podsMetricsNotFound", podsMetricsNotFound,
		"podsMetricsErrorsOther", podsMetricsErrorsOther,
		"podsApplyFailed", podsApplyFailed,
		"podsResized", podsResized,
		"boundedCPU.min", boundedCPUMinTotal,
		"boundedCPU.max", boundedCPUMaxTotal,
		"boundedMem.min", boundedMemMinTotal,
		"boundedMem.max", boundedMemMaxTotal,
		"deltaCPU(m)", totalDelta.CPURequestMilli,
		"deltaMem(Mi)", bytesToMi(totalDelta.MemoryRequestByte),
		"requeueAfter", requeueAfter.String(),
	)

	observability.ObserveRun(ipr.Name, started, observability.RunStats{
		PodsListed:               podsListed,
		PodsEligible:             podsEligible,
		PodsSkippedCooldown:      podsSkippedCooldown,
		PodsMaxPodsSkipped:       podsMaxPodsSkipped,
		PodsPendingStabilization: podsPendingStabilization,
		PodsBudgetSkipped:        podsBudgetSkipped,
		PodsMetricsNotFound:      podsMetricsNotFound,
		PodsMetricsErrorsOther:   podsMetricsErrorsOther,
		PodsQoSSkipped:           podsQoSSkipped,
		PodsConflictSkipped:      podsConflictSkipped,
		PodsNoChange:             podsNoChange,
		PodsApplyFailed:          podsApplyFailed,
		PodsResized:              podsResized,
		MetricsOK:                metricsOK,
		DeltaCPURequestMilli:     totalDelta.CPURequestMilli,
		DeltaMemoryRequestByte:   totalDelta.MemoryRequestByte,
	})
	for ns, a := range nsAggs {
		observability.ObserveNamespace(ipr.Name, ns, a.resized, a.deltaCPUm, a.deltaMemB)
	}

	if metricsOK {
		setCond(metav1.Condition{
			Type:               conditionMetricsAvailable,
			Status:             metav1.ConditionTrue,
			Reason:             "OK",
			Message:            "metrics API available",
			ObservedGeneration: ipr.Generation,
			LastTransitionTime: metav1.NewTime(now()),
		})
	} else if podsMetricsErrorsOther > 0 {
		setCond(metav1.Condition{
			Type:               conditionMetricsAvailable,
			Status:             metav1.ConditionFalse,
			Reason:             "Unavailable",
			Message:            metricsErrMessage,
			ObservedGeneration: ipr.Generation,
			LastTransitionTime: metav1.NewTime(now()),
		})
	} else if podsMetricsNotFound > 0 {
		setCond(metav1.Condition{
			Type:               conditionMetricsAvailable,
			Status:             metav1.ConditionUnknown,
			Reason:             "PodMetricsNotReady",
			Message:            "pod metrics not found yet (fresh pods); will retry",
			ObservedGeneration: ipr.Generation,
			LastTransitionTime: metav1.NewTime(now()),
		})
	} else {
		setCond(metav1.Condition{
			Type:               conditionMetricsAvailable,
			Status:             metav1.ConditionUnknown,
			Reason:             "NoEligiblePods",
			Message:            "no eligible pods to query metrics",
			ObservedGeneration: ipr.Generation,
			LastTransitionTime: metav1.NewTime(now()),
		})
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *InPlacePodResizeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resizev1alpha1.InPlacePodResize{}).
		Named("inplacepodresize").
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

func isEligiblePod(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	if pod.DeletionTimestamp != nil {
		return false
	}
	switch pod.Status.Phase {
	case corev1.PodRunning:
	default:
		return false
	}
	if pod.Spec.NodeName == "" {
		return false
	}
	return true
}

func shouldSkipByCooldown(pod *corev1.Pod, cooldown time.Duration, now time.Time) bool {
	if cooldown <= 0 {
		return false
	}
	if pod == nil || pod.Annotations == nil {
		return false
	}
	t, ok := pod.Annotations[annotationLastResizeTime]
	if !ok || t == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339Nano, t)
	if err != nil {
		return false
	}
	return now.Sub(parsed) < cooldown
}

func stabilizationAllows(pod *corev1.Pod, desiredHash string, window time.Duration, now time.Time) bool {
	if window <= 0 {
		return true
	}
	if pod == nil || pod.Annotations == nil {
		return false
	}
	if pod.Annotations[annotationLastAppliedHash] == desiredHash {
		return false
	}
	pending := pod.Annotations[annotationPendingHash]
	if pending != desiredHash {
		return false
	}
	since := pod.Annotations[annotationPendingSince]
	if since == "" {
		return false
	}
	ts, err := time.Parse(time.RFC3339Nano, since)
	if err != nil {
		return false
	}
	return now.Sub(ts) >= window
}

func (r *InPlacePodResizeReconciler) patchPodAnnotations(ctx context.Context, pod *corev1.Pod, set map[string]string) error {
	if pod == nil {
		return nil
	}

	ann := map[string]*string{}
	for k, v := range set {
		if v == "" {
			ann[k] = nil
			continue
		}
		v := v
		ann[k] = &v
	}
	patchObj := map[string]any{
		"metadata": map[string]any{
			"annotations": ann,
		},
	}
	b, err := json.Marshal(patchObj)
	if err != nil {
		return err
	}
	return r.Patch(ctx, pod, client.RawPatch(types.MergePatchType, b))
}

func (r *InPlacePodResizeReconciler) ensurePending(ctx context.Context, pod *corev1.Pod, desiredHash string, now time.Time) error {
	if pod == nil {
		return nil
	}
	formatTime := func(t time.Time) string {
		if r != nil && r.TimeLocation != nil {
			return t.In(r.TimeLocation).Format(time.RFC3339Nano)
		}
		return t.UTC().Format(time.RFC3339Nano)
	}
	pending := ""
	since := ""
	if pod.Annotations != nil {
		pending = pod.Annotations[annotationPendingHash]
		since = pod.Annotations[annotationPendingSince]
	}
	patch := map[string]string{}
	if pending != desiredHash {
		patch[annotationPendingHash] = desiredHash
		patch[annotationPendingSince] = formatTime(now)
	} else if since == "" {
		patch[annotationPendingSince] = formatTime(now)
	}
	if len(patch) == 0 {
		return nil
	}
	return r.patchPodAnnotations(ctx, pod, patch)
}

func bytesToMi(v int64) int64 {
	const mi = 1024 * 1024
	if mi == 0 {
		return v
	}
	return v / mi
}
