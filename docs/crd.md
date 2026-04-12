# API Reference

## Packages
- [resize.halje.ru/v1alpha1](#resizemaximtechnologyv1alpha1)


## resize.halje.ru/v1alpha1

Package v1alpha1 contains API Schema definitions for the resize v1alpha1 API group.

### Resource Types
- [InPlacePodResize](#inplacepodresize)
- [InPlacePodResizeList](#inplacepodresizelist)



#### AlgorithmSpec







_Appears in:_
- [InPlacePodResizeSpec](#inplacepodresizespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `headroomPercent` _integer_ |  | 20 | Maximum: 300 <br />Minimum: 0 <br />Optional: \{\} <br /> |
| `allowDownscale` _boolean_ |  | false | Optional: \{\} <br /> |


#### BoundsSpec







_Appears in:_
- [InPlacePodResizeSpec](#inplacepodresizespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cpuMin` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#quantity-resource-api)_ |  |  | Optional: \{\} <br /> |
| `cpuMax` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#quantity-resource-api)_ |  |  | Optional: \{\} <br /> |
| `memoryMin` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#quantity-resource-api)_ |  |  | Optional: \{\} <br /> |
| `memoryMax` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#quantity-resource-api)_ |  |  | Optional: \{\} <br /> |
| `minChangeCPU` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#quantity-resource-api)_ |  |  | Optional: \{\} <br /> |
| `minChangeMemory` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#quantity-resource-api)_ |  |  | Optional: \{\} <br /> |


#### InPlacePodResize







_Appears in:_
- [InPlacePodResizeList](#inplacepodresizelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `resize.halje.ru/v1alpha1` | | |
| `kind` _string_ | `InPlacePodResize` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[InPlacePodResizeSpec](#inplacepodresizespec)_ |  |  |  |
| `status` _[InPlacePodResizeStatus](#inplacepodresizestatus)_ |  |  | Optional: \{\} <br /> |


#### InPlacePodResizeList









| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `resize.halje.ru/v1alpha1` | | |
| `kind` _string_ | `InPlacePodResizeList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[InPlacePodResize](#inplacepodresize) array_ |  |  |  |


#### InPlacePodResizeSpec







_Appears in:_
- [InPlacePodResize](#inplacepodresize)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | true | Optional: \{\} <br /> |
| `namespaceSelector` _[NamespaceSelector](#namespaceselector)_ |  |  |  |
| `podSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#labelselector-v1-meta)_ |  |  | Optional: \{\} <br /> |
| `resources` _[ManagedResourcesSpec](#managedresourcesspec)_ |  |  |  |
| `algorithm` _[AlgorithmSpec](#algorithmspec)_ |  |  | Optional: \{\} <br /> |
| `bounds` _[BoundsSpec](#boundsspec)_ |  |  | Optional: \{\} <br /> |
| `thresholds` _[ThresholdsSpec](#thresholdsspec)_ |  |  | Optional: \{\} <br /> |
| `timing` _[TimingSpec](#timingspec)_ |  |  | Optional: \{\} <br /> |


#### InPlacePodResizeStatus







_Appears in:_
- [InPlacePodResize](#inplacepodresize)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ |  |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ |  |  | Optional: \{\} <br /> |
| `lastRunTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#time-v1-meta)_ |  |  | Optional: \{\} <br /> |
| `lastCapabilityCheckTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#time-v1-meta)_ |  |  | Optional: \{\} <br /> |


#### LimitsMode

_Underlying type:_ _string_





_Appears in:_
- [ManagedResourcesSpec](#managedresourcesspec)

| Field | Description |
| --- | --- |
| `Unchanged` |  |
| `EqualRequests` |  |


#### ManagedResourcesSpec







_Appears in:_
- [InPlacePodResizeSpec](#inplacepodresizespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requests` _[ResourceToggles](#resourcetoggles)_ |  |  |  |
| `limitsMode` _[LimitsMode](#limitsmode)_ |  | Unchanged | Enum: [Unchanged EqualRequests] <br />Optional: \{\} <br /> |


#### NamespaceMatchType

_Underlying type:_ _string_





_Appears in:_
- [NamespaceSelector](#namespaceselector)

| Field | Description |
| --- | --- |
| `Glob` |  |
| `Regexp` |  |


#### NamespaceSelector







_Appears in:_
- [InPlacePodResizeSpec](#inplacepodresizespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `matchType` _[NamespaceMatchType](#namespacematchtype)_ |  | Glob | Enum: [Glob Regexp] <br />Optional: \{\} <br /> |
| `include` _string array_ |  |  | Optional: \{\} <br /> |
| `exclude` _string array_ |  |  | Optional: \{\} <br /> |


#### ResourceToggles







_Appears in:_
- [ManagedResourcesSpec](#managedresourcesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cpu` _boolean_ |  |  | Optional: \{\} <br /> |
| `memory` _boolean_ |  |  | Optional: \{\} <br /> |


#### ThresholdsSpec







_Appears in:_
- [InPlacePodResizeSpec](#inplacepodresizespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `upPercent` _integer_ |  | 20 | Maximum: 1000 <br />Minimum: 0 <br />Optional: \{\} <br /> |
| `downPercent` _integer_ |  | 20 | Maximum: 1000 <br />Minimum: 0 <br />Optional: \{\} <br /> |


#### TimingSpec







_Appears in:_
- [InPlacePodResizeSpec](#inplacepodresizespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#duration-v1-meta)_ |  | 1m | Optional: \{\} <br /> |
| `cooldown` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#duration-v1-meta)_ |  | 10m | Optional: \{\} <br /> |
| `stabilizationWindow` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#duration-v1-meta)_ |  | 5m | Optional: \{\} <br /> |
| `maxPodsPerRun` _integer_ |  | 20 | Minimum: 1 <br />Optional: \{\} <br /> |
| `maxConcurrentPods` _integer_ |  | 10 | Minimum: 1 <br />Optional: \{\} <br /> |
| `maxTotalCPUIncrease` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#quantity-resource-api)_ |  |  | Optional: \{\} <br /> |
| `maxTotalMemoryIncrease` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#quantity-resource-api)_ |  |  | Optional: \{\} <br /> |
| `capabilityCheckInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#duration-v1-meta)_ |  | 1h | Optional: \{\} <br /> |
