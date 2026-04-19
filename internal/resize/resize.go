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
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
)

type SupportState string

const (
	SupportUnknown     SupportState = "Unknown"
	SupportSupported   SupportState = "Supported"
	SupportUnsupported SupportState = "Unsupported"
)

type Capability struct {
	State   SupportState
	Reason  string
	Message string
	Checked time.Time
}

type Prober struct {
	discovery discovery.DiscoveryInterface
	clientset kubernetes.Interface

	mu    sync.Mutex
	cache Capability
}

func NewProber(d discovery.DiscoveryInterface, cs kubernetes.Interface) *Prober {
	return &Prober{
		discovery: d,
		clientset: cs,
		cache: Capability{
			State: SupportUnknown,
		},
	}
}

type UnsupportedError struct {
	Err error
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("in-place resize unsupported: %v", e.Err)
}
func (e *UnsupportedError) Unwrap() error {
	return e.Err
}

type ForbiddenError struct {
	Err error
}

func (e *ForbiddenError) Error() string {
	return fmt.Sprintf("in-place resize forbidden: %v", e.Err)
}
func (e *ForbiddenError) Unwrap() error {
	return e.Err
}

func (p *Prober) Supported(ctx context.Context, ttl time.Duration, now time.Time) Capability {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ttl > 0 && !p.cache.Checked.IsZero() && now.Sub(p.cache.Checked) < ttl {
		return p.cache
	}

	cap := Capability{
		State:   SupportUnsupported,
		Reason:  "NotFound",
		Message: "pods/resize was not found in discovery",
		Checked: now,
	}
	if p.discovery == nil {
		cap.State = SupportUnknown
		cap.Reason = "DiscoveryClientMissing"
		cap.Message = "discovery client is not configured"
		p.cache = cap
		return cap
	}
	list, err := p.discovery.ServerResourcesForGroupVersion("v1")
	if err != nil {
		cap.State = SupportUnknown
		cap.Reason = "DiscoveryFailed"
		cap.Message = err.Error()
		p.cache = cap
		return cap
	}

	for _, r := range list.APIResources {
		if r.Name != "pods/resize" {
			continue
		}
		if !hasVerb(r.Verbs, "update") {
			cap.State = SupportUnknown
			cap.Reason = "VerbMissing"
			cap.Message = "pods/resize exists but does not advertise update verb"
			p.cache = cap
			return cap
		}
		cap.State = SupportSupported
		cap.Reason = "Discovered"
		cap.Message = "pods/resize discovered"
		p.cache = cap
		return cap
	}

	p.cache = cap
	return cap
}

func (p *Prober) Mark(cap Capability) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache = cap
}

func (p *Prober) UpdateResize(ctx context.Context, pod *corev1.Pod, desired *corev1.Pod) (*corev1.Pod, error) {
	if p == nil || p.clientset == nil {
		return nil, fmt.Errorf("kubernetes clientset is not configured")
	}
	if pod == nil || desired == nil {
		return nil, fmt.Errorf("pod/desired must not be nil")
	}
	out, err := p.clientset.CoreV1().Pods(pod.Namespace).UpdateResize(ctx, pod.Name, desired, metav1.UpdateOptions{})
	if err == nil {
		return out, nil
	}
	if isUnsupportedResizeErr(err) {
		return nil, &UnsupportedError{Err: err}
	}
	if apierrors.IsForbidden(err) {
		return nil, &ForbiddenError{Err: err}
	}
	return nil, err
}

func isUnsupportedResizeErr(err error) bool {
	if err == nil {
		return false
	}

	// HTTP 405 Method Not Allowed from the API server.
	if apierrors.IsMethodNotSupported(err) {
		return true
	}

	var se *apierrors.StatusError
	if errors.As(err, &se) {
		if isPodNotFoundStatus(se) {
			return false
		}
		code := se.ErrStatus.Code
		return code == 404 || code == 405
	}

	msg := strings.ToLower(err.Error())
	if isPodNotFoundMsg(msg) {
		return false
	}
	return strings.Contains(msg, "pods/resize") &&
		(strings.Contains(msg, "not found") || strings.Contains(msg, "method not allowed"))
}

func isPodNotFoundStatus(se *apierrors.StatusError) bool {
	d := se.ErrStatus.Details
	return se.ErrStatus.Code == 404 &&
		d != nil &&
		d.Kind == "pods" &&
		d.Name != "" &&
		d.Group == ""
}

func isPodNotFoundMsg(msg string) bool {
	return strings.Contains(msg, "pods \"") && strings.Contains(msg, "\" not found")
}

func hasVerb(verbs []string, verb string) bool {
	for _, v := range verbs {
		if v == verb {
			return true
		}
	}
	return false
}
