# thunderid-operator

A Helm chart for the ThunderID Kubernetes operator, which deploys and configures
[ThunderID](https://github.com/thunder-id/thunderid) via CRDs.

This chart is a Helm-packaged equivalent of the project's `config/` kustomize
manifests — same RBAC, same Deployment, same 3 CRDs — so either install path
produces the same cluster state.

## Installing

```bash
# CRDs live in crds/ and are installed automatically by `helm install`.
helm install thunderid-operator ./charts/thunderid-operator \
  --namespace thunderid-system --create-namespace \
  --set image.repository=<your-registry>/thunderid-operator \
  --set image.tag=<your-tag>
```

> Helm's `crds/` directory is installed once on `helm install` and is never
> upgraded or deleted by `helm upgrade`/`helm uninstall`. If a CRD changes,
> apply it manually: `kubectl apply -f charts/thunderid-operator/crds/`.

## Uninstalling

```bash
helm uninstall thunderid-operator --namespace thunderid-system
# CRDs (and any CRs still using them) are left in place deliberately — remove by hand if desired:
kubectl delete -f charts/thunderid-operator/crds/
```

## Values

| Key | Default | Description |
|---|---|---|
| `replicaCount` | `1` | Operator pod replica count. |
| `image.repository` | `controller` | Operator image repository. |
| `image.tag` | `""` | Operator image tag; defaults to `.Chart.AppVersion` if unset. |
| `image.pullPolicy` | `IfNotPresent` | |
| `imagePullSecrets` | `[]` | |
| `serviceAccount.create` | `true` | Create a ServiceAccount for the operator. |
| `serviceAccount.name` | `""` | Override the ServiceAccount name (defaults to the chart fullname). |
| `leaderElection.enabled` | `true` | Enable leader election (`--leader-elect`). Required if `replicaCount > 1`. |
| `healthProbe.port` | `8081` | Liveness/readiness probe port. |
| `metrics.enabled` | `true` | Expose `/metrics` over HTTPS and install the metrics Service + auth RBAC. |
| `metrics.port` | `8443` | |
| `rbac.helperRoles` | `true` | Install the `thunderidinstance-admin/editor/viewer` convenience ClusterRoles (not used by the operator itself). |
| `extraArgs` | `[]` | Extra flags appended to the `manager` container args. |
| `resources` | `limits: 500m/128Mi, requests: 10m/64Mi` | |
| `nodeSelector`, `tolerations`, `affinity` | `{}` / `[]` / `{}` | |
| `podAnnotations`, `podLabels` | `{}` | |
| `podSecurityContext`, `securityContext` | restricted-profile defaults | Matches the kustomize manifest's Pod Security Standards. |
| `terminationGracePeriodSeconds` | `10` | |

## What this chart does not include

Sub-controller RBAC and the 3 CRDs cover `apps.thunderid.io` resources only.
This chart does not install ThunderID itself — that happens when you apply a
`ThunderIDInstance` CR after the operator is running (see
[../../README.md](../../README.md)).
