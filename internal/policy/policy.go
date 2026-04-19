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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	resizev1alpha1 "github.com/iamhalje/resize-operator/api/v1alpha1"
)

// const (
// 	// CPU, in cores. (500m = .5 cores)
// 	ResourceCPU ResourceName = "cpu"
// 	// Memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
// 	ResourceMemory ResourceName = "memory"
// 	// Volume size, in bytes (e,g. 5Gi = 5GiB = 5 * 1024 * 1024 * 1024)
// 	ResourceStorage ResourceName = "storage"
// 	// Local ephemeral storage, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
// 	ResourceEphemeralStorage ResourceName = "ephemeral-storage"
// )

type Delta struct {
	CPURequestMilli   int64
	MemoryRequestByte int64
}

const (
	cpuStepMilli    int64 = 50
	memoryStepBytes int64 = 64 * 1024 * 1024
)

type Result struct {
	Desired       *corev1.Pod
	Hash          string
	Changed       bool
	Delta         Delta
	Containers    int
	NoMetrics     int
	SkippedDown   int
	ClampedCPU    int
	ClampedMem    int
	BoundedCPUMin int
	BoundedCPUMax int
	BoundedMemMin int
	BoundedMemMax int
}

type QoSChangeError struct {
	From corev1.PodQOSClass
	To   corev1.PodQOSClass
}

func (e *QoSChangeError) Error() string {
	return fmt.Sprintf("pod QoS class may not change (%s -> %s)", e.From, e.To)
}

func ComputeDesired(pod *corev1.Pod, pm *metricsv1beta1.PodMetrics, spec resizev1alpha1.InPlacePodResizeSpec) (Result, error) {
	if pod == nil {
		return Result{}, fmt.Errorf("pod is nil")
	}
	if pm == nil {
		return Result{}, fmt.Errorf("pod metrics is nil")
	}
	if !spec.Resources.Requests.CPU && !spec.Resources.Requests.Memory {
		return Result{Desired: pod.DeepCopy(), Hash: hashPodResources(pod), Changed: false}, nil
	}

	currentQoS := podQoSClass(pod)
	limitsMode := spec.Resources.LimitsMode
	if limitsMode == "" {
		limitsMode = resizev1alpha1.LimitsModeUnchanged
	}
	if currentQoS != corev1.PodQOSGuaranteed && limitsMode == resizev1alpha1.LimitsModeEqualRequests {
		limitsMode = resizev1alpha1.LimitsModeUnchanged
	}

	usageByContainer := make(map[string]corev1.ResourceList, len(pm.Containers))
	for _, c := range pm.Containers {
		usageByContainer[c.Name] = c.Usage
	}

	headroom := int64(100 + spec.Algorithm.HeadroomPercent)
	allowDown := spec.Algorithm.AllowDownscale
	upPct := int64(spec.Thresholds.UpPercent)
	downPct := int64(spec.Thresholds.DownPercent)

	desired := pod.DeepCopy()
	var res Result
	res.Desired = desired

	for i := range desired.Spec.Containers {
		c := &desired.Spec.Containers[i]
		res.Containers++

		usage, ok := usageByContainer[c.Name]
		if !ok {
			res.NoMetrics++
			continue
		}

		curReq := c.Resources.Requests.DeepCopy()
		curLim := c.Resources.Limits.DeepCopy()

		newReq := curReq.DeepCopy()
		newLim := curLim.DeepCopy()
		if newReq == nil {
			newReq = corev1.ResourceList{}
		}
		if newLim == nil {
			newLim = corev1.ResourceList{}
		}

		if spec.Resources.Requests.CPU {
			usageMilli := quantityMilliOrZero(usage, corev1.ResourceCPU)
			curMilli := quantityMilliOrZero(curReq, corev1.ResourceCPU)
			targetMilli := divCeil(usageMilli*headroom, 100)
			targetMilli = quantizeMilliCPU(targetMilli, curMilli, allowDown)
			decidedMilli, changed, downSkipped := decide(targetMilli, curMilli, upPct, downPct, allowDown, quantityMilli(spec.Bounds.MinChangeCPU))
			if downSkipped {
				res.SkippedDown++
			}
			if changed {
				newReq[corev1.ResourceCPU] = *resource.NewMilliQuantity(decidedMilli, resource.DecimalSI)
			}
		}

		if spec.Resources.Requests.Memory {
			usageBytes := quantityBytesOrZero(usage, corev1.ResourceMemory)
			curBytes := quantityBytesOrZero(curReq, corev1.ResourceMemory)
			targetBytes := divCeil(usageBytes*headroom, 100)
			targetBytes = quantizeBytesMemory(targetBytes, curBytes, allowDown)
			decidedBytes, changed, downSkipped := decide(targetBytes, curBytes, upPct, downPct, allowDown, quantityBytes(spec.Bounds.MinChangeMemory))
			if downSkipped {
				res.SkippedDown++
			}
			if changed {
				newReq[corev1.ResourceMemory] = *resource.NewQuantity(decidedBytes, resource.BinarySI)
			}
		}

		cpuMin, cpuMax := applyBounds(&newReq, corev1.ResourceCPU, spec.Bounds.CPUMin, spec.Bounds.CPUMax)
		memMin, memMax := applyBounds(&newReq, corev1.ResourceMemory, spec.Bounds.MemoryMin, spec.Bounds.MemoryMax)
		if cpuMin {
			res.BoundedCPUMin++
		}
		if cpuMax {
			res.BoundedCPUMax++
		}
		if memMin {
			res.BoundedMemMin++
		}
		if memMax {
			res.BoundedMemMax++
		}

		switch limitsMode {
		case resizev1alpha1.LimitsModeEqualRequests:
			if spec.Resources.Requests.CPU {
				newLim[corev1.ResourceCPU] = newReq[corev1.ResourceCPU].DeepCopy()
			}
			if spec.Resources.Requests.Memory {
				newLim[corev1.ResourceMemory] = newReq[corev1.ResourceMemory].DeepCopy()
			}
		case "", resizev1alpha1.LimitsModeUnchanged:
			if lim, ok := curLim[corev1.ResourceCPU]; ok && spec.Resources.Requests.CPU {
				if req, ok2 := newReq[corev1.ResourceCPU]; ok2 && req.Cmp(lim) > 0 {
					res.ClampedCPU++
					newReq[corev1.ResourceCPU] = lim.DeepCopy()
				}
			}
			if lim, ok := curLim[corev1.ResourceMemory]; ok && spec.Resources.Requests.Memory {
				if req, ok2 := newReq[corev1.ResourceMemory]; ok2 && req.Cmp(lim) > 0 {
					res.ClampedMem++
					newReq[corev1.ResourceMemory] = lim.DeepCopy()
				}
			}
		default:
			return Result{}, fmt.Errorf("unsupported limitsMode %q", spec.Resources.LimitsMode)
		}

		if !resourceListEqual(curReq, newReq) || !resourceListEqual(curLim, newLim) {
			c.Resources.Requests = newReq
			c.Resources.Limits = newLim
			res.Changed = true
		}
	}

	if res.Changed {
		desiredQoS := podQoSClass(desired)
		if desiredQoS != currentQoS {
			if currentQoS == corev1.PodQOSBurstable && desiredQoS == corev1.PodQOSGuaranteed {
				if forceBurstable(desired, spec) {
					desiredQoS = podQoSClass(desired)
				}
			}
		}
		if desiredQoS != currentQoS {
			return Result{Desired: pod.DeepCopy(), Hash: hashPodResources(pod), Changed: false}, &QoSChangeError{
				From: currentQoS,
				To:   desiredQoS,
			}
		}
	}

	res.Delta = computeDeltaRequests(pod, desired)
	res.Hash = hashPodResources(desired)
	return res, nil
}

func hashPodResources(pod *corev1.Pod) string {
	type c struct {
		Name string            `json:"name"`
		Req  map[string]string `json:"req,omitempty"`
		Lim  map[string]string `json:"lim,omitempty"`
	}
	if pod == nil {
		return ""
	}

	allContainers := make([]corev1.Container, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers))
	allContainers = append(allContainers, pod.Spec.InitContainers...)
	allContainers = append(allContainers, pod.Spec.Containers...)

	cs := make([]c, 0, len(allContainers))
	for _, ctr := range allContainers {
		item := c{Name: ctr.Name}
		if len(ctr.Resources.Requests) > 0 {
			item.Req = make(map[string]string, len(ctr.Resources.Requests))
			for k, v := range ctr.Resources.Requests {
				item.Req[string(k)] = v.String()
			}
		}
		if len(ctr.Resources.Limits) > 0 {
			item.Lim = make(map[string]string, len(ctr.Resources.Limits))
			for k, v := range ctr.Resources.Limits {
				item.Lim[string(k)] = v.String()
			}
		}
		cs = append(cs, item)
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].Name < cs[j].Name })

	b, _ := json.Marshal(cs)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func resourceListEqual(a, b corev1.ResourceList) bool {
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			if !av.IsZero() {
				return false
			}
			continue
		}
		if av.Cmp(bv) != 0 {
			return false
		}
	}
	for k, bv := range b {
		av, ok := a[k]
		if !ok {
			if !bv.IsZero() {
				return false
			}
			continue
		}
		if av.Cmp(bv) != 0 {
			return false
		}
	}
	return true
}

func quantityMilli(q *resource.Quantity) int64 {
	if q == nil {
		return 0
	}
	return q.MilliValue()
}

func quantityBytes(q *resource.Quantity) int64 {
	if q == nil {
		return 0
	}
	return q.Value()
}

func quantityMilliOrZero(rl corev1.ResourceList, name corev1.ResourceName) int64 {
	q, ok := rl[name]
	if !ok {
		return 0
	}
	return q.MilliValue()
}

func quantityBytesOrZero(rl corev1.ResourceList, name corev1.ResourceName) int64 {
	q, ok := rl[name]
	if !ok {
		return 0
	}
	return q.Value()
}

func decide(target, current, upPct, downPct int64, allowDown bool, minChangeAbs int64) (int64, bool, bool) {
	if current < 0 {
		current = 0
	}
	if target < 0 {
		target = 0
	}
	if target == current {
		return current, false, false
	}
	if target > current {
		if !passesPercent(target, current, upPct, true) {
			return current, false, false
		}
		if minChangeAbs > 0 && (target-current) < minChangeAbs {
			return current, false, false
		}
		return target, true, false
	}

	if !allowDown {
		return current, false, true
	}
	if !passesPercent(target, current, downPct, false) {
		return current, false, false
	}
	if minChangeAbs > 0 && (current-target) < minChangeAbs {
		return current, false, false
	}
	return target, true, false
}

func passesPercent(target, current, pct int64, up bool) bool {
	if pct <= 0 {
		return true
	}
	if current == 0 {
		return up && target > 0
	}
	if up {
		return (target-current)*100 >= current*pct
	}
	return (current-target)*100 >= current*pct
}

func divCeil(n, d int64) int64 {
	if d == 0 {
		return n
	}
	if n == 0 {
		return 0
	}
	if n > 0 && d > 0 {
		return (n + d - 1) / d
	}
	return n / d
}

func applyBounds(rl *corev1.ResourceList, name corev1.ResourceName, minQ, maxQ *resource.Quantity) (bool, bool) {
	if rl == nil {
		return false, false
	}
	cur, ok := (*rl)[name]
	if !ok {
		cur = resource.Quantity{}
	}
	hitMin := false
	hitMax := false
	if minQ != nil && cur.Cmp(*minQ) < 0 {
		(*rl)[name] = minQ.DeepCopy()
		hitMin = true
	}
	if maxQ != nil && cur.Cmp(*maxQ) > 0 {
		(*rl)[name] = maxQ.DeepCopy()
		hitMax = true
	}
	return hitMin, hitMax
}

func quantizeMilliCPU(targetMilli, currentMilli int64, allowDown bool) int64 {
	if cpuStepMilli <= 0 {
		return targetMilli
	}
	if targetMilli > currentMilli {
		return roundUp(targetMilli, cpuStepMilli)
	}
	if targetMilli < currentMilli && allowDown {
		return roundDown(targetMilli, cpuStepMilli)
	}
	return targetMilli
}

func quantizeBytesMemory(targetBytes, currentBytes int64, allowDown bool) int64 {
	if memoryStepBytes <= 0 {
		return targetBytes
	}
	if targetBytes > currentBytes {
		return roundUp(targetBytes, memoryStepBytes)
	}
	if targetBytes < currentBytes && allowDown {
		return roundDown(targetBytes, memoryStepBytes)
	}
	return targetBytes
}

func roundUp(v, step int64) int64 {
	if step <= 0 {
		return v
	}
	if v <= 0 {
		return 0
	}
	return ((v + step - 1) / step) * step
}

func roundDown(v, step int64) int64 {
	if step <= 0 {
		return v
	}
	if v <= 0 {
		return 0
	}
	return (v / step) * step
}

func podQoSClass(pod *corev1.Pod) corev1.PodQOSClass {
	if pod == nil {
		return corev1.PodQOSBestEffort
	}

	containers := make([]corev1.Container, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers))
	containers = append(containers, pod.Spec.InitContainers...)
	containers = append(containers, pod.Spec.Containers...)

	bestEffort := true
	guaranteed := true

	for _, c := range containers {
		req := c.Resources.Requests
		lim := c.Resources.Limits

		reqCPU, reqCPUSet := req[corev1.ResourceCPU]
		reqMem, reqMemSet := req[corev1.ResourceMemory]
		limCPU, limCPUSet := lim[corev1.ResourceCPU]
		limMem, limMemSet := lim[corev1.ResourceMemory]

		if reqCPUSet || reqMemSet || limCPUSet || limMemSet {
			bestEffort = false
		}

		if !reqCPUSet || !reqMemSet || !limCPUSet || !limMemSet {
			guaranteed = false
			continue
		}
		if reqCPU.Cmp(limCPU) != 0 || reqMem.Cmp(limMem) != 0 {
			guaranteed = false
		}
	}
	if guaranteed {
		return corev1.PodQOSGuaranteed
	}
	if bestEffort {
		return corev1.PodQOSBestEffort
	}
	return corev1.PodQOSBurstable
}

func forceBurstable(desired *corev1.Pod, spec resizev1alpha1.InPlacePodResizeSpec) bool {
	if desired == nil {
		return false
	}

	minCPUMilli := int64(0)
	if spec.Bounds.CPUMin != nil {
		minCPUMilli = spec.Bounds.CPUMin.MilliValue()
	}
	minMemBytes := int64(0)
	if spec.Bounds.MemoryMin != nil {
		minMemBytes = spec.Bounds.MemoryMin.Value()
	}

	for i := range desired.Spec.Containers {
		c := &desired.Spec.Containers[i]
		if spec.Resources.Requests.CPU {
			if lim, ok := c.Resources.Limits[corev1.ResourceCPU]; ok {
				if req, ok2 := c.Resources.Requests[corev1.ResourceCPU]; ok2 && req.Cmp(lim) == 0 {
					limMilli := lim.MilliValue()
					newMilli := limMilli - cpuStepMilli
					if newMilli < minCPUMilli {
						newMilli = minCPUMilli
					}
					if newMilli < limMilli {
						c.Resources.Requests[corev1.ResourceCPU] = *resource.NewMilliQuantity(newMilli, resource.DecimalSI)
						return true
					}
				}
			}
		}
		if spec.Resources.Requests.Memory {
			if lim, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
				if req, ok2 := c.Resources.Requests[corev1.ResourceMemory]; ok2 && req.Cmp(lim) == 0 {
					limBytes := lim.Value()
					newBytes := limBytes - memoryStepBytes
					if newBytes < minMemBytes {
						newBytes = minMemBytes
					}
					if newBytes < limBytes {
						c.Resources.Requests[corev1.ResourceMemory] = *resource.NewQuantity(newBytes, resource.BinarySI)
						return true
					}
				}
			}
		}
	}
	return false
}

func computeDeltaRequests(oldPod, newPod *corev1.Pod) Delta {
	var d Delta
	if oldPod == nil || newPod == nil {
		return d
	}

	oldByName := make(map[string]corev1.ResourceList, len(oldPod.Spec.Containers))
	for i := range oldPod.Spec.Containers {
		c := &oldPod.Spec.Containers[i]
		oldByName[c.Name] = c.Resources.Requests
	}
	for i := range newPod.Spec.Containers {
		c := &newPod.Spec.Containers[i]
		oldReq := oldByName[c.Name]
		newReq := c.Resources.Requests
		d.CPURequestMilli += quantityMilliOrZero(newReq, corev1.ResourceCPU) - quantityMilliOrZero(oldReq, corev1.ResourceCPU)
		d.MemoryRequestByte += quantityBytesOrZero(newReq, corev1.ResourceMemory) - quantityBytesOrZero(oldReq, corev1.ResourceMemory)
	}
	return d
}
