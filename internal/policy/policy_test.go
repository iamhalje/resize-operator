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

package policy

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	resizev1alpha1 "github.com/iamhalje/resize-operator/api/v1alpha1"
)

func baseSpec() resizev1alpha1.InPlacePodResizeSpec {
	return resizev1alpha1.InPlacePodResizeSpec{
		Enabled: true,
		NamespaceSelector: resizev1alpha1.NamespaceSelector{
			MatchType: resizev1alpha1.NamespaceMatchTypeGlob,
			Include:   []string{"*"},
		},
		Resources: resizev1alpha1.ManagedResourcesSpec{
			Requests: resizev1alpha1.ResourceToggles{CPU: true, Memory: true},
		},
		Bounds: resizev1alpha1.BoundsSpec{
			CPUMin:       qtyPtr("0m"),
			CPUMax:       qtyPtr("100000m"),
			MemoryMin:    qtyPtr("0Mi"),
			MemoryMax:    qtyPtr("1000Gi"),
			MinChangeCPU: qtyPtr("0m"),
		},
		Thresholds: resizev1alpha1.ThresholdsSpec{
			UpPercent:   0,
			DownPercent: 0,
		},
	}
}

func TestComputeDesired_RoundsUpCPUAndMemory(t *testing.T) {
	pod := podWith("p", "ns", corev1.ResourceList{
		corev1.ResourceCPU:    mustQty("50m"),
		corev1.ResourceMemory: mustQty("64Mi"),
	}, corev1.ResourceList{})
	pm := podMetrics("p", "ns",
		"c", mustQty("101m"), mustQty("65Mi"),
	)
	spec := baseSpec()
	spec.Algorithm.HeadroomPercent = 0
	spec.Algorithm.AllowDownscale = true
	spec.Thresholds.UpPercent = 0
	spec.Thresholds.DownPercent = 0

	res, err := ComputeDesired(pod, pm, spec)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	got := res.Desired.Spec.Containers[0].Resources.Requests
	if got.Cpu().MilliValue() != 150 {
		t.Fatalf("cpu: got %dm, want 150m", got.Cpu().MilliValue())
	}
	expMem := mustQty("128Mi")
	if got.Memory().Value() != expMem.Value() {
		t.Fatalf("mem: got %d, want %d", got.Memory().Value(), expMem.Value())
	}
}

func TestComputeDesired_RoundsDownOnDownscale(t *testing.T) {
	pod := podWith("p", "ns", corev1.ResourceList{
		corev1.ResourceCPU:    mustQty("250m"),
		corev1.ResourceMemory: mustQty("256Mi"),
	}, nil)
	pm := podMetrics("p", "ns",
		"c", mustQty("51m"), mustQty("10Mi"),
	)
	spec := baseSpec()
	spec.Algorithm.HeadroomPercent = 0
	spec.Algorithm.AllowDownscale = true
	spec.Thresholds.UpPercent = 0
	spec.Thresholds.DownPercent = 0

	res, err := ComputeDesired(pod, pm, spec)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	got := res.Desired.Spec.Containers[0].Resources.Requests
	if got.Cpu().MilliValue() != 50 {
		t.Fatalf("cpu: got %dm, want 50m", got.Cpu().MilliValue())
	}
}

func TestComputeDesired_ClampToLimitWhenUnchanged(t *testing.T) {
	pod := podWith("p", "ns",
		corev1.ResourceList{
			corev1.ResourceCPU: mustQty("50m"),
		},
		corev1.ResourceList{
			corev1.ResourceCPU: mustQty("100m"),
		},
	)
	pm := podMetrics("p", "ns",
		"c", mustQty("200m"), mustQty("0Mi"),
	)
	spec := baseSpec()
	spec.Resources.LimitsMode = resizev1alpha1.LimitsModeUnchanged
	spec.Algorithm.HeadroomPercent = 0
	spec.Thresholds.UpPercent = 0

	res, err := ComputeDesired(pod, pm, spec)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	got := res.Desired.Spec.Containers[0].Resources.Requests
	if got.Cpu().MilliValue() != 100 {
		t.Fatalf("cpu: got %dm, want 100m (clamped to limit)", got.Cpu().MilliValue())
	}
	if res.ClampedCPU != 1 {
		t.Fatalf("clampedCPU: got %d, want 1", res.ClampedCPU)
	}
}

func podWith(name, ns string, req, lim corev1.ResourceList) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.PodSpec{
			NodeName: "n1",
			Containers: []corev1.Container{
				{
					Name: "c",
					Resources: corev1.ResourceRequirements{
						Requests: req,
						Limits:   lim,
					},
				},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func podMetrics(podName, ns string, container string, cpu, mem resource.Quantity) *metricsv1beta1.PodMetrics {
	return &metricsv1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: ns},
		Containers: []metricsv1beta1.ContainerMetrics{
			{
				Name: container,
				Usage: corev1.ResourceList{
					corev1.ResourceCPU:    cpu,
					corev1.ResourceMemory: mem,
				},
			},
		},
	}
}

func mustQty(s string) resource.Quantity {
	return resource.MustParse(s)
}

func qtyPtr(s string) *resource.Quantity {
	q := mustQty(s)
	return &q
}

func TestComputeDesired_PreserversBusrtableWhenClampWouldMakeGuaranteed(t *testing.T) {
	pod := podWith("p", "ns", corev1.ResourceList{
		corev1.ResourceCPU:    mustQty("50m"),
		corev1.ResourceMemory: mustQty("64Mi"),
	}, corev1.ResourceList{
		corev1.ResourceCPU:    mustQty("100m"),
		corev1.ResourceMemory: mustQty("128Mi"),
	})
	pm := podMetrics("p", "ns", "c", mustQty("5000m"), mustQty("5Gi"))

	spec := baseSpec()
	spec.Resources.LimitsMode = resizev1alpha1.LimitsModeUnchanged

	if podQoSClass(pod) != corev1.PodQOSBurstable {
		t.Fatalf("precondition: want Burstable QoS")
	}

	res, err := ComputeDesired(pod, pm, spec)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if podQoSClass(res.Desired) != corev1.PodQOSBurstable {
		t.Fatalf("want Burstable QoS, got %s", podQoSClass(res.Desired))
	}
	gotReq := res.Desired.Spec.Containers[0].Resources.Requests
	gotLim := res.Desired.Spec.Containers[0].Resources.Limits
	if gotReq.Cpu().MilliValue() == gotLim.Cpu().MilliValue() && gotReq.Memory().Value() == gotLim.Memory().Value() {
		t.Fatalf("would become Guaranteed (requests == limits for cpu+memory), but must stay Burstable")
	}
}

func TestComputeDesired_ReturnsQoSErrorWhenBestEfforstWoudlChangeQoS(t *testing.T) {
	pod := podWith("p", "ns", corev1.ResourceList{}, corev1.ResourceList{})
	pm := podMetrics("p", "ns", "c", mustQty("100m"), mustQty("100Mi"))

	spec := baseSpec()

	if podQoSClass(pod) != corev1.PodQOSBestEffort {
		t.Fatalf("precondition: want BestEffort QoS")
	}

	res, err := ComputeDesired(pod, pm, spec)
	if err == nil {
		t.Fatalf("want error")
	}
	var q *QoSChangeError
	if !errors.As(err, &q) {
		t.Fatalf("want QoSChangeError, got %T: %v", err, err)
	}
	if res.Changed {
		t.Fatalf("want Changed=false")
	}
}

func TestComputeDesired_BoundedMaxCounters(t *testing.T) {
	pod := podWith("p", "ns",
		corev1.ResourceList{
			corev1.ResourceCPU:    mustQty("100m"),
			corev1.ResourceMemory: mustQty("128Mi"),
		},
		corev1.ResourceList{},
	)
	pm := podMetrics("p", "ns", "c", mustQty("10"), mustQty("100Gi"))

	spec := baseSpec()
	cpuMax := mustQty("200m")
	memMax := mustQty("256Mi")
	spec.Bounds.CPUMax = &cpuMax
	spec.Bounds.MemoryMax = &memMax
	spec.Algorithm.HeadroomPercent = 0
	spec.Thresholds.UpPercent = 0

	res, err := ComputeDesired(pod, pm, spec)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	got := res.Desired.Spec.Containers[0].Resources.Requests
	if got.Cpu().MilliValue() != 200 {
		t.Fatalf("cpu: got %dm, want 200m (bounded by max)", got.Cpu().MilliValue())
	}
	expMem := mustQty("256Mi")
	if got.Memory().Value() != expMem.Value() {
		t.Fatalf("mem: got %d, want %d (bounded by max)", got.Memory().Value(), expMem.Value())
	}
	if res.BoundedCPUMax != 1 || res.BoundedMemMax != 1 {
		t.Fatalf("bounded: got cpu=%d mem=%d, want 1/1", res.BoundedCPUMax, res.BoundedMemMax)
	}
}
