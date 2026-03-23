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

package resize

import (
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIsUnsupportedResizeErr_PodNotFoundIsTransient(t *testing.T) {
	t.Parallel()

	err := apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, "pod-1")
	if isUnsupportedResizeErr(err) {
		t.Fatalf("expected pod not found to be treated as transient")
	}
}

func TestIsUnsupportedResizeErr_SubresourceNotFoundIsUnsupported(t *testing.T) {
	t.Parallel()

	err := apierrors.NewNotFound(schema.GroupResource{Resource: "pods/resize"}, "pod-1")
	if !isUnsupportedResizeErr(err) {
		t.Fatalf("expected pods/resize not found to be treated as unsupported")
	}
}

func TestIsUnsupportedResizeErr_MethodNotSupportedIsUnsupported(t *testing.T) {
	t.Parallel()

	err := apierrors.NewMethodNotSupported(schema.GroupResource{Resource: "pods/resize"}, "update")
	if !isUnsupportedResizeErr(err) {
		t.Fatalf("expected method not supported to be treated as unsupported")
	}
}

func TestIsUnsupportedResizeErr_MessageFallbacks(t *testing.T) {
	t.Parallel()

	if isUnsupportedResizeErr(errors.New(`pods "pod-1" not found`)) {
		t.Fatalf("expected pod not found message to be treated as transient")
	}
	if !isUnsupportedResizeErr(errors.New(`pods/resize "pod-1" not found`)) {
		t.Fatalf("expected pods/resize not found message to be treated as unsupported")
	}
}
