// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

// Remaining "a sub-step failed" and untested-logic branches in ThunderIDResourceReconciler.Reconcile /
// checkThunderIDHealth / rebuildResourcesConfigMap / the two Watch-mapping functions — same
// interceptor approach as reconcile_failure_test.go, applied to the second reconciler.

type ResourceReconcileFailureTestSuite struct {
	suite.Suite
}

func TestResourceReconcileFailureTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceReconcileFailureTestSuite))
}

func (suite *ResourceReconcileFailureTestSuite) TestReconcileDeletionInstanceGetErrorPropagates() {
	now := metav1.Now()
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{resourceFinalizer},
			Labels:            map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
	}
	wantErr := errors.New("simulated instance get failure")
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*appsv1alpha1.ThunderIDInstance); ok {
				return wantErr
			}
			return c.Get(ctx, key, o, opts...)
		},
	}, obj)

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *ResourceReconcileFailureTestSuite) TestReconcileDeletionRebuildConfigMapErrorPropagates() {
	now := metav1.Now()
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{resourceFinalizer},
			Labels:            map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
		Spec: rawSpec(`{"resource_type":"role","name":"x"}`),
	}
	wantErr := errors.New("simulated list failure")
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, l client.ObjectList, opts ...client.ListOption) error {
			if _, ok := l.(*appsv1alpha1.ThunderIDResourceList); ok {
				return wantErr
			}
			return c.List(ctx, l, opts...)
		},
	}, instance, obj)

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *ResourceReconcileFailureTestSuite) TestReconcileDeletionFinalizerUpdateErrorPropagates() {
	now := metav1.Now()
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{resourceFinalizer},
			Labels:            map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
		Spec: rawSpec(`{"resource_type":"role","name":"x"}`),
	}
	wantErr := errors.New("simulated finalizer-removal update failure")
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.UpdateOption) error {
			if _, ok := o.(*appsv1alpha1.ThunderIDResource); ok {
				return wantErr
			}
			return c.Update(ctx, o, opts...)
		},
	}, instance, obj)

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *ResourceReconcileFailureTestSuite) TestReconcileInstanceGetErrorPropagates() {
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}},
		Spec:       rawSpec(`{"resource_type":"role","name":"x"}`),
	}
	wantErr := errors.New("simulated instance get failure")
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*appsv1alpha1.ThunderIDInstance); ok {
				return wantErr
			}
			return c.Get(ctx, key, o, opts...)
		},
	}, obj)

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *ResourceReconcileFailureTestSuite) TestReconcileAddFinalizerUpdateErrorPropagates() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}},
		Spec:       rawSpec(`{"resource_type":"role","name":"x"}`),
	}
	wantErr := errors.New("simulated finalizer-add update failure")
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.UpdateOption) error {
			if _, ok := o.(*appsv1alpha1.ThunderIDResource); ok {
				return wantErr
			}
			return c.Update(ctx, o, opts...)
		},
	}, instance, obj)

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *ResourceReconcileFailureTestSuite) TestReconcilePendingPastWindowRunsHealthCheck() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			Finalizers: []string{resourceFinalizer},
			Labels:     map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
		Spec: rawSpec(`{"resource_type":"role","name":"x"}`),
	}
	r := newFakeThunderIDResourceReconciler(instance, obj)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)

	// Backdate LastSyncTime past the 30s window without touching spec (content stays
	// unchanged=false either way) - Pending + elapsed>=30s must fall through to
	// checkThunderIDHealth instead of requeuing again.
	current := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, current))
	old := metav1.NewTime(time.Now().Add(-31 * time.Second))
	current.Status.LastSyncTime = &old
	suite.Require().NoError(r.Status().Update(context.Background(), current))

	_, err = r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)

	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	suite.Equal("Ready", updated.Status.Phase, "no matching pods means checkThunderIDHealth reports healthy")
}

func (suite *ResourceReconcileFailureTestSuite) TestReconcileUnchangedReadyIsNoop() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			Finalizers: []string{resourceFinalizer},
			Labels:     map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
		Spec: rawSpec(`{"resource_type":"role","name":"x"}`),
	}
	r := newFakeThunderIDResourceReconciler(instance, obj)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)

	current := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, current))
	current.Status.Phase = "Ready"
	current.Status.Synced = true
	suite.Require().NoError(r.Status().Update(context.Background(), current))

	_, err = r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)

	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	suite.Equal("Ready", updated.Status.Phase)
	suite.True(updated.Status.Synced)
}

func (suite *ResourceReconcileFailureTestSuite) TestReconcileFirstSyncStatusUpdateErrorPropagates() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			Finalizers: []string{resourceFinalizer},
			Labels:     map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
		Spec: rawSpec(`{"resource_type":"role","name":"x"}`),
	}
	wantErr := errors.New("simulated status update failure")
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, o client.Object, opts ...client.SubResourceUpdateOption) error {
			if subResourceName == "status" {
				return wantErr
			}
			return c.Status().Update(ctx, o, opts...)
		},
	}, instance, obj)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *ResourceReconcileFailureTestSuite) TestCheckThunderIDHealthListErrorPropagates() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	obj := &appsv1alpha1.ThunderIDResource{ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"}}
	wantErr := errors.New("simulated pod list failure")
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, l client.ObjectList, opts ...client.ListOption) error {
			if _, ok := l.(*corev1.PodList); ok {
				return wantErr
			}
			return c.List(ctx, l, opts...)
		},
	}, instance, obj)

	_, err := r.checkThunderIDHealth(context.Background(), obj, instance)
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *ResourceReconcileFailureTestSuite) TestCheckThunderIDHealthBecomingReadyStatusUpdateErrorPropagates() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	obj := &appsv1alpha1.ThunderIDResource{ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"}}
	wantErr := errors.New("simulated status update failure")
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, o client.Object, opts ...client.SubResourceUpdateOption) error {
			if subResourceName == "status" {
				return wantErr
			}
			return c.Status().Update(ctx, o, opts...)
		},
	}, instance, obj)
	r.Recorder = fakeRecorder()

	_, err := r.checkThunderIDHealth(context.Background(), obj, instance)
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *ResourceReconcileFailureTestSuite) TestRebuildResourcesConfigMapCreateErrorPropagates() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	res := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}},
		Spec:       rawSpec(`{"resource_type":"resource_server","name":"x"}`),
	}
	wantErr := errors.New("simulated ConfigMap create failure")
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.CreateOption) error {
			if _, ok := o.(*corev1.ConfigMap); ok {
				return wantErr
			}
			return c.Create(ctx, o, opts...)
		},
	}, instance, res)

	_, err := r.rebuildResourcesConfigMap(context.Background(), instance, "resource_server", "")
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *ResourceReconcileFailureTestSuite) TestRebuildResourcesConfigMapGetErrorPropagates() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	res := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}},
		Spec:       rawSpec(`{"resource_type":"organization_unit","name":"x"}`),
	}
	wantErr := errors.New("simulated ConfigMap get failure")
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*corev1.ConfigMap); ok {
				return wantErr
			}
			return c.Get(ctx, key, o, opts...)
		},
	}, instance, res)

	_, err := r.rebuildResourcesConfigMap(context.Background(), instance, "organization_unit", "")
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *ResourceReconcileFailureTestSuite) TestRebuildResourcesConfigMapUpdateErrorPropagates() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	res := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}},
		Spec:       rawSpec(`{"resource_type":"translation","name":"x"}`),
	}
	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-translations", Namespace: "default"},
		Data:       map[string]string{"translations.yaml": "stale content that will differ from the freshly rendered version"},
	}
	wantErr := errors.New("simulated ConfigMap update failure")
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.UpdateOption) error {
			if _, ok := o.(*corev1.ConfigMap); ok {
				return wantErr
			}
			return c.Update(ctx, o, opts...)
		},
	}, instance, res, controlledBy(instance, existingCM))

	_, err := r.rebuildResourcesConfigMap(context.Background(), instance, "translation", "")
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *ResourceReconcileFailureTestSuite) TestResourcesForDeploymentIgnoresNonDeploymentObjects() {
	r := newFakeThunderIDResourceReconciler()
	reqs := r.resourcesForDeployment(context.Background(), &corev1.Secret{})
	suite.Nil(reqs)
}

func (suite *ResourceReconcileFailureTestSuite) TestResourcesForDeploymentListErrorReturnsNil() {
	wantErr := errors.New("simulated list failure")
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, l client.ObjectList, opts ...client.ListOption) error {
			if _, ok := l.(*appsv1alpha1.ThunderIDResourceList); ok {
				return wantErr
			}
			return c.List(ctx, l, opts...)
		},
	})
	reqs := r.resourcesForDeployment(context.Background(), &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}})
	suite.Nil(reqs, "a List failure is swallowed - nothing to map to, not a fatal error")
}

func (suite *ResourceReconcileFailureTestSuite) TestResourcesForInstanceListErrorReturnsNil() {
	wantErr := errors.New("simulated list failure")
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, l client.ObjectList, opts ...client.ListOption) error {
			if _, ok := l.(*appsv1alpha1.ThunderIDResourceList); ok {
				return wantErr
			}
			return c.List(ctx, l, opts...)
		},
	})
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	reqs := r.resourcesForInstance(context.Background(), instance)
	suite.Nil(reqs)
}

func (suite *ResourceReconcileFailureTestSuite) TestRebuildResourcesConfigMapRetryGetErrorPropagates() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	res := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}},
		Spec:       rawSpec(`{"resource_type":"server_config","name":"x"}`),
	}
	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-serverconfigs", Namespace: "default"},
		Data:       map[string]string{"serverconfigs.yaml": "stale content"},
	}
	wantErr := errors.New("simulated retry-get failure")
	// The first Get of *ConfigMap ("does it exist yet") must succeed and find existingCM, so
	// the retry.RetryOnConflict closure actually runs - its own Get is the second call, and
	// that's the one this counts up to and fails.
	getCount := 0
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*corev1.ConfigMap); ok {
				getCount++
				if getCount == 2 {
					return wantErr
				}
			}
			return c.Get(ctx, key, o, opts...)
		},
	}, instance, res, controlledBy(instance, existingCM))

	_, err := r.rebuildResourcesConfigMap(context.Background(), instance, "server_config", "")
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

// On a move (role -> group), the new location must be recorded in status before the new
// ConfigMap is written: if it were recorded after and that write failed, a second move before a
// successful retry would purge the old (already empty) location and orphan the entry here.
func (suite *ResourceReconcileFailureTestSuite) TestReconcileRecordsNewLocationBeforeRebuild() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			Finalizers: []string{resourceFinalizer},
			Labels:     map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
		Spec:   rawSpec(`{"resource_type":"group","name":"x"}`),
		Status: appsv1alpha1.ThunderIDResourceStatus{SyncedResourceType: "role", SyncedInstance: "inst"},
	}
	wantErr := errors.New("simulated ConfigMap create failure")
	r := newFakeThunderIDResourceReconcilerWithInterceptor(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.CreateOption) error {
			if cm, ok := o.(*corev1.ConfigMap); ok && cm.Name == "inst-groups" {
				return wantErr
			}
			return c.Create(ctx, o, opts...)
		},
	}, instance, obj)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().ErrorIs(err, wantErr)

	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	suite.Equal("group", updated.Status.SyncedResourceType, "status must already name the new location when its write fails")
	suite.Equal("inst", updated.Status.SyncedInstance)
}

func (suite *ResourceReconcileFailureTestSuite) TestRebuildResourcesConfigMapNilDataInitialized() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	res := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}},
		Spec:       rawSpec(`{"resource_type":"theme","name":"x"}`),
	}
	// Data left nil (not just empty) - the update path must initialize it rather than panic on
	// a nil-map write.
	existingCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "inst-themes", Namespace: "default"}}
	r := newFakeThunderIDResourceReconciler(instance, res, controlledBy(instance, existingCM))

	changed, err := r.rebuildResourcesConfigMap(context.Background(), instance, "theme", "")
	suite.Require().NoError(err)
	suite.True(changed)

	cm := &corev1.ConfigMap{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst-themes", Namespace: "default"}, cm))
	suite.Contains(cm.Data["themes.yaml"], "name: x")
}

func (suite *ResourceReconcileFailureTestSuite) TestCheckThunderIDHealthIgnoresOtherContainers() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-pod", Namespace: "default", Labels: map[string]string{"app": "inst"}},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
			{
				// A sidecar reported as crashed must not be mistaken for the "thunderid"
				// container itself.
				Name:  "some-sidecar",
				Ready: false,
				LastTerminationState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
				},
			},
			{Name: "thunderid", Ready: true},
		}},
	}
	obj := &appsv1alpha1.ThunderIDResource{ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"}}
	r := newFakeThunderIDResourceReconciler(instance, pod, obj)
	r.Recorder = fakeRecorder()

	_, err := r.checkThunderIDHealth(context.Background(), obj, instance)
	suite.Require().NoError(err)
	suite.Equal("Ready", obj.Status.Phase, "the thunderid container itself is healthy regardless of the sidecar")
}

func (suite *ResourceReconcileFailureTestSuite) TestFetchAndEmitPodErrorsCapsAtFiveErrorLines() {
	obj := &appsv1alpha1.ThunderIDResource{ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"}}
	r := newFakeThunderIDResourceReconciler(obj)
	fr := record.NewFakeRecorder(20)
	r.Recorder = fr
	cs := k8sfake.NewSimpleClientset()
	var contentBuilder strings.Builder
	for range 7 {
		contentBuilder.WriteString("level=ERROR msg=\"boom\"\n")
	}
	content := contentBuilder.String()
	cs.PrependReactor("get", "pods", podLogsReactor(content, nil))
	r.KubeClient = cs

	r.fetchAndEmitPodErrors(context.Background(), obj, "default", "crashing-pod")

	suite.Len(fr.Events, 5, "must stop emitting once errCount reaches 5, even though 7 ERROR lines exist")
}
