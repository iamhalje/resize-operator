/*
Copyright (c) 2026 Maxim Technology. Author: Dmitry Ponomaryov.

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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/iamhalje/resize-operator/internal/metrics"
	"github.com/iamhalje/resize-operator/internal/resize"
)

const (
	annotationLastResizeTime  = "resize.maxim.technology/last-resize-time"
	annotationLastAppliedHash = "resize.maxim.technology/last-applied-hash"
	annotationPendingHash     = "resize.maxim.technology/pending-hash"
	annotationPendingSince    = "resize.maxim.technology/pending-since"
)

const (
	conditionActive               = "Active"
	conditionMetricsAvailable     = "MetricsAvailable"
	conditionInPlaceResizeSupport = "InPlaceResizeSupported"
)

type InPlacePodResizeReconciler struct {
	client.Client
	APIReader client.Reader
	Scheme    *runtime.Scheme

	Metrics  *metrics.Client
	Resizer  *resize.Prober
	Recorder record.EventRecorder

	Now func() time.Time

	TimeLocation *time.Location

	MetricsTimeout time.Duration
	ResizeTimeout  time.Duration
	PatchTimeout   time.Duration
}
