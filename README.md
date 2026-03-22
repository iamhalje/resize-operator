# resize-operator (alpha)

## Requirements

- **metrics-server** (pod usage via `metrics.k8s.io`)
- Kubernetes cluster with **in-place Pod resize support** (`pods/resize` subresource). The operator checks capability and degrades gracefully if unsupported.

## Why

In environments resource requests are often set “with big headroom” or copied from templates:

- **requests >> real usage** → scheduler thinks the cluster is more full than it is → fewer Pods can be scheduled
- real workload needs change dynamically, but requests stay static

`resize-operator` periodically adjusts CPU/memory **requests** closer to observed usage and applies changes **without restarts** using `pods/resize`.

## How it works

On each reconcile loop the operator:

- selects candidate Pods (namespace selector + pod label selector)
- fetches `PodMetrics` from `metrics.k8s.io`
- computes desired requests based on the policy
- executes `pods/resize` (dry-run + apply)
- writes annotations and emits `Resized` events

```mermaid
%%{init: {'flowchart': {'nodeSpacing': 60, 'rankSpacing': 80}}}%%
flowchart TB

  subgraph CONTROL_PLANE["Kubernetes Control Plane"]
    APIS[kube-apiserver]
    SCHED[scheduler]
  end

  subgraph OPERATOR["Resize Operator"]
    OP[resize-operator]
  end

  subgraph NODE["Node"]
    KUBELET[kubelet]
  end

  subgraph METRICS["Metrics"]
    MS[metrics-server]
  end

  MS -->|PodMetrics| OP
  OP -->|pods/resize| APIS
  APIS --> KUBELET
  KUBELET -->|resource update| SCHED
```

## Resize logic

The operator computes new `requests` based on the current resource usage from `metrics.k8s.io`, using `headroomPercent`.

- **Target**: `target = usage * (1 + headroomPercent/100)`
- **Rounding**: CPU is rounded to `50m` steps, memory to `64Mi` steps
- **Small changes are skipped** based on:
  - relative thresholds `upPercent` / `downPercent`
  - absolute minimum change `minChangeCPU` / `minChangeMemory`
- **Bounds**: computed `requests` are clamped to configured min/max
- **Limits** behavior is controlled by `limitsMode`:
  - `Unchanged`: keep limits as-is
  - `EqualRequests`: set limits equal to requests
- **Anti-thrash**:
  - `stabilizationWindow`: the desired target must stay stable before apply
  - `cooldown`: skip pods shortly after a resize

## Observability

- Prometheus metrics are exposed on `/metrics`
- Grafana dashboard: [docs/grafana/resize-operator-dashboard.json](docs/grafana/resize-operator-dashboard.json)

## More docs

- CRD reference: [docs/crd.md](docs/crd.md)
