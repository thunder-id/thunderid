// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

// maskPrefix marks a spec.env value the controller has already synced to the Secret and
// overwritten in place. A value not matching maskPattern is treated as new plaintext to sync —
// this prefix+suffix combination is specific enough that a real credential accidentally
// colliding with it isn't a realistic concern.
const maskPrefix = "thunderid-masked:"

var maskPattern = regexp.MustCompile(`^thunderid-masked:.{0,3}\*{10}$`)

func isMasked(v string) bool {
	return maskPattern.MatchString(v)
}

// managedKeysAnnotation records, on the generated Secret, exactly which keys this controller
// last wrote there — the only keys it's ever safe to remove on a later sync. A Secret sharing
// this EnvironmentValues's name but never written by it (hand-made, or otherwise missing the
// annotation) carries no such record, so its keys are left alone instead of being wiped by
// treating the Secret's Data as a full replace.
const managedKeysAnnotation = "thunderid.io/managed-keys"

// lastAppliedConfigAnnotation is written by plain (client-side) `kubectl apply`, computed and
// sent by the client itself before this controller ever sees the object — it embeds the full
// submitted manifest, plaintext spec.env included, bypassing the masking below entirely. Stripping
// it after the fact wouldn't help: the next plain apply of the same file recomputes and rewrites it
// with plaintext again, regardless of what the controller did in between. Refusing to sync while
// it's present (see Reconcile) enforces the --server-side-only requirement documented on
// EnvironmentValues instead of just asking nicely.
const lastAppliedConfigAnnotation = "kubectl.kubernetes.io/last-applied-configuration"

// managedKeySet parses secret's managedKeysAnnotation into a set of key names, or an empty set
// if the annotation is absent (a Secret this controller has never written to).
func managedKeySet(secret *corev1.Secret) map[string]bool {
	set := map[string]bool{}
	raw := secret.Annotations[managedKeysAnnotation]
	if raw == "" {
		return set
	}
	for k := range strings.SplitSeq(raw, ",") {
		if k != "" {
			set[k] = true
		}
	}
	return set
}

// maskValue reveals at most the first 3 characters of v (never all of it, even for a value 3
// characters or shorter — a short value gets a shorter reveal instead) so a developer can
// eyeball "is this the value I think it is" without the full secret ever living in etcd. The
// star run is always exactly 10 characters, and v's real length is never stated anywhere in the
// output — both deliberately: v's length isn't needed for the eyeball check the reveal already
// gives, and stating it (exactly, or even as a coarse bucket) alongside a fixed-size reveal would
// narrow a short value down to very few possibilities — e.g. a 4-character value would have only
// one truly unknown character left if its exact length were known too.
func maskValue(v string) string {
	n := len(v)
	reveal := min(3, max(n-1, 0))
	masked := v[:reveal] + strings.Repeat("*", 10)
	return maskPrefix + masked
}

// EnvironmentValuesReconciler mirrors spec.env into a Secret of the same name, then overwrites each
// synced key's value in spec.env with a masked marker (up to 3 leading characters + a fixed-size
// star run, never the rest of the raw value, and never its real length either) — see
// EnvironmentValues's own doc comment for the reasoning.
// Status.Keys lists which keys are currently synced (names only, no values) for debugging "did
// my key actually get picked up."
//
// Updating a value later means re-applying a manifest naming only that key, with
// `kubectl apply --server-side --force-conflicts`: the masking write above already owns every
// synced key's field, so --force-conflicts is needed to retake ownership of the one key being
// changed. Do NOT put EnvironmentValues under GitOps (Flux) management — see the GitOps
// (Flux) section of README.md.
type EnvironmentValuesReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.thunderid.io,resources=environmentvalues,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=apps.thunderid.io,resources=environmentvalues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;create;update;patch

// Reconcile syncs an EnvironmentValues's spec.env into a same-named Secret and masks each newly-synced
// key's value in place, then reflects the result (Phase/Synced/SecretName/Keys) in its status.
func (r *EnvironmentValuesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the EnvironmentValues.
	obj := &appsv1alpha1.EnvironmentValues{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Re-injected into ctx so every sub-function below gets this EnvironmentValues's identity on its
	// own logf.FromContext(ctx) calls for free, without threading obj.Name through each of them.
	log = log.WithValues("environment", obj.Name, "namespace", obj.Namespace)
	ctx = logf.IntoContext(ctx, log)

	// A plain `kubectl apply` embeds this EnvironmentValues's full plaintext spec.env into this
	// annotation client-side, before the controller ever runs — bypassing masking entirely and,
	// unlike a hand-written Secret, exposing it to a wider audience than Secret RBAC would (see
	// lastAppliedConfigAnnotation's doc comment). Refuse to sync at all while it's present rather
	// than masking spec.env and leaving the annotation's copy behind unmasked.
	if _, ok := obj.Annotations[lastAppliedConfigAnnotation]; ok {
		log.Info("refusing to sync: object carries " + lastAppliedConfigAnnotation + " from a plain `kubectl apply`")
		r.Recorder.Eventf(obj, corev1.EventTypeWarning, "ClientSideApplyDetected",
			"%s is present, meaning this object was applied with plain `kubectl apply` instead of "+
				"--server-side; that embeds plaintext spec.env in the annotation, bypassing masking. "+
				"Strip it (kubectl annotate environmentvalues %s %s- -n %s) and re-apply with "+
				"--server-side to resync.",
			lastAppliedConfigAnnotation, obj.Name, lastAppliedConfigAnnotation, obj.Namespace)
		obj.Status.Phase = phaseError
		obj.Status.Synced = false
		obj.Status.SecretName = obj.Name
		_ = r.Status().Update(ctx, obj)
		return ctrl.Result{}, nil
	}

	// Sync spec.env into the Secret and mask any newly-synced keys.
	keys, missingKeys, maskedAny, err := r.syncSecretAndMask(ctx, obj)
	if err != nil {
		log.Error(err, "failed to sync Secret from EnvironmentValues")
		r.Recorder.Event(obj, corev1.EventTypeWarning, "SyncFailed", err.Error())
		obj.Status.Phase = phaseError
		obj.Status.Synced = false
		obj.Status.SecretName = obj.Name
		_ = r.Status().Update(ctx, obj)
		return ctrl.Result{}, err
	}

	// A key masked in spec.env but missing from the Secret (e.g. deleted out-of-band) can't be
	// recovered from its marker — surface it as unsynced instead of silently reporting it in
	// status.keys as if it were still backed by a real value.
	if len(missingKeys) > 0 {
		log.Info("key(s) masked in spec.env are missing from the Secret", "missingKeys", missingKeys)
		r.Recorder.Eventf(obj, corev1.EventTypeWarning, "MissingSecretValue",
			"key(s) %v are masked in spec.env but missing from Secret %q; re-apply with plaintext to resync them",
			missingKeys, obj.Name)
		obj.Status.Phase = phaseError
		obj.Status.Synced = false
		obj.Status.SecretName = obj.Name
		obj.Status.Keys = keys
		_ = r.Status().Update(ctx, obj)
		return ctrl.Result{}, nil
	}

	// Reflect the result in status.
	obj.Status.Phase = phaseSynced
	obj.Status.Synced = true
	obj.Status.SecretName = obj.Name
	obj.Status.Keys = keys
	if err := r.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}

	// Only fires the reconcile that actually masked new plaintext — an already-fully-masked
	// object is a no-op above this point, so this doesn't spam an event every time something
	// else (e.g. the status write itself) re-triggers a reconcile.
	if maskedAny {
		r.Recorder.Eventf(obj, corev1.EventTypeNormal, "SecretSynced",
			"Secret %q synced from this EnvironmentValues's spec.env; synced key values are now masked "+
				"in spec.env (see status.keys for the current key names). Re-apply with new "+
				"plaintext for a key to change it.", obj.Name)
	}

	return ctrl.Result{}, nil
}

// syncSecretAndMask makes the Secret's Data an exact match for obj.Spec.Env's real values
// (a masked key keeps whatever the Secret already holds; a new-plaintext key is written then
// masked in place) and returns the current key names, plus any masked key whose value is missing
// from the Secret (see the loop below). It persists the spec mutation via Update only when at
// least one key held new plaintext this pass, so a fully-masked object converges instead of
// looping.
func (r *EnvironmentValuesReconciler) syncSecretAndMask(ctx context.Context, obj *appsv1alpha1.EnvironmentValues) (keys, missingKeys []string, maskedAny bool, err error) {
	// Fetch the existing Secret, if any.
	existing := &corev1.Secret{}
	getErr := r.Get(ctx, client.ObjectKey{Namespace: obj.Namespace, Name: obj.Name}, existing)
	found := true
	if errors.IsNotFound(getErr) {
		found = false
	} else if getErr != nil {
		return nil, nil, false, getErr
	}

	data := make(map[string][]byte, len(obj.Spec.Env))
	maskedSpec := make(map[string]string, len(obj.Spec.Env))
	keys = make([]string, 0, len(obj.Spec.Env))
	var newlyMasked []string

	// Split spec.env into already-masked (carry over) and new-plaintext (write and mask) keys.
	for k, v := range obj.Spec.Env {
		if isMasked(v) {
			// Already synced on a previous reconcile — carry over whatever the Secret already
			// holds for this key rather than the marker itself. If the Secret lost the key (e.g.
			// someone deleted it out-of-band) there's no way to recover the real value from the
			// marker — report it as missing instead of claiming it's still synced.
			maskedSpec[k] = v
			if found {
				if b, ok := existing.Data[k]; ok {
					data[k] = b
					keys = append(keys, k)
					continue
				}
			}
			missingKeys = append(missingKeys, k)
			continue
		}
		keys = append(keys, k)
		data[k] = []byte(v)
		maskedSpec[k] = maskValue(v)
		maskedAny = true
		newlyMasked = append(newlyMasked, k)
	}
	slices.Sort(keys)

	// Write the Secret.
	if err := r.upsertSecret(ctx, obj, found, existing, data); err != nil {
		return nil, nil, false, err
	}

	if maskedAny {
		// Key names only, never values — same "names are safe, values never are" rule this
		// controller's own status.keys field already follows.
		slices.Sort(newlyMasked)
		logf.FromContext(ctx).Info("synced new plaintext key(s) to the Secret and masked them in spec.env",
			"newlyMaskedKeys", newlyMasked, "totalKeys", len(keys))
		if err := r.maskSpec(ctx, obj, maskedSpec); err != nil {
			return nil, nil, false, err
		}
	}

	return keys, missingKeys, maskedAny, nil
}

// upsertSecret creates the Secret if it doesn't exist yet, or otherwise merges data into it:
// keys this controller previously managed but no longer wants are dropped, this pass's data is
// laid on top, and any other pre-existing key (never managed by this EnvironmentValues) is left alone.
func (r *EnvironmentValuesReconciler) upsertSecret(ctx context.Context, obj *appsv1alpha1.EnvironmentValues, found bool, existing *corev1.Secret, data map[string][]byte) error {
	log := logf.FromContext(ctx)
	// Track which keys this pass manages, for managedKeysAnnotation.
	managedKeys := make([]string, 0, len(data))
	for k := range data {
		managedKeys = append(managedKeys, k)
	}
	slices.Sort(managedKeys)
	managedKeysValue := strings.Join(managedKeys, ",")

	// Create if it doesn't exist yet.
	if !found {
		log.Info("creating Secret", "managedKeys", managedKeys)
		return r.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        obj.Name,
				Namespace:   obj.Namespace,
				Labels:      map[string]string{appsv1alpha1.EnvironmentValuesAdoptLabel: obj.Name},
				Annotations: map[string]string{managedKeysAnnotation: managedKeysValue},
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		})
	}

	// A Secret this controller has never written to before (no managedKeysAnnotation) must
	// explicitly opt in via EnvironmentValuesAdoptLabel before it's touched — matching by Secret
	// name alone would let anyone who can create an EnvironmentValues overwrite keys in any
	// same-named Secret in the namespace, including ones they have no RBAC permission to edit
	// directly. A Secret this controller already manages always carries the label itself (set on
	// create, just above), so this only gates the first touch of a pre-existing, hand-written one.
	if len(managedKeySet(existing)) == 0 && existing.Labels[appsv1alpha1.EnvironmentValuesAdoptLabel] != obj.Name {
		return fmt.Errorf(
			"secret %q already exists and is not labeled for adoption by this EnvironmentValues "+
				"(add label %s=%s to the Secret to allow it)",
			obj.Name, appsv1alpha1.EnvironmentValuesAdoptLabel, obj.Name)
	}

	// Merge, not replace: start from whatever the Secret already has, drop only the keys this
	// controller previously wrote (per managedKeysAnnotation) that are no longer in spec.env —
	// preserving the existing declarative "dropped key stops being synced" behavior — then lay
	// this pass's data on top. A key neither previously-managed nor in this pass's data (a
	// foreign key) is never touched.
	merge := func(base *corev1.Secret) map[string][]byte {
		previouslyManaged := managedKeySet(base)
		finalData := make(map[string][]byte, len(base.Data)+len(data))
		maps.Copy(finalData, base.Data)
		for k := range previouslyManaged {
			if _, stillWanted := data[k]; !stillWanted {
				delete(finalData, k)
			}
		}
		maps.Copy(finalData, data)
		return finalData
	}

	finalData := merge(existing)
	// With no spec.env keys there is nothing to adopt - this is the documented "hand-written Secret
	// plus an empty EnvironmentValues" setup, which never writes to the Secret, so don't warn.
	if existing.Annotations[managedKeysAnnotation] == "" && len(existing.Data) > 0 && len(managedKeys) > 0 {
		log.Info("Secret already existed with unmanaged key(s), adopting only the keys spec.env names", "managedKeys", managedKeys)
		r.Recorder.Eventf(obj, corev1.EventTypeWarning, "AdoptedExistingSecret",
			"Secret %q already existed with key(s) not previously managed by this EnvironmentValues; "+
				"those are left untouched, and only %v are now managed by this EnvironmentValues.",
			obj.Name, managedKeys)
	}

	if reflect.DeepEqual(existing.Data, finalData) && existing.Annotations[managedKeysAnnotation] == managedKeysValue {
		log.V(1).Info("Secret unchanged, skipping update")
		return nil
	}

	log.Info("Secret content changed, updating", "managedKeys", managedKeys)
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := &corev1.Secret{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: obj.Namespace, Name: obj.Name}, latest); err != nil {
			return err
		}
		// Recomputed from latest, not the outer finalData snapshot: a retry only happens after a
		// conflict, meaning latest.Data may hold a concurrent change (e.g. a foreign key added
		// out-of-band) made since this function's own initial Get - overwriting with the stale
		// snapshot would silently drop it even though the write itself succeeds. Must run before
		// the managedKeysAnnotation write just below: merge reads that same annotation off latest
		// to know what was previously managed, so writing the new value first would make every
		// key look previously-managed as of this pass and never get dropped.
		newData := merge(latest)
		if latest.Annotations == nil {
			latest.Annotations = map[string]string{}
		}
		latest.Annotations[managedKeysAnnotation] = managedKeysValue
		latest.Data = newData
		latest.StringData = nil
		return r.Update(ctx, latest)
	})
}

// maskSpec persists obj.Spec.Env = maskedSpec so the raw value of any key synced this pass is
// never visible via `kubectl get/describe environmentvalues` again. Writes using obj's own
// ResourceVersion (from Reconcile's original Get) rather than retrying against a freshly-fetched
// copy: maskedSpec was computed from obj.Spec.Env as read at the start of this reconcile, so
// blindly retrying past a conflict and overwriting spec.env with that now-stale value would
// silently discard whatever a concurrent edit (e.g. a new key added via kubectl apply) just
// wrote. Letting the conflict surface instead means the caller's error path runs and
// controller-runtime requeues, re-reading the current spec.env from scratch next time.
func (r *EnvironmentValuesReconciler) maskSpec(ctx context.Context, obj *appsv1alpha1.EnvironmentValues, maskedSpec map[string]string) error {
	obj.Spec.Env = maskedSpec
	return r.Update(ctx, obj)
}

// SetupWithManager sets up the controller with the Manager.
func (r *EnvironmentValuesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.EnvironmentValues{}).
		Named("environmentvalues").
		Complete(r)
}
