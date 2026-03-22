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

package metrics

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

type Client struct {
	cs metricsclientset.Interface
}

func New(cs metricsclientset.Interface) *Client {
	return &Client{
		cs: cs,
	}
}

func (c *Client) GetPodMetrics(ctx context.Context, namespace, name string) (*metricsv1beta1.PodMetrics, error) {
	if c == nil || c.cs == nil {
		return nil, fmt.Errorf("metrics client is not configured")
	}
	return c.cs.MetricsV1beta1().PodMetricses(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *Client) ListPodMetrics(ctx context.Context, namespace, labelSelector string) (*metricsv1beta1.PodMetricsList, error) {
	if c == nil || c.cs == nil {
		return nil, fmt.Errorf("metrics client is not configured")
	}
	return c.cs.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
}
