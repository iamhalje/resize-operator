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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/iamhalje/resize-operator/internal/resize"
)

const (
	annotationLastResizeTime  = "resize.halje.ru/last-resize-time"
	annotationLastAppliedHash = "resize.halje.ru/last-applied-hash"
	annotationPendingHash     = "resize.halje.ru/pending-hash"
	annotationPendingSince    = "resize.halje.ru/pending-since"
)

const (
	conditionActive               = "Active"
	conditionMetricsAvailable     = "MetricsAvailable"
	conditionInPlaceResizeSupport = "InPlaceResizeSupported"
)

type ResizerInterface interface {
	Supported(ctx context.Context, ttl time.Duration, now time.Time) resize.Capability
	UpdateResize(ctx context.Context, pod *corev1.Pod, desired *corev1.Pod) (*corev1.Pod, error)
	Mark(cap resize.Capability)
}

type MetricsInterface interface {
	ListPodMetrics(ctx context.Context, namespace, labelSelector string) (*metricsv1beta1.PodMetricsList, error)
}

type InPlacePodResizeReconciler struct {
	client.Client
	APIReader client.Reader
	Scheme    *runtime.Scheme
	Metrics   MetricsInterface
	Resizer   ResizerInterface
	Recorder  record.EventRecorder

	Now            func() time.Time
	TimeLocation   *time.Location
	MetricsTimeout time.Duration
	ResizeTimeout  time.Duration
	PatchTimeout   time.Duration
}
