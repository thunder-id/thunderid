# ThunderID Operator Helm Chart

A Helm chart for the ThunderID Kubernetes operator, which deploys and configures
[ThunderID](https://github.com/thunder-id/thunderid) via CRDs.

This chart is a Helm-packaged equivalent of the project's `config/` kustomize
manifests (same RBAC, same Deployment, same 3 CRDs), so either install path
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

If you're removing the CRs or CRDs as well, do that first, while the operator is still running (see below).
Each `ThunderIDResource` has a `thunderid.io/resource-cleanup` finalizer that only the operator removes; once
the operator is gone, deleting one (or its namespace, or the CRDs) leaves it stuck in `Terminating`. To unstick
one, clear the finalizer by hand:
`kubectl patch thunderidresource <name> -n <namespace> --type merge -p '{"metadata":{"finalizers":null}}'`.

```bash
helm uninstall thunderid-operator --namespace thunderid-system
```

This removes the operator's own `Deployment`/RBAC only. CRDs, and every `ThunderIDInstance`/`ThunderIDResource`/
`EnvironmentValues` still using them, are left in place deliberately; the operator just stops reconciling them.

To remove the CRDs too:

```bash
kubectl delete -f charts/thunderid-operator/crds/
```

CRDs are cluster-scoped, so this deletes every CR of that kind across the whole cluster, not just this
namespace. Each CR's `Deployment`, `Service`, `Ingress`, `HPA`, ConfigMaps, security Secret, and SQLite data PVC
carry an owner reference back to it, so Kubernetes' garbage collector cascades the deletion to all of them,
including the PVC, taking its data with it. Only do this once you're sure nothing still needs those instances.

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
| `watchNamespaces` | `[]` | Namespaces to watch. Empty watches every namespace through a ClusterRole; when set, the operator gets a Role in each listed namespace instead and ignores every other namespace. Each namespace must already exist. |
| `rbac.helperRoles` | `true` | Install the `thunderidinstance-admin/editor/viewer` convenience ClusterRoles (not used by the operator itself). |
| `extraArgs` | `[]` | Extra flags appended to the `manager` container args. |
| `resources` | `limits: 500m/128Mi, requests: 10m/64Mi` | |
| `nodeSelector`, `tolerations`, `affinity` | `{}` / `[]` / `{}` | |
| `podAnnotations`, `podLabels` | `{}` | |
| `podSecurityContext`, `securityContext` | restricted-profile defaults | Matches the kustomize manifest's Pod Security Standards. |
| `terminationGracePeriodSeconds` | `10` | |

## What this chart does not include

Sub-controller RBAC and the 3 CRDs cover `apps.thunderid.io` resources only.
This chart does not install ThunderID itself; that happens when you apply a
`ThunderIDInstance` CR after the operator is running (see
[../../README.md](../../README.md)).
