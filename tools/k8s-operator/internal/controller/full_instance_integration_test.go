//go:build integration

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

// TestFullInstanceOnMinikube reconciles a real SQLite-backed ThunderIDInstance against whatever
// cluster the ambient kubeconfig points at - intended to be a real, already-running minikube, the
// same one you'd otherwise deploy to by hand - and waits for an actual pod to go Ready. This is
// the one thing TestControllers (envtest, above) structurally cannot prove: envtest's control
// plane has no kubelet, so a Deployment object existing there never means a container actually
// ran. Bundled into the same "integration" build tag rather than a separate one, so a plain
// `go test -tags=integration ./...` always requires a real cluster to be up - see the package's
// test-tier notes for the three-way split (unit / envtest / this).
//
// SQLite only, deliberately: Postgres needs an external database provisioned first, which this
// test has no way to set up for itself; SQLite just needs a PVC, which any cluster can provision.
//
// Runs in its own generated namespace, torn down at the end either way, specifically so this can
// never collide with a real deployed instance (e.g. thunderid-prod) sitting in the same cluster.
func TestFullInstanceOnMinikube(t *testing.T) {
	// Only suite_test.go's BeforeSuite calls logf.SetLogger, and Go runs Test funcs in the order
	// their source files sort alphabetically - "full_instance..." sorts before "suite_test.go", so
	// running this test alone (or even bundled - it still runs first) would otherwise reconcile
	// against controller-runtime's default no-op logger, silently discarding every log line this
	// test exists partly to let you watch live while debugging a real instance boot.
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	cfg, err := ctrl.GetConfig()
	require.NoError(t, err, "no reachable cluster via the ambient kubeconfig - this test needs a "+
		"real cluster (e.g. minikube) already running, since it proves a pod actually boots, "+
		"which envtest's fake control plane (no kubelet) structurally cannot do")

	scheme := newFakeReconcilerScheme()
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	require.NoError(t, err)
	kubeClient, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "thunderid-fulltest-"}}
	require.NoError(t, k8sClient.Create(ctx, ns), "failed to create a disposable test namespace")
	t.Cleanup(func() {
		_ = k8sClient.Delete(context.Background(), ns)
	})

	const instanceName = "full-test"
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: ns.Name},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			Image:          "ghcr.io/thunder-id/thunderid:latest",
			DataVolumeSize: "1Gi",
		},
	}
	require.NoError(t, k8sClient.Create(ctx, instance), "failed to create the ThunderIDInstance CR - "+
		"is its CRD installed on this cluster? (config/crd/bases/apps.thunderid.io_thunderidinstances.yaml)")

	r := &ThunderIDInstanceReconciler{
		Client: k8sClient, Scheme: scheme,
		Recorder: record.NewFakeRecorder(50), KubeClient: kubeClient,
	}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: instanceName, Namespace: ns.Name}}

	// A manager's watch loop would call Reconcile() repeatedly as objects settle; busy-polling it
	// directly here is the same idea without needing a live manager. Note this only proves the
	// operator successfully wrote the Deployment/Service/etc - ThunderIDInstance.status.phase is
	// set to "Running" unconditionally once Reconcile returns no error (see Reconcile's tail), it
	// does not itself reflect real pod health. The actual proof is the pod check below.
	require.Eventually(t, func() bool {
		_, err := r.Reconcile(ctx, req)
		if err != nil {
			t.Logf("reconcile error (may resolve on a later pass): %v", err)
			return false
		}
		return true
	}, 30*time.Second, 2*time.Second, "operator never finished reconciling the instance's objects")

	// The real proof: an actual container, actually running, actually passing its readiness probe
	// - not just a Deployment object sitting in etcd. First image pull can be slow, hence the long
	// timeout; re-reconciles keep running throughout so a transient failure (e.g. a slow PVC bind)
	// gets retried the same way a live manager's requeue would.
	require.Eventually(t, func() bool {
		_, _ = r.Reconcile(ctx, req)

		pods := &corev1.PodList{}
		if err := k8sClient.List(ctx, pods, client.InNamespace(ns.Name), client.MatchingLabels{"app": instanceName}); err != nil {
			return false
		}
		for _, pod := range pods.Items {
			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return true
				}
			}
		}
		return false
	}, 5*time.Minute, 3*time.Second, "ThunderID pod never became Ready - check `kubectl get pods -n "+ns.Name+"` "+
		"and `kubectl describe pod -n "+ns.Name+"` for why (image pull, crash loop, etc.)")
}
