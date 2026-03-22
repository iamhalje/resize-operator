# resize-operator

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.1](https://img.shields.io/badge/AppVersion-0.0.1-informational?style=flat-square)

In-place Pod resize operator

**Homepage:** <https://github.com/iamhalje/resize-operator>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Dmitry Ponomaryov |  |  |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchLabels."app.kubernetes.io/name" | string | `"resize-operator"` |  |
| affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey | string | `"kubernetes.io/hostname"` |  |
| affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight | int | `1` |  |
| args.bindAddress | string | `":8081"` |  |
| args.extra[0] | string | `"--zap-log-level=info"` |  |
| args.kubeApiBurst | int | `0` |  |
| args.kubeApiQps | int | `0` |  |
| args.leaderElect | bool | `true` |  |
| args.metricsBindAddress | string | `":8080"` |  |
| args.metricsTimeout | string | `""` |  |
| args.patchTimeout | string | `""` |  |
| args.resizeTimeout | string | `""` |  |
| args.timeZone | string | `""` |  |
| containerSecurityContext.allowPrivilegeEscalation | bool | `false` |  |
| containerSecurityContext.capabilities.drop[0] | string | `"ALL"` |  |
| containerSecurityContext.readOnlyRootFilesystem | bool | `true` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"resize-operator"` |  |
| image.tag | string | `""` |  |
| metrics.service.create | bool | `true` |  |
| metrics.service.port | int | `8080` |  |
| metrics.serviceMonitor.additionalLabels | object | `{}` |  |
| metrics.serviceMonitor.enabled | bool | `false` |  |
| metrics.serviceMonitor.interval | string | `"30s"` |  |
| nodeSelector."node-role.kubernetes.io/control-plane" | string | `""` |  |
| podSecurityContext.runAsNonRoot | bool | `true` |  |
| podSecurityContext.seccompProfile.type | string | `"RuntimeDefault"` |  |
| policy.create | bool | `false` |  |
| policy.name | string | `"EXAMPLE"` |  |
| policy.spec.algorithm.allowDownscale | bool | `false` |  |
| policy.spec.algorithm.headroomPercent | int | `20` |  |
| policy.spec.bounds.cpuMax | int | `4` |  |
| policy.spec.bounds.cpuMin | string | `"50m"` |  |
| policy.spec.bounds.memoryMax | string | `"4Gi"` |  |
| policy.spec.bounds.memoryMin | string | `"128Mi"` |  |
| policy.spec.bounds.minChangeCPU | string | `"50m"` |  |
| policy.spec.bounds.minChangeMemory | string | `"64Mi"` |  |
| policy.spec.enabled | bool | `false` |  |
| policy.spec.namespaceSelector.exclude | list | `[]` |  |
| policy.spec.namespaceSelector.include | list | `[]` |  |
| policy.spec.podSelector.matchLabels."resize.maxim.technology/enabled" | string | `"true"` |  |
| policy.spec.resources.limitsMode | string | `"Unchanged"` |  |
| policy.spec.resources.requests.cpu | bool | `true` |  |
| policy.spec.resources.requests.memory | bool | `true` |  |
| policy.spec.thresholds.downPercent | int | `0` |  |
| policy.spec.thresholds.upPercent | int | `0` |  |
| policy.spec.timing.capabilityCheckInterval | string | `"24h"` |  |
| policy.spec.timing.cooldown | string | `"2m"` |  |
| policy.spec.timing.interval | string | `"1m"` |  |
| policy.spec.timing.maxConcurrentPods | int | `10` |  |
| policy.spec.timing.maxPodsPerRun | int | `50` |  |
| policy.spec.timing.stabilizationWindow | string | `"30s"` |  |
| rbac.create | bool | `true` |  |
| replicaCount | int | `1` |  |
| resources.limits.cpu | string | `"1000m"` |  |
| resources.limits.memory | string | `"256Mi"` |  |
| resources.requests.cpu | string | `"100m"` |  |
| resources.requests.memory | string | `"128Mi"` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` |  |
| tolerations[0].effect | string | `"NoSchedule"` |  |
| tolerations[0].key | string | `"node-role.kubernetes.io/control-plane"` |  |
| tolerations[0].operator | string | `"Exists"` |  |

