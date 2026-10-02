// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

type ResourceReconcileTestSuite struct {
	suite.Suite
}

func TestResourceReconcileTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceReconcileTestSuite))
}

func (suite *ResourceReconcileTestSuite) TestReconcileNotFound() {
	r := newFakeThunderIDResourceReconciler()
	_, err := r.Reconcile(context.Background(), reconcileRequest("missing"))
	suite.NoError(err)
}

// Reconcile's parseResourceSpec-error branch (invalid JSON in spec.Raw) turns out to be
// unreachable through any real object: runtime.RawExtension validates that Raw is well-formed
// JSON when it's marshaled, so no client — fake or real API server — can ever persist a ThunderIDResource
// whose spec.Raw fails to parse back out. Confirmed by trying to seed exactly that: the fake
// client rejects it at Create time with "invalid character ... looking for beginning of object
// key string", the same validation a real apiserver's own decode/re-encode round trip would
// enforce before this code ever sees the object. Not worth chasing coverage on dead code.

func (suite *ResourceReconcileTestSuite) TestReconcileDeletionInstanceGoneStillClearsFinalizer() {
	now := metav1.Now()
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{resourceFinalizer},
			Labels:            map[string]string{appsv1alpha1.InstanceRefLabel: "does-not-exist"},
		},
		Spec: rawSpec(`{"resource_type":"role","name":"x"}`),
	}
	r := newFakeThunderIDResourceReconciler(obj)
	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.NoError(err, "owning instance already gone must not block finalizer removal")
}

func (suite *ResourceReconcileTestSuite) TestReconcileUnknownResourceType() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}},
		Spec:       rawSpec(`{"resource_type":"not_a_real_type"}`),
	}
	r := newFakeThunderIDResourceReconciler(instance, obj)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.NoError(err)

	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	suite.Equal("Error", updated.Status.Phase)
}

func (suite *ResourceReconcileTestSuite) TestReconcileInstanceNotFoundRequeues() {
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "does-not-exist"}},
		Spec:       rawSpec(`{"resource_type":"role","name":"x"}`),
	}
	r := newFakeThunderIDResourceReconciler(obj)

	res, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)
	suite.Greater(res.RequeueAfter.Seconds(), 0.0)
}

func (suite *ResourceReconcileTestSuite) TestReconcileAddsFinalizerBeforeSyncing() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}},
		Spec:       rawSpec(`{"resource_type":"role","name":"x"}`),
	}
	r := newFakeThunderIDResourceReconciler(instance, obj)

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)

	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	suite.Contains(updated.Finalizers, resourceFinalizer)
	suite.Empty(updated.Status.Phase, "finalizer-add reconcile returns before touching status")
}

func (suite *ResourceReconcileTestSuite) TestReconcileSyncFailureSetsErrorAndReturnsErr() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			Finalizers: []string{resourceFinalizer},
			Labels:     map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
		Spec: rawSpec(`{"resource_type":"role","name":"x"}`),
	}
	// A sibling ThunderIDResource with unparseable spec — this actually fails rebuildResourcesConfigMap's
	// own r.List call, not the per-item parseResourceSpec inside its loop: the fake client
	// chokes unmarshaling the malformed RawExtension while building the list, before any item
	// is ever iterated. Confirmed via an isolated coverage run showing List's own error branch
	// hit and the in-loop parse-error branch still unreached. Same root cause as the
	// known-unreachable "invalid spec" Reconcile branch, one layer up — malformed RawExtension
	// breaks list-enumeration too, not just per-item decode.
	sibling := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}},
		Spec:       rawSpec(`{not json`),
	}
	r := newFakeThunderIDResourceReconciler(instance, obj, sibling)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Error(err)

	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	suite.Equal("Error", updated.Status.Phase)
}

func (suite *ResourceReconcileTestSuite) TestReconcileFirstSyncSetsPendingAndRequeues() {
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

	res, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)
	suite.InDelta(30, res.RequeueAfter.Seconds(), 1)

	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	suite.Equal("Pending", updated.Status.Phase)
	suite.False(updated.Status.Synced)
	suite.NotNil(updated.Status.LastSyncTime)
	suite.Equal(string(updated.UID), updated.Status.ID)
}

func (suite *ResourceReconcileTestSuite) TestReconcileResourceTypeChangeCleansUpOldConfigMap() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	// spec.resource_type is now "group", but status.syncedResourceType still says "role" - as if
	// this object was previously synced as a role and just got edited.
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			Finalizers: []string{resourceFinalizer},
			Labels:     map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
		Spec:   rawSpec(`{"resource_type":"group","name":"x"}`),
		Status: appsv1alpha1.ThunderIDResourceStatus{SyncedResourceType: "role"},
	}
	r := newFakeThunderIDResourceReconciler(instance, obj)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)

	rolesCM := &corev1.ConfigMap{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst-roles", Namespace: "default"}, rolesCM))
	suite.NotContains(rolesCM.Data["roles.yaml"], "name: x", "the old type's ConfigMap must no longer carry this object's stale entry")

	groupsCM := &corev1.ConfigMap{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst-groups", Namespace: "default"}, groupsCM))
	suite.Contains(groupsCM.Data["groups.yaml"], "name: x", "the new type's ConfigMap must carry the entry")

	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	suite.Equal("group", updated.Status.SyncedResourceType)
}

// rebuildResourcesConfigMap lists every object currently bound to (instance, resourceType), so a
// concurrent reconcile of a sibling resource can write the exact byte-identical ConfigMap -
// this object's own entry included - moments before this reconcile's own rebuild runs, making
// rebuildResourcesConfigMap report changed=false even though this object itself just moved to a
// new (instance, type) pair. Status.SyncedResourceType/SyncedInstance must still advance to the
// new pair on that path - skipping it would leave them pointing at the stale pair purgeIfStale
// already emptied, and a later deletion would clean up that now-empty old location instead of
// the real one, orphaning the object's actual entry in the new ConfigMap forever.
func (suite *ResourceReconcileTestSuite) TestReconcileRecordsSyncedPairEvenWhenConfigMapUnchanged() {
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

	// First reconcile: a genuine first sync, writing the real ConfigMap and recording the pair.
	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)

	// Simulate the race: something else already wrote this exact ConfigMap content (so the next
	// rebuild sees changed=false), but this object's own status still names a stale pair - as if
	// an earlier reconcile hit this exact bug and never advanced it.
	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	updated.Status.SyncedResourceType = "stale-type"
	updated.Status.SyncedInstance = "stale-instance-long-gone"
	updated.Status.Phase = phaseReady
	updated.Status.Synced = true
	suite.Require().NoError(r.Status().Update(context.Background(), updated))

	_, err = r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)

	final := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, final))
	suite.Equal("role", final.Status.SyncedResourceType, "must advance to the real current pair, not stay stuck on the stale one")
	suite.Equal("inst", final.Status.SyncedInstance)
}

func (suite *ResourceReconcileTestSuite) TestReconcileInstanceChangeCleansUpOldInstanceConfigMap() {
	instA := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst-a", Namespace: "default"}}
	instB := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst-b", Namespace: "default"}}
	// The label now points at inst-b, but status.syncedInstance still says inst-a - as if this
	// object was previously synced under inst-a and just got relabeled to a different instance.
	// resource_type is unchanged, isolating the instance-change axis from the type-change one.
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			Finalizers: []string{resourceFinalizer},
			Labels:     map[string]string{appsv1alpha1.InstanceRefLabel: "inst-b"},
		},
		Spec:   rawSpec(`{"resource_type":"role","name":"x"}`),
		Status: appsv1alpha1.ThunderIDResourceStatus{SyncedResourceType: "role", SyncedInstance: "inst-a"},
	}
	r := newFakeThunderIDResourceReconciler(instA, instB, obj)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)

	oldCM := &corev1.ConfigMap{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst-a-roles", Namespace: "default"}, oldCM))
	suite.NotContains(oldCM.Data["roles.yaml"], "name: x", "the old instance's ConfigMap must no longer carry this object's stale entry")

	newCM := &corev1.ConfigMap{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst-b-roles", Namespace: "default"}, newCM))
	suite.Contains(newCM.Data["roles.yaml"], "name: x", "the new instance's ConfigMap must carry the entry")

	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	suite.Equal("inst-b", updated.Status.SyncedInstance)
}

// A resource_type edited from a valid type to an unknown one stops Reconcile before the sync
// path, which is the only other place the entry under the old type gets purged - so the purge
// has to happen on that early-return path too, or the stale entry lives in the old ConfigMap
// until the object is deleted or the typo is fixed.
func (suite *ResourceReconcileTestSuite) TestReconcileUnknownTypePurgesStaleEntryFromOldConfigMap() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	staleCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-roles", Namespace: "default"},
		Data:       map[string]string{"roles.yaml": "---\nname: x\nresource_type: role\n"},
	}
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			Finalizers: []string{resourceFinalizer},
			Labels:     map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
		Spec:   rawSpec(`{"resource_type":"bogus","name":"x"}`),
		Status: appsv1alpha1.ThunderIDResourceStatus{SyncedResourceType: "role", SyncedInstance: "inst"},
	}
	r := newFakeThunderIDResourceReconciler(instance, staleCM, obj)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)

	oldCM := &corev1.ConfigMap{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst-roles", Namespace: "default"}, oldCM))
	suite.NotContains(oldCM.Data["roles.yaml"], "name: x", "the old type's ConfigMap must no longer carry this object's stale entry")

	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	suite.Equal(phaseError, updated.Status.Phase)
	suite.Empty(updated.Status.SyncedResourceType, "the purged location must not be re-purged on every later reconcile")
	suite.Empty(updated.Status.SyncedInstance)
}

// Relabeling to an instance that doesn't exist requeues forever waiting for it; the entry under
// the instance this object was actually synced to has to be purged on that path too.
func (suite *ResourceReconcileTestSuite) TestReconcileMissingInstancePurgesStaleEntryFromOldInstance() {
	instA := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst-a", Namespace: "default"}}
	staleCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-a-roles", Namespace: "default"},
		Data:       map[string]string{"roles.yaml": "---\nname: x\nresource_type: role\n"},
	}
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			Finalizers: []string{resourceFinalizer},
			Labels:     map[string]string{appsv1alpha1.InstanceRefLabel: "inst-gone"},
		},
		Spec:   rawSpec(`{"resource_type":"role","name":"x"}`),
		Status: appsv1alpha1.ThunderIDResourceStatus{SyncedResourceType: "role", SyncedInstance: "inst-a"},
	}
	r := newFakeThunderIDResourceReconciler(instA, staleCM, obj)
	r.Recorder = fakeRecorder()

	res, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)
	suite.Positive(res.RequeueAfter, "must keep waiting for the newly-referenced instance to appear")

	oldCM := &corev1.ConfigMap{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst-a-roles", Namespace: "default"}, oldCM))
	suite.NotContains(oldCM.Data["roles.yaml"], "name: x", "the old instance's ConfigMap must no longer carry this object's stale entry")

	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	suite.Empty(updated.Status.SyncedResourceType)
	suite.Empty(updated.Status.SyncedInstance)
}

// The mirror of the test above: the owning instance merely being deleted is not a move, so the
// recorded location stays accurate and must survive - clearing it would lose the only record of
// where this object's entry lived if the instance comes back.
func (suite *ResourceReconcileTestSuite) TestReconcileMissingInstanceKeepsSyncedFieldsWhenLocationUnchanged() {
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			Finalizers: []string{resourceFinalizer},
			Labels:     map[string]string{appsv1alpha1.InstanceRefLabel: "inst-gone"},
		},
		Spec:   rawSpec(`{"resource_type":"role","name":"x"}`),
		Status: appsv1alpha1.ThunderIDResourceStatus{SyncedResourceType: "role", SyncedInstance: "inst-gone"},
	}
	r := newFakeThunderIDResourceReconciler(obj)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)

	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	suite.Equal("role", updated.Status.SyncedResourceType)
	suite.Equal("inst-gone", updated.Status.SyncedInstance)
}

func (suite *ResourceReconcileTestSuite) TestReconcileDeletionUsesSyncedInstanceNotCurrentLabel() {
	now := metav1.Now()
	instA := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst-a", Namespace: "default"}}
	instB := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst-b", Namespace: "default"}}
	// Relabeled to inst-b (which exists but never actually synced this object) and now being
	// deleted - cleanup must target inst-a (status.syncedInstance), where the real stale entry
	// lives, not inst-b (the current label).
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{resourceFinalizer},
			Labels:            map[string]string{appsv1alpha1.InstanceRefLabel: "inst-b"},
		},
		Spec:   rawSpec(`{"resource_type":"role","name":"x"}`),
		Status: appsv1alpha1.ThunderIDResourceStatus{SyncedResourceType: "role", SyncedInstance: "inst-a"},
	}
	r := newFakeThunderIDResourceReconciler(instA, instB, obj)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err, "deletion must succeed and clean up against the recorded instance")

	updated := &appsv1alpha1.ThunderIDResource{}
	err = r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated)
	suite.True(k8serrors.IsNotFound(err), "object must be fully deleted once its last finalizer clears")
}

func (suite *ResourceReconcileTestSuite) TestReconcileDeletionAfterTypeEditedToUnknownStillClearsFinalizer() {
	now := metav1.Now()
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	// This object was previously synced as a role (status.syncedResourceType), but
	// spec.resource_type has since been edited to something the operator no longer recognizes,
	// and it's now being deleted. The old (pre-fix) behavior used the freshly-parsed, now-invalid
	// type for cleanup, which errored and left the finalizer stuck forever.
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "r", Namespace: "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{resourceFinalizer},
			Labels:            map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
		Spec:   rawSpec(`{"resource_type":"not_a_real_type","name":"x"}`),
		Status: appsv1alpha1.ThunderIDResourceStatus{SyncedResourceType: "role"},
	}
	r := newFakeThunderIDResourceReconciler(instance, obj)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err, "an unknown current resource_type must not block finalizer removal on delete")

	// Clearing the last finalizer on an object that already has a DeletionTimestamp lets the
	// (fake, same as real) apiserver complete the deletion - so a NotFound here is the proof the
	// finalizer was actually removed, not left stuck.
	updated := &appsv1alpha1.ThunderIDResource{}
	err = r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated)
	suite.True(k8serrors.IsNotFound(err), "object must be fully deleted once its last finalizer clears")
}

func (suite *ResourceReconcileTestSuite) TestReconcilePendingWithinWindowSkipsHealthCheck() {
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

	// First reconcile: nothing synced yet, so this sets Phase=Pending.
	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)

	// Second reconcile immediately after: content is unchanged (changed=false) and
	// LastSyncTime is seconds old, well within the 30s window — must requeue without ever
	// calling checkThunderIDHealth (which, given zero matching pods, would flip this straight
	// to Ready and prove the guard didn't hold).
	res, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)
	suite.Greater(res.RequeueAfter.Seconds(), 0.0)

	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	suite.Equal("Pending", updated.Status.Phase, "health check must not have run yet")
}

func (suite *ResourceReconcileTestSuite) TestReconcileUnchangedErrorRunsHealthCheck() {
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

	// First reconcile actually syncs the content (changed=true), landing on Phase=Pending.
	_, err := r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)

	// Flip to Error out-of-band, as a pod crash would via checkThunderIDHealth — without
	// touching spec, so the ConfigMap content is still exactly what's already been synced.
	current := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, current))
	current.Status.Phase = "Error"
	suite.Require().NoError(r.Status().Update(context.Background(), current))

	// Second reconcile: content is unchanged (changed=false), so the Phase=="Error" branch
	// must run checkThunderIDHealth directly — no matching pods means no crash found, so it
	// should flip to Ready.
	_, err = r.Reconcile(context.Background(), reconcileRequest("r"))
	suite.Require().NoError(err)

	updated := &appsv1alpha1.ThunderIDResource{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "r", Namespace: "default"}, updated))
	suite.Equal("Ready", updated.Status.Phase)
	suite.True(updated.Status.Synced)
}

func (suite *ResourceReconcileTestSuite) TestCheckThunderIDHealthNoPodsBecomesReady() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"},
		Status:     appsv1alpha1.ThunderIDResourceStatus{Phase: "Pending"},
	}
	r := newFakeThunderIDResourceReconciler(instance, obj)
	r.Recorder = fakeRecorder()

	_, err := r.checkThunderIDHealth(context.Background(), obj, instance)
	suite.Require().NoError(err)
	suite.Equal("Ready", obj.Status.Phase)
	suite.True(obj.Status.Synced)
}

func (suite *ResourceReconcileTestSuite) TestCheckThunderIDHealthCrashNotLoopingRequeuesWithoutFetchingLogs() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-pod", Namespace: "default", Labels: map[string]string{"app": "inst"}},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name:  "thunderid",
			Ready: false,
			LastTerminationState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
			},
		}}},
	}
	obj := &appsv1alpha1.ThunderIDResource{ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"}}
	r := newFakeThunderIDResourceReconciler(instance, pod, obj)
	r.Recorder = fakeRecorder()
	// KubeClient deliberately left nil: fetchAndEmitPodErrors would panic dereferencing it, so
	// this test only passes if the "not actually CrashLoopBackOff yet" branch really does skip
	// calling it.

	res, err := r.checkThunderIDHealth(context.Background(), obj, instance)
	suite.Require().NoError(err)
	suite.Equal("Error", obj.Status.Phase)
	suite.InDelta(10, res.RequeueAfter.Seconds(), 1)
}

func (suite *ResourceReconcileTestSuite) TestCheckThunderIDHealthCrashLoopBackOffFetchesLogs() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-pod", Namespace: "default", Labels: map[string]string{"app": "inst"}},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name:  "thunderid",
			Ready: false,
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		}}},
	}
	obj := &appsv1alpha1.ThunderIDResource{ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"}}
	r := newFakeThunderIDResourceReconciler(instance, pod, obj)
	fr := record.NewFakeRecorder(20)
	r.Recorder = fr
	// fakePods.GetLogs defaults to returning the literal "fake logs" (client-go's own stub) -
	// no "level=ERROR" marker in it, so this exercises the tail-fallback branch of
	// fetchAndEmitPodErrors, proving checkThunderIDHealth's CrashLoopBackOff path really does
	// call it (see the two focused fetchAndEmitPodErrors tests below for the other branches).
	r.KubeClient = k8sfake.NewSimpleClientset()

	res, err := r.checkThunderIDHealth(context.Background(), obj, instance)
	suite.Require().NoError(err)
	suite.Equal("Error", obj.Status.Phase)
	suite.Equal(float64(0), res.RequeueAfter.Seconds(), "CrashLoopBackOff path returns immediately, logs already handled")

	suite.Require().NotEmpty(fr.Events)
	suite.Contains(<-fr.Events, "fake logs")
}

// capturingRecorder records which object each event was emitted on. record.FakeRecorder keeps
// only the formatted message string, which can't show attribution - the thing the crash-
// diagnosis path has to get right.
type capturingRecorder struct {
	record.EventRecorder
	objects []apimachineryruntime.Object
}

func (c *capturingRecorder) Event(object apimachineryruntime.Object, eventtype, reason, message string) {
	c.objects = append(c.objects, object)
	c.EventRecorder.Event(object, eventtype, reason, message)
}

func (c *capturingRecorder) Eventf(object apimachineryruntime.Object, eventtype, reason, messageFmt string, args ...any) {
	c.objects = append(c.objects, object)
	c.EventRecorder.Eventf(object, eventtype, reason, messageFmt, args...)
}

// The crashed pod is shared by every ThunderIDResource bound to the instance, so nothing in its
// logs identifies which resource's config caused the crash. Attributing them to whichever
// resource happens to be polling stamps one resource's error onto unrelated healthy ones, word
// for word - every bystander re-diagnoses the same pod during the same outage. They belong on
// the instance.
func (suite *ResourceReconcileTestSuite) TestCheckThunderIDHealthEmitsCrashDiagnosisOnInstanceNotResource() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-pod", Namespace: "default", Labels: map[string]string{"app": "inst"}},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name:  "thunderid",
			Ready: false,
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
				Reason:  "CrashLoopBackOff",
				Message: "back-off restarting failed container",
			}},
		}}},
	}
	// A resource that is merely bound to the same instance - it did nothing wrong.
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bystander", Namespace: "default",
			Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
	}
	r := newFakeThunderIDResourceReconciler(instance, pod, obj)
	rec := &capturingRecorder{EventRecorder: record.NewFakeRecorder(20)}
	r.Recorder = rec
	r.KubeClient = k8sfake.NewSimpleClientset()

	_, err := r.checkThunderIDHealth(context.Background(), obj, instance)
	suite.Require().NoError(err)

	suite.Require().NotEmpty(rec.objects, "an unhealthy pod must produce at least one diagnostic event")
	for _, o := range rec.objects {
		inst, ok := o.(*appsv1alpha1.ThunderIDInstance)
		suite.Require().Truef(ok, "crash diagnosis must be emitted on the ThunderIDInstance, got %T", o)
		suite.Equal("inst", inst.Name)
	}
}

// podLogsReactor makes a fake Clientset's Pods().GetLogs(...).Stream() return content (or fail)
// instead of client-go's own "fake logs" stub - GetLogs is implemented as an invokable Action
// with Subresource "log" (see fake_pod_expansion.go), so a plain reactor keyed on verb+resource
// alone would also catch ordinary pod Gets; filtering on GetSubresource() is what scopes it to
// just the log fetch.
var errStreamFailed = errors.New("simulated log stream failure")

func podLogsReactor(content string, err error) clienttesting.ReactionFunc {
	return func(action clienttesting.Action) (bool, apimachineryruntime.Object, error) {
		if action.GetSubresource() != "log" {
			return false, nil, nil
		}
		if err != nil {
			return true, nil, err
		}
		return true, &apimachineryruntime.Unknown{Raw: []byte(content)}, nil
	}
}

func (suite *ResourceReconcileTestSuite) TestFetchAndEmitPodErrorsStreamFailsEmitsFetchFailure() {
	obj := &appsv1alpha1.ThunderIDResource{ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"}}
	r := newFakeThunderIDResourceReconciler(obj)
	fr := record.NewFakeRecorder(20)
	r.Recorder = fr
	cs := k8sfake.NewSimpleClientset()
	cs.PrependReactor("get", "pods", podLogsReactor("", errStreamFailed))
	r.KubeClient = cs

	r.fetchAndEmitPodErrors(context.Background(), obj, "default", "crashing-pod")

	suite.Require().NotEmpty(fr.Events)
	suite.Contains(<-fr.Events, "could not fetch logs")
}

func (suite *ResourceReconcileTestSuite) TestFetchAndEmitPodErrorsParsesErrorLines() {
	obj := &appsv1alpha1.ThunderIDResource{ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "default"}}
	r := newFakeThunderIDResourceReconciler(obj)
	fr := record.NewFakeRecorder(20)
	r.Recorder = fr
	cs := k8sfake.NewSimpleClientset()
	cs.PrependReactor("get", "pods", podLogsReactor(
		"level=INFO msg=starting\n"+
			`{"level":"ERROR","msg":"boom one"}`+"\n"+
			"level=ERROR msg=\"boom two\"\n"+
			"level=INFO msg=done\n", nil))
	r.KubeClient = cs

	r.fetchAndEmitPodErrors(context.Background(), obj, "default", "crashing-pod")

	suite.Require().Len(fr.Events, 2, "only the two ERROR lines should be emitted, not the INFO ones")
	suite.Contains(<-fr.Events, "boom one")
	suite.Contains(<-fr.Events, "boom two")
}
