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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamespaceMatchType string

const (
	NamespaceMatchTypeGlob   NamespaceMatchType = "Glob"
	NamespaceMatchTypeRegexp NamespaceMatchType = "Regexp"
)

type NamespaceSelector struct {
	// +kubebuilder:validation:Enum=Glob;Regexp
	// +kubebuilder:default=Glob
	// +optional
	MatchType NamespaceMatchType `json:"matchType,omitempty"`

	// +optional
	Include []string `json:"include,omitempty"`

	// +optional
	Exclude []string `json:"exclude,omitempty"`
}

type ResourceToggles struct {
	// +optional
	CPU bool `json:"cpu,omitempty"`

	// +optional
	Memory bool `json:"memory,omitempty"`
}

type LimitsMode string

const (
	LimitsModeUnchanged     LimitsMode = "Unchanged"
	LimitsModeEqualRequests LimitsMode = "EqualRequests"
)

type ManagedResourcesSpec struct {
	Requests ResourceToggles `json:"requests"`

	// +kubebuilder:validation:Enum=Unchanged;EqualRequests
	// +kubebuilder:default=Unchanged
	// +optional
	LimitsMode LimitsMode `json:"limitsMode,omitempty"`
}

type AlgorithmSpec struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=300
	// +kubebuilder:default=20
	// +optional
	HeadroomPercent int32 `json:"headroomPercent,omitempty"`

	// +kubebuilder:default=false
	// +optional
	AllowDownscale bool `json:"allowDownscale,omitempty"`
}

type BoundsSpec struct {
	// +optional
	CPUMin *resource.Quantity `json:"cpuMin,omitempty"`
	// +optional
	CPUMax *resource.Quantity `json:"cpuMax,omitempty"`

	// +optional
	MemoryMin *resource.Quantity `json:"memoryMin,omitempty"`
	// +optional
	MemoryMax *resource.Quantity `json:"memoryMax,omitempty"`

	// +optional
	MinChangeCPU *resource.Quantity `json:"minChangeCPU,omitempty"`
	// +optional
	MinChangeMemory *resource.Quantity `json:"minChangeMemory,omitempty"`
}

type ThresholdsSpec struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000
	// +kubebuilder:default=20
	// +optional
	UpPercent int32 `json:"upPercent,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000
	// +kubebuilder:default=20
	// +optional
	DownPercent int32 `json:"downPercent,omitempty"`
}

type TimingSpec struct {
	// +kubebuilder:default:="1m"
	// +optional
	Interval metav1.Duration `json:"interval,omitempty"`

	// +kubebuilder:default:="10m"
	// +optional
	Cooldown metav1.Duration `json:"cooldown,omitempty"`

	// +kubebuilder:default:="5m"
	// +optional
	StabilizationWindow metav1.Duration `json:"stabilizationWindow,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=20
	// +optional
	MaxPodsPerRun int32 `json:"maxPodsPerRun,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	// +optional
	MaxConcurrentPods int32 `json:"maxConcurrentPods,omitempty"`

	// +optional
	MaxTotalCPUIncrease *resource.Quantity `json:"maxTotalCPUIncrease,omitempty"`
	// +optional
	MaxTotalMemoryIncrease *resource.Quantity `json:"maxTotalMemoryIncrease,omitempty"`

	// +kubebuilder:default:="1h"
	// +optional
	CapabilityCheckInterval metav1.Duration `json:"capabilityCheckInterval,omitempty"`
}

type InPlacePodResizeSpec struct {
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	NamespaceSelector NamespaceSelector `json:"namespaceSelector"`

	// +optional
	PodSelector *metav1.LabelSelector `json:"podSelector,omitempty"`

	Resources ManagedResourcesSpec `json:"resources"`

	// +optional
	Algorithm AlgorithmSpec `json:"algorithm,omitempty"`

	// +optional
	Bounds BoundsSpec `json:"bounds,omitempty"`

	// +optional
	Thresholds ThresholdsSpec `json:"thresholds,omitempty"`

	// +optional
	Timing TimingSpec `json:"timing,omitempty"`
}

type InPlacePodResizeStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	LastRunTime *metav1.Time `json:"lastRunTime,omitempty"`

	// +optional
	LastCapabilityCheckTime *metav1.Time `json:"lastCapabilityCheckTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

type InPlacePodResize struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec InPlacePodResizeSpec `json:"spec"`

	// +optional
	Status InPlacePodResizeStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

type InPlacePodResizeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []InPlacePodResize `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InPlacePodResize{}, &InPlacePodResizeList{})
}
