// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

type EnvironmentValuesControllerTestSuite struct {
	suite.Suite
}

func TestEnvironmentControllerTestSuite(t *testing.T) {
	suite.Run(t, new(EnvironmentValuesControllerTestSuite))
}

func (suite *EnvironmentValuesControllerTestSuite) TestIsMasked() {
	suite.True(isMasked(maskValue("hunter2")))
	suite.False(isMasked("hunter2"))
	// Right prefix, but the trailing star run is 9 long instead of the fixed 10 - must not
	// pass as an accidental near-miss.
	suite.False(isMasked("thunderid-masked:abc*********"))
}

// Neither the revealed prefix nor the trailing star run may vary with the real value's length:
// a fixed reveal count (3) combined with a *known* length would narrow a short value down to
// very few possibilities (a 4-character value would have only one truly unknown character left).
// Keeping the star run a constant 10 characters, and never stating the real length anywhere,
// means every masked output looks identical in shape regardless of how long the real secret is.
func (suite *EnvironmentValuesControllerTestSuite) TestMaskValueRevealsAtMostThreeCharsAndNeverTheLength() {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"long value", "supersecret", "thunderid-masked:sup**********"},
		{"exactly 4 chars", "abcd", "thunderid-masked:abc**********"},
		{"3 chars", "abc", "thunderid-masked:ab**********"},
		{"empty", "", "thunderid-masked:**********"},
	}
	for _, c := range cases {
		suite.Run(c.name, func() {
			got := maskValue(c.in)
			suite.Equal(c.want, got)
			suite.True(isMasked(got), "maskValue's own output must satisfy isMasked")
			// The full plaintext must never appear verbatim once masked (for anything longer
			// than the revealed prefix).
			if len(c.in) > 3 {
				suite.NotContains(got, c.in)
			}
		})
	}
	// Same shape (same total length) regardless of how long the real value was.
	suite.Len(maskValue("short"), len(maskValue(strings.Repeat("x", 500))))
}

func (suite *EnvironmentValuesControllerTestSuite) TestManagedKeySet() {
	withAnnotation := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{managedKeysAnnotation: "A,B,C"},
	}}
	suite.Equal(map[string]bool{"A": true, "B": true, "C": true}, managedKeySet(withAnnotation))

	noAnnotation := &corev1.Secret{}
	suite.Empty(managedKeySet(noAnnotation))
}

func (suite *EnvironmentValuesControllerTestSuite) TestSyncSecretAndMaskNewPlaintextCreatesAndMasks() {
	obj := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{Name: "env", Namespace: "default"},
		Spec:       appsv1alpha1.EnvironmentValuesSpec{Env: map[string]string{"PASSWORD": "hunter2"}},
	}
	r := newFakeEnvironmentReconciler(obj)

	keys, missingKeys, maskedAny, err := r.syncSecretAndMask(context.Background(), obj)
	suite.Require().NoError(err)
	suite.True(maskedAny)
	suite.Equal([]string{"PASSWORD"}, keys)
	suite.Empty(missingKeys)

	secret := &corev1.Secret{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "env", Namespace: "default"}, secret))
	suite.Equal("hunter2", string(secret.Data["PASSWORD"]))

	updated := &appsv1alpha1.EnvironmentValues{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "env", Namespace: "default"}, updated))
	suite.True(isMasked(updated.Spec.Env["PASSWORD"]), "plaintext must be masked in spec.env after sync")
}

func (suite *EnvironmentValuesControllerTestSuite) TestSyncSecretAndMaskAlreadyMaskedIsNoop() {
	masked := maskValue("hunter2")
	obj := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{Name: "env", Namespace: "default"},
		Spec:       appsv1alpha1.EnvironmentValuesSpec{Env: map[string]string{"PASSWORD": masked}},
	}
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "env", Namespace: "default",
			Annotations: map[string]string{managedKeysAnnotation: "PASSWORD"},
		},
		Data: map[string][]byte{"PASSWORD": []byte("hunter2")},
	}
	r := newFakeEnvironmentReconciler(obj, existingSecret)

	keys, missingKeys, maskedAny, err := r.syncSecretAndMask(context.Background(), obj)
	suite.Require().NoError(err)
	suite.False(maskedAny, "an already-masked key must not be treated as new plaintext")
	suite.Equal([]string{"PASSWORD"}, keys)
	suite.Empty(missingKeys)

	// spec.env's masked marker is untouched (still the very same string).
	suite.Equal(masked, obj.Spec.Env["PASSWORD"])
}

// A masked spec.env value is only a marker - the real value lives in the Secret. If the Secret
// lost that key (deleted out-of-band), the value is unrecoverable, so the key must be reported
// as missing rather than listed in status.keys as though it were still backed by a real value.
func (suite *EnvironmentValuesControllerTestSuite) TestSyncSecretAndMaskReportsMaskedKeyMissingFromSecret() {
	obj := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{Name: "env", Namespace: "default"},
		Spec: appsv1alpha1.EnvironmentValuesSpec{Env: map[string]string{
			"PRESENT": maskValue("hunter2"),
			"GONE":    maskValue("deleted-out-of-band"),
		}},
	}
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "env", Namespace: "default",
			Annotations: map[string]string{managedKeysAnnotation: "PRESENT,GONE"},
		},
		Data: map[string][]byte{"PRESENT": []byte("hunter2")},
	}
	r := newFakeEnvironmentReconciler(obj, existingSecret)

	keys, missingKeys, maskedAny, err := r.syncSecretAndMask(context.Background(), obj)
	suite.Require().NoError(err)
	suite.False(maskedAny)
	suite.Equal([]string{"PRESENT"}, keys, "a key with no value in the Secret must not be reported as synced")
	suite.Equal([]string{"GONE"}, missingKeys)
}

func (suite *EnvironmentValuesControllerTestSuite) TestReconcileMaskedKeyMissingFromSecretIsNotSynced() {
	obj := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{Name: "env", Namespace: "default"},
		Spec:       appsv1alpha1.EnvironmentValuesSpec{Env: map[string]string{"GONE": maskValue("deleted-out-of-band")}},
	}
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "env", Namespace: "default",
			Annotations: map[string]string{managedKeysAnnotation: "GONE"},
		},
	}
	r := newFakeEnvironmentReconciler(obj, existingSecret)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("env"))
	suite.Require().NoError(err, "a missing value is a reportable state, not an infra failure to retry")

	updated := &appsv1alpha1.EnvironmentValues{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "env", Namespace: "default"}, updated))
	suite.Equal("Error", updated.Status.Phase)
	suite.False(updated.Status.Synced)
	suite.Equal("env", updated.Status.SecretName)
	suite.Empty(updated.Status.Keys)
}

func (suite *EnvironmentValuesControllerTestSuite) TestSyncSecretAndMaskDropsPreviouslyManagedKey() {
	obj := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{Name: "env", Namespace: "default"},
		Spec:       appsv1alpha1.EnvironmentValuesSpec{Env: map[string]string{"KEEP": "new-value"}},
	}
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "env", Namespace: "default",
			Annotations: map[string]string{managedKeysAnnotation: "KEEP,DROPPED"},
		},
		Data: map[string][]byte{"KEEP": []byte("old-value"), "DROPPED": []byte("gone-now")},
	}
	r := newFakeEnvironmentReconciler(obj, existingSecret)

	_, _, _, err := r.syncSecretAndMask(context.Background(), obj)
	suite.Require().NoError(err)

	secret := &corev1.Secret{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "env", Namespace: "default"}, secret))
	suite.Equal("new-value", string(secret.Data["KEEP"]))
	suite.NotContains(secret.Data, "DROPPED")
}

func (suite *EnvironmentValuesControllerTestSuite) TestSyncSecretAndMaskLeavesForeignKeysAlone() {
	obj := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-name", Namespace: "default"},
		Spec:       appsv1alpha1.EnvironmentValuesSpec{Env: map[string]string{"MINE": "value"}},
	}
	// A hand-made Secret sharing this EnvironmentValues's name, never written by this controller
	// (no managedKeysAnnotation) but explicitly opted in via EnvironmentValuesAdoptLabel - its
	// keys must survive the sync untouched.
	handMade := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "shared-name", Namespace: "default",
			Labels: map[string]string{appsv1alpha1.EnvironmentValuesAdoptLabel: "shared-name"},
		},
		Data: map[string][]byte{"FOREIGN": []byte("do-not-touch")},
	}
	r := newFakeEnvironmentReconciler(obj, handMade)
	r.Recorder = fakeRecorder() // adopting an unmanaged Secret emits an AdoptedExistingSecret event

	_, _, _, err := r.syncSecretAndMask(context.Background(), obj)
	suite.Require().NoError(err)

	secret := &corev1.Secret{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "shared-name", Namespace: "default"}, secret))
	suite.Equal("do-not-touch", string(secret.Data["FOREIGN"]))
	suite.Equal("value", string(secret.Data["MINE"]))
}

func (suite *EnvironmentValuesControllerTestSuite) TestSyncSecretAndMaskRefusesUnlabeledForeignSecret() {
	obj := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-name", Namespace: "default"},
		Spec:       appsv1alpha1.EnvironmentValuesSpec{Env: map[string]string{"MINE": "value"}},
	}
	// A pre-existing Secret sharing this EnvironmentValues's name, never written by this
	// controller and never opted in via EnvironmentValuesAdoptLabel - e.g. an unrelated Secret
	// someone else owns that just happens to collide on name. Must be refused, not merged into.
	foreign := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-name", Namespace: "default"},
		Data:       map[string][]byte{"UNRELATED": []byte("someone-elses-secret")},
	}
	r := newFakeEnvironmentReconciler(obj, foreign)

	_, _, _, err := r.syncSecretAndMask(context.Background(), obj)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not labeled for adoption")

	secret := &corev1.Secret{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "shared-name", Namespace: "default"}, secret))
	suite.Equal("someone-elses-secret", string(secret.Data["UNRELATED"]))
	suite.NotContains(secret.Data, "MINE")
}

func (suite *EnvironmentValuesControllerTestSuite) TestReconcileSetsSyncedStatus() {
	obj := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{Name: "env", Namespace: "default"},
		Spec:       appsv1alpha1.EnvironmentValuesSpec{Env: map[string]string{"A": "b"}},
	}
	r := newFakeEnvironmentReconciler(obj)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("env"))
	suite.Require().NoError(err)

	updated := &appsv1alpha1.EnvironmentValues{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "env", Namespace: "default"}, updated))
	suite.Equal("Synced", updated.Status.Phase)
	suite.True(updated.Status.Synced)
	suite.Equal("env", updated.Status.SecretName)
	suite.Equal([]string{"A"}, updated.Status.Keys)
}

// TestReconcileSyncFailureSetsErrorStatus exercises Reconcile's error branch, which only fires
// when the underlying Secret write genuinely fails — not something the plain fake client will
// ever do on its own for a well-formed request. WithInterceptorFuncs (controller-runtime's
// pkg/client/interceptor) wraps the fake client so a specific call can be made to fail on
// command, the same pattern used across the ecosystem for exactly this: forcing an
// infra-failure branch without needing a real API server to actually break.
func (suite *EnvironmentValuesControllerTestSuite) TestReconcileSyncFailureSetsErrorStatus() {
	obj := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{Name: "env", Namespace: "default"},
		Spec:       appsv1alpha1.EnvironmentValuesSpec{Env: map[string]string{"A": "b"}},
	}

	scheme := runtime.NewScheme()
	suite.Require().NoError(corev1.AddToScheme(scheme))
	suite.Require().NoError(appsv1alpha1.AddToScheme(scheme))

	wantErr := errors.New("simulated Secret create failure")
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&appsv1alpha1.EnvironmentValues{}).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.CreateOption) error {
				if _, ok := o.(*corev1.Secret); ok {
					return wantErr
				}
				return c.Create(ctx, o, opts...)
			},
		}).
		WithRuntimeObjects(obj).
		Build()

	r := &EnvironmentValuesReconciler{Client: fakeClient, Scheme: scheme, Recorder: fakeRecorder()}

	_, err := r.Reconcile(context.Background(), reconcileRequest("env"))
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)

	updated := &appsv1alpha1.EnvironmentValues{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "env", Namespace: "default"}, updated))
	suite.Equal("Error", updated.Status.Phase)
	suite.False(updated.Status.Synced)
	suite.Equal("env", updated.Status.SecretName)
}

func (suite *EnvironmentValuesControllerTestSuite) TestReconcileGetErrorPropagates() {
	wantErr := errors.New("simulated get failure")
	r := newFakeEnvironmentReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key types.NamespacedName, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*appsv1alpha1.EnvironmentValues); ok {
				return wantErr
			}
			return c.Get(ctx, key, o, opts...)
		},
	})

	_, err := r.Reconcile(context.Background(), reconcileRequest("env"))
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *EnvironmentValuesControllerTestSuite) TestReconcileSuccessStatusUpdateErrorPropagates() {
	obj := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{Name: "env", Namespace: "default"},
		Spec:       appsv1alpha1.EnvironmentValuesSpec{Env: map[string]string{"A": "b"}},
	}
	wantErr := errors.New("simulated status update failure")
	r := newFakeEnvironmentReconcilerWithInterceptor(interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, o client.Object, opts ...client.SubResourceUpdateOption) error {
			if subResourceName == "status" {
				return wantErr
			}
			return c.Status().Update(ctx, o, opts...)
		},
	}, obj)
	r.Recorder = fakeRecorder()

	_, err := r.Reconcile(context.Background(), reconcileRequest("env"))
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *EnvironmentValuesControllerTestSuite) TestSyncSecretAndMaskGetErrorPropagates() {
	obj := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{Name: "env", Namespace: "default"},
		Spec:       appsv1alpha1.EnvironmentValuesSpec{Env: map[string]string{"A": "b"}},
	}
	wantErr := errors.New("simulated secret get failure")
	r := newFakeEnvironmentReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key types.NamespacedName, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*corev1.Secret); ok {
				return wantErr
			}
			return c.Get(ctx, key, o, opts...)
		},
	}, obj)

	_, _, _, err := r.syncSecretAndMask(context.Background(), obj)
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *EnvironmentValuesControllerTestSuite) TestSyncSecretAndMaskMaskSpecErrorPropagates() {
	obj := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{Name: "env", Namespace: "default"},
		Spec:       appsv1alpha1.EnvironmentValuesSpec{Env: map[string]string{"A": "new-plaintext"}},
	}
	wantErr := errors.New("simulated environment update failure")
	r := newFakeEnvironmentReconcilerWithInterceptor(interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.UpdateOption) error {
			if _, ok := o.(*appsv1alpha1.EnvironmentValues); ok {
				return wantErr
			}
			return c.Update(ctx, o, opts...)
		},
	}, obj)

	// maskedAny=true (new plaintext) drives syncSecretAndMask into calling maskSpec, whose own
	// Update is what's intercepted here - this covers both syncSecretAndMask's error-propagation
	// wrapper and maskSpec's own "if err != nil" branch around the same call in one shot.
	_, _, _, err := r.syncSecretAndMask(context.Background(), obj)
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *EnvironmentValuesControllerTestSuite) TestUpsertSecretRetryGetErrorPropagates() {
	obj := &appsv1alpha1.EnvironmentValues{ObjectMeta: metav1.ObjectMeta{Name: "env", Namespace: "default"}}
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "env", Namespace: "default",
			Annotations: map[string]string{managedKeysAnnotation: "A"},
		},
		Data: map[string][]byte{"A": []byte("old")},
	}
	wantErr := errors.New("simulated retry-get failure")
	r := newFakeEnvironmentReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key types.NamespacedName, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*corev1.Secret); ok {
				return wantErr
			}
			return c.Get(ctx, key, o, opts...)
		},
	}, obj, existing)

	// Called directly (not via syncSecretAndMask) so this interceptor's Get override only ever
	// sees upsertSecret's own retry-loop Get, not some earlier unrelated one.
	err := r.upsertSecret(context.Background(), obj, true, existing, map[string][]byte{"A": []byte("new")})
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *EnvironmentValuesControllerTestSuite) TestUpsertSecretRecomputesFromLatestOnRetry() {
	obj := &appsv1alpha1.EnvironmentValues{ObjectMeta: metav1.ObjectMeta{Name: "env", Namespace: "default"}}
	// What upsertSecret is called with here is stale relative to the real stored Secret below -
	// as if another actor added a foreign key between this snapshot being read and this call.
	staleExisting := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "env", Namespace: "default",
			Annotations: map[string]string{managedKeysAnnotation: "A"},
		},
		Data: map[string][]byte{"A": []byte("old")},
	}
	realSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "env", Namespace: "default",
			Annotations: map[string]string{managedKeysAnnotation: "A"},
		},
		Data: map[string][]byte{"A": []byte("old"), "CONCURRENT": []byte("added-out-of-band")},
	}
	r := newFakeEnvironmentReconciler(obj, realSecret)

	suite.Require().NoError(r.upsertSecret(context.Background(), obj, true, staleExisting, map[string][]byte{"A": []byte("new")}))

	secret := &corev1.Secret{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "env", Namespace: "default"}, secret))
	suite.Equal("new", string(secret.Data["A"]))
	suite.Equal("added-out-of-band", string(secret.Data["CONCURRENT"]),
		"a key added concurrently to the real Secret must survive, not be dropped by the stale snapshot passed in")
}

func (suite *EnvironmentValuesControllerTestSuite) TestMaskSpecConflictSurfacesWithoutRetry() {
	// maskSpec writes obj directly (no internal Get-and-retry) precisely so a conflict is not
	// silently retried past: retrying would re-fetch the concurrently-changed object and then
	// overwrite its spec.env with maskedSpec anyway, since maskedSpec was computed from this
	// call's own stale snapshot - discarding whatever the concurrent edit added. A single Update
	// attempt that fails and returns is what lets that concurrent edit survive.
	obj := &appsv1alpha1.EnvironmentValues{ObjectMeta: metav1.ObjectMeta{Name: "env", Namespace: "default"}}
	callCount := 0
	r := newFakeEnvironmentReconcilerWithInterceptor(interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.UpdateOption) error {
			if _, ok := o.(*appsv1alpha1.EnvironmentValues); ok {
				callCount++
				return k8serrors.NewConflict(schema.GroupResource{Group: "apps.thunderid.io", Resource: "environmentvalues"}, "env", errors.New("conflict"))
			}
			return c.Update(ctx, o, opts...)
		},
	}, obj)

	err := r.maskSpec(context.Background(), obj, map[string]string{"A": "masked"})
	suite.Require().Error(err)
	suite.True(k8serrors.IsConflict(err))
	suite.Equal(1, callCount, "a conflict must surface immediately, not be silently retried and overwritten")
}
