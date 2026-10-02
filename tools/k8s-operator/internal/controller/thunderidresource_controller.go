// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

const resourceFinalizer = "thunderid.io/resource-cleanup"

// appLabelKey selects a serving pod/Deployment by instance name - the same "app: <instance name>"
// label thunderidinstance_controller.go's reconcileDeployment stamps on the Deployment/pod
// template, used here to find the pods belonging to a given instance.
const appLabelKey = "app"

// thunderidContainerName is the serving container's name in the Deployment built by
// reconcileDeployment - matched here against a pod's container statuses to find its health.
const thunderidContainerName = "thunderid"

// Bounds on fetchAndEmitPodErrors's crash-log fetch, which reads the whole response into memory
// but only ever inspects its tail. 500 lines is far more than the 5 it reports while still
// comfortably covering a startup failure's stack trace; the byte cap is the backstop for a
// container logging very long lines (the API server truncates mid-line at the limit).
const (
	podLogTailLines  = 500
	podLogLimitBytes = 1 << 20 // 1 MiB
)

// ThunderIDResource.Status.Phase values (a plain string field, not a typed enum - see ThunderIDResourceStatus).
// phaseError is also used by EnvironmentValuesReconciler for the same "something went wrong" meaning
// on EnvironmentValues.Status.Phase.
const (
	phaseError   = "Error"
	phasePending = "Pending"
	phaseReady   = "Ready"
)

// resourceKinds is the single registry of every ThunderID resource kind (spec.resource_type
// value) this operator forwards — adding a new kind means adding one entry here, full stop.
// Both reconcileConfigMap's resources.yaml merge (thunderidinstance_controller.go) and
// configHash's per-type version hashing read this same slice directly instead of each keeping
// their own hand-copied list of resource types, so the operator's behavior is consistent and doesn't drift over time.
//
// Order matters: the merge loop appends each kind's ConfigMap content in this slice's order,
// and ThunderID's own bootstrap loader wants that in dependency order (organization units
// first, since everything else can reference ouId).
var resourceKinds = []struct {
	resourceType    string
	configMapSuffix string
	fileKey         string
}{
	{"organization_unit", "organizationunits", "organizationunits.yaml"},
	{"user_type", "usertypes", "usertypes.yaml"},
	{"resource_server", "resourceservers", "resourceservers.yaml"},
	{"role", "roles", "roles.yaml"}, //nolint:goconst
	{"group", "groups", "groups.yaml"},
	{"user", "users", "users.yaml"},
	{"flow", "flows", "flows.yaml"},
	{"theme", "themes", "themes.yaml"},
	{"application", "applications", "applications.yaml"},
	{"translation", "translations", "translations.yaml"},
	{"server_config", "serverconfigs", "serverconfigs.yaml"},
	{"agent_type", "agenttypes", "agenttypes.yaml"},
	{"agent", "agents", "agents.yaml"},
	{"connection", "connections", "connections.yaml"},
	{"layout", "layouts", "layouts.yaml"},
	{"presentation_definition", "presentationdefinitions", "presentationdefinitions.yaml"},
	{"credential_configuration", "credentialconfigurations", "credentialconfigurations.yaml"},
	{"template", "templates", "templates.yaml"},
}

// resourceTypeMeta indexes resourceKinds by resource_type for the O(1) lookups below — built
// once from the single ordered list above, never maintained separately.
var resourceTypeMeta = func() map[string]struct{ configMapSuffix, fileKey string } {
	m := make(map[string]struct{ configMapSuffix, fileKey string }, len(resourceKinds))
	for _, k := range resourceKinds {
		m[k.resourceType] = struct{ configMapSuffix, fileKey string }{k.configMapSuffix, k.fileKey}
	}
	return m
}()

// parseResourceSpec decodes obj.Spec.Raw into a generic map and reads resource_type out of it —
// the one key the operator itself acts on, to know which ConfigMap this belongs in. The key stays
// in the returned map and is forwarded to ThunderID along with everything else (see
// resource_type.go's doc comment).
func parseResourceSpec(raw runtime.RawExtension) (resourceType string, data map[string]any, err error) {
	data = map[string]any{}
	if len(raw.Raw) > 0 {
		if err := json.Unmarshal(raw.Raw, &data); err != nil {
			return "", nil, fmt.Errorf("spec: %w", err)
		}
	}
	resourceType, _ = data["resource_type"].(string)
	return resourceType, data, nil
}

// ThunderIDResourceReconciler reconciles a ThunderIDResource object.
type ThunderIDResourceReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	KubeClient kubernetes.Interface
}

// +kubebuilder:rbac:groups=apps.thunderid.io,resources=thunderidresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.thunderid.io,resources=thunderidresources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.thunderid.io,resources=thunderidinstances,verbs=get;patch
// +kubebuilder:rbac:groups=apps.thunderid.io,resources=thunderidinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;create;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch

// Reconcile syncs one ThunderIDResource into its resource_type's ConfigMap (via rebuildResourcesConfigMap)
// and tracks whether ThunderID actually picked it up, via checkThunderIDHealth.
// nolint:gocyclo
func (r *ThunderIDResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the ThunderIDResource.
	obj := &appsv1alpha1.ThunderIDResource{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Re-injected into ctx so every sub-function below gets this resource's identity on its own
	// logf.FromContext(ctx) calls for free, without threading obj.Name through each of them.
	log = log.WithValues("resource", obj.Name, "namespace", obj.Namespace)
	ctx = logf.IntoContext(ctx, log)

	// Parse spec.resource_type out of the spec.
	resourceType, _, err := parseResourceSpec(obj.Spec)
	if err != nil {
		if !obj.DeletionTimestamp.IsZero() {
			log.Info("deleting ThunderIDResource with an invalid spec, dropping finalizer without syncing")
			obj.Finalizers = slices.DeleteFunc(obj.Finalizers, func(f string) bool { return f == resourceFinalizer })
			return ctrl.Result{}, r.Update(ctx, obj)
		}
		log.Error(err, "invalid ThunderIDResource spec")
		r.Recorder.Event(obj, corev1.EventTypeWarning, "InvalidSpec", err.Error())
		obj.Status.Phase = phaseError
		obj.Status.Synced = false
		_ = r.Status().Update(ctx, obj)
		return ctrl.Result{}, nil
	}
	log = log.WithValues("resourceType", resourceType)
	ctx = logf.IntoContext(ctx, log)

	// Deletion is handled before the instance lookup below: once the owning ThunderIDInstance
	// is itself gone, that lookup would return NotFound forever and this finalizer could never
	// clear (the object would be stuck Terminating indefinitely) if it were checked first.
	if !obj.DeletionTimestamp.IsZero() {
		if slices.Contains(obj.Finalizers, resourceFinalizer) {
			// Use the last-synced instance, not the current label: the label may have been
			// changed (or the object moved) after the finalizer was added, and the stale entry
			// actually lives under whichever instance it was last synced to, not wherever the
			// label points now. Falls back to the current label only if nothing was ever
			// successfully synced.
			cleanupInstanceName := obj.Status.SyncedInstance
			if cleanupInstanceName == "" {
				cleanupInstanceName = obj.Labels[appsv1alpha1.InstanceRefLabel]
			}
			instance := &appsv1alpha1.ThunderIDInstance{}
			if err := r.Get(ctx, client.ObjectKey{Namespace: obj.Namespace, Name: cleanupInstanceName}, instance); err != nil {
				if client.IgnoreNotFound(err) != nil {
					return ctrl.Result{}, err
				}
				// Owning instance already gone — its ConfigMaps are gone with it, nothing left
				// to remove this resource from. Fall through to just drop the finalizer.
				log.Info("owning ThunderIDInstance already gone, dropping finalizer without syncing")
			} else {
				// Use the last-synced type, not the freshly-parsed one: spec.resource_type may
				// have been edited to something invalid or unknown after the finalizer was added,
				// and that type is where this object's stale entry actually lives, not wherever
				// spec.resource_type points now. Falls back to the freshly-parsed type only if
				// nothing was ever successfully synced (the object never got past its first sync
				// attempt) - matches the old behavior for that edge case.
				cleanupType := obj.Status.SyncedResourceType
				if cleanupType == "" {
					cleanupType = resourceType
				}
				if _, ok := resourceTypeMeta[cleanupType]; !ok {
					// Nothing was ever validly synced under a known type - there's no ConfigMap
					// entry to remove. Erroring here (as rebuildResourcesConfigMap would) leaves
					// the finalizer stuck forever with no way to clear it.
					log.Info("no known resource_type to clean up on deletion, dropping finalizer without syncing", "cleanupType", cleanupType)
				} else {
					log.Info("ThunderIDResource deleted, removing it from the resources ConfigMap", "cleanupType", cleanupType)
					if _, err := r.rebuildResourcesConfigMap(ctx, instance, cleanupType, string(obj.UID)); err != nil {
						return ctrl.Result{}, err
					}
				}
			}
			obj.Finalizers = slices.DeleteFunc(obj.Finalizers, func(f string) bool { return f == resourceFinalizer })
			if err := r.Update(ctx, obj); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	instanceName := obj.Labels[appsv1alpha1.InstanceRefLabel]

	// Reject an unknown resource_type.
	if _, ok := resourceTypeMeta[resourceType]; !ok {
		log.Info("unknown spec.resource_type")
		// spec.resource_type may have been edited from a valid type to this one after a
		// successful sync: that earlier entry is stale now, and the sync path below - the only
		// other thing that purges it - is never reached while the type stays unknown.
		purged, err := r.purgeIfStale(ctx, obj, nil, instanceName, resourceType)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(obj, corev1.EventTypeWarning, "UnknownType", "spec.resource_type %q is not a known ThunderID resource_type", resourceType)
		obj.Status.Phase = phaseError
		obj.Status.Synced = false
		if purged {
			obj.Status.SyncedResourceType = ""
			obj.Status.SyncedInstance = ""
		}
		_ = r.Status().Update(ctx, obj)
		return ctrl.Result{}, nil
	}

	// Fetch the owning ThunderIDInstance.
	instance := &appsv1alpha1.ThunderIDInstance{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: obj.Namespace, Name: instanceName}, instance); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("ThunderIDInstance not found, waiting for it to be created", "instanceRef", instanceName)
			// Same reasoning as the unknown-type branch: if the thunderid.io/instance label was
			// repointed at an instance that doesn't exist (yet), the entry under the instance
			// this object was last synced to is stale, and requeuing until the new instance
			// shows up would leave it in that old ConfigMap indefinitely. A still-current
			// location (the owning instance merely deleted) isn't stale and is left alone - its
			// ConfigMaps went with it.
			purged, perr := r.purgeIfStale(ctx, obj, nil, instanceName, resourceType)
			if perr != nil {
				return ctrl.Result{}, perr
			}
			if purged {
				obj.Status.SyncedResourceType = ""
				obj.Status.SyncedInstance = ""
				obj.Status.Synced = false
				_ = r.Status().Update(ctx, obj)
			}
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		log.Error(err, "failed to get ThunderIDInstance", "instanceRef", instanceName)
		return ctrl.Result{}, err
	}

	// Add the finalizer on first sight.
	if !slices.Contains(obj.Finalizers, resourceFinalizer) {
		obj.Finalizers = append(obj.Finalizers, resourceFinalizer)
		if err := r.Update(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// spec.resource_type and/or the thunderid.io/instance label may have changed since the last
	// successful sync (e.g. role -> group, or moved to a different instance): the old
	// (instance, type) pair's ConfigMap still has this object's stale entry, and nothing else
	// will remove it — rebuildResourcesConfigMap for the new instance/type below only ever looks
	// at the new pair's own ConfigMap. Purge the old entry first, the same way deletion does,
	// before syncing into the new one.
	if _, err := r.purgeIfStale(ctx, obj, instance, instance.Name, resourceType); err != nil {
		obj.Status.Phase = phaseError
		obj.Status.Synced = false
		_ = r.Status().Update(ctx, obj)
		return ctrl.Result{}, err
	}

	// Sync into the resources ConfigMap.
	changed, err := r.rebuildResourcesConfigMap(ctx, instance, resourceType, "")
	if err != nil {
		log.Error(err, "failed to sync resources ConfigMap")
		r.Recorder.Event(obj, corev1.EventTypeWarning, "SyncFailed", err.Error())
		r.Recorder.Event(instance, corev1.EventTypeWarning, "ThunderIDResourceSyncFailed", fmt.Sprintf("ThunderIDResource %s (%s): %v", obj.Name, resourceType, err))
		obj.Status.Phase = phaseError
		obj.Status.Synced = false
		_ = r.Status().Update(ctx, obj)
		return ctrl.Result{}, err
	}

	// Record the pair this object is now synced to even though changed can be false here on a
	// genuine move: rebuildResourcesConfigMap lists every object currently bound to (instance,
	// resourceType), so a concurrent reconcile of a sibling resource can already have written the
	// exact same content (this object's entry included) moments earlier, before this reconcile's
	// own rebuild ran. Skipping this would leave Status.SyncedResourceType/SyncedInstance pointing
	// at the pair purgeIfStale just emptied above, and every later reconcile would attempt the same
	// pointless purge again - worse, on deletion the cleanup would target that now-empty old
	// location instead of this one, leaving the object's real entry behind in the new ConfigMap
	// forever.
	if obj.Status.SyncedResourceType != resourceType || obj.Status.SyncedInstance != instance.Name {
		obj.Status.SyncedResourceType = resourceType
		obj.Status.SyncedInstance = instance.Name
		if err := r.Status().Update(ctx, obj); err != nil {
			log.Error(err, "failed to record synced resource_type/instance")
			return ctrl.Result{}, err
		}
	}

	// Nothing changed - just verify ThunderID health instead of re-syncing.
	if obj.Status.Phase == phasePending && !changed {
		if obj.Status.LastSyncTime != nil {
			elapsed := time.Since(obj.Status.LastSyncTime.Time)
			if elapsed < 30*time.Second {
				return ctrl.Result{RequeueAfter: 30*time.Second - elapsed}, nil
			}
		}
		return r.checkThunderIDHealth(ctx, obj, instance)
	}

	if obj.Status.Phase == phaseError && !changed {
		return r.checkThunderIDHealth(ctx, obj, instance)
	}

	if !changed && obj.Status.Phase == phaseReady {
		return r.checkThunderIDHealth(ctx, obj, instance)
	}

	// Mark Pending and requeue to verify ThunderID picked it up.
	now := metav1.Now()
	r.Recorder.Event(obj, corev1.EventTypeNormal, "Synced", "ThunderIDResource synced to ConfigMap, waiting for ThunderID to apply")
	obj.Status.Phase = phasePending
	obj.Status.Synced = false
	obj.Status.LastSyncTime = &now
	obj.Status.ID = string(obj.UID)
	obj.Status.SyncedResourceType = resourceType
	obj.Status.SyncedInstance = instance.Name
	if err := r.Status().Update(ctx, obj); err != nil {
		log.Error(err, "failed to update resource status")
		return ctrl.Result{}, err
	}

	log.Info("resource synced; requeuing in 30s to verify ThunderID health", "name", obj.Name, "type", resourceType)
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// purgeIfStale removes obj's entry from the ConfigMap it was last successfully synced to
// (Status.SyncedInstance + Status.SyncedResourceType), when that is no longer the location obj
// would sync to now. It returns true once that stale location has been dealt with — purged, or
// found already gone — so a caller that stops reconciling here (an unknown resource_type, a
// missing owning instance) knows it can clear those two status fields and not re-purge the same
// entry on every subsequent reconcile. It returns false when nothing was ever synced, or when
// the recorded location is still the current one: the status fields are accurate then and must
// be left alone.
//
// currentInstance is the already-fetched ThunderIDInstance named by newInstanceName when the
// caller has one, purely to skip a redundant Get, or nil when it has none (not fetched yet, or
// no longer exists).
func (r *ThunderIDResourceReconciler) purgeIfStale(ctx context.Context, obj *appsv1alpha1.ThunderIDResource,
	currentInstance *appsv1alpha1.ThunderIDInstance, newInstanceName, newResourceType string) (bool, error) {
	log := logf.FromContext(ctx)

	oldType := obj.Status.SyncedResourceType
	if oldType == "" {
		// Never successfully synced anywhere - there's no entry to remove.
		return false, nil
	}
	oldInstanceName := obj.Status.SyncedInstance
	if oldInstanceName == "" {
		// Synced before SyncedInstance was recorded: the only instance it can have gone to is
		// whichever one the object is bound to now.
		oldInstanceName = newInstanceName
	}
	if oldInstanceName == "" {
		return false, nil
	}
	if oldInstanceName == newInstanceName && oldType == newResourceType {
		// Still the current location - an ordinary re-sync, not a move.
		return false, nil
	}

	oldInstance := currentInstance
	if oldInstance == nil || oldInstanceName != oldInstance.Name {
		fetched := &appsv1alpha1.ThunderIDInstance{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: obj.Namespace, Name: oldInstanceName}, fetched); err != nil {
			if client.IgnoreNotFound(err) != nil {
				return false, err
			}
			// Old instance already gone - its ConfigMaps went with it, nothing left to remove
			// this resource from.
			return true, nil
		}
		oldInstance = fetched
	}
	if _, ok := resourceTypeMeta[oldType]; !ok {
		// A type this operator no longer knows maps to no ConfigMap at all.
		return true, nil
	}

	log.Info("spec.resource_type or instance changed, removing stale entry from old ConfigMap", "oldInstance", oldInstance.Name, "oldType", oldType)
	if _, err := r.rebuildResourcesConfigMap(ctx, oldInstance, oldType, string(obj.UID)); err != nil {
		log.Error(err, "failed to clean up old ConfigMap", "oldInstance", oldInstance.Name, "oldType", oldType)
		r.Recorder.Eventf(obj, corev1.EventTypeWarning, "SyncFailed", "failed to remove stale entry from old instance %q / resource_type %q's ConfigMap: %v", oldInstance.Name, oldType, err)
		return false, err
	}
	return true, nil
}

// checkThunderIDHealth inspects the serving pod's container status for obj's owning instance:
// a crash sets Phase=Error (and, once escalated to CrashLoopBackOff, emits the pod's error logs
// via fetchAndEmitPodErrors); otherwise it sets Phase=Ready/Synced=true once healthy. Every
// diagnostic Event (CrashLoopBackOff/missing-secret here, ThunderIDError/ThunderIDCrashed in
// fetchAndEmitPodErrors) is emitted on instance, never on obj: the pod - and therefore its crash
// reason and logs - is shared by every ThunderIDResource bound to instance, so a diagnosis
// attached to whichever resource happens to be polling would misattribute it to that resource
// specifically, and every other resource re-verifying health during the same outage would
// independently re-diagnose and re-emit the identical message onto itself too. Targeting instance
// also means repeated calls for the same ongoing issue collapse into one Event with an
// incrementing count (the EventRecorder's own dedup keys on object+reason+message), instead of
// spamming a separate misattributed Event per bystander resource.
func (r *ThunderIDResourceReconciler) checkThunderIDHealth(ctx context.Context, obj *appsv1alpha1.ThunderIDResource, instance *appsv1alpha1.ThunderIDInstance) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Find the serving pods.
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(instance.Namespace), client.MatchingLabels{appLabelKey: instance.Name}); err != nil {
		return ctrl.Result{}, err
	}

	for _, pod := range podList.Items {
		unhealthy, issue := diagnosePod(&pod)
		if !unhealthy {
			continue
		}
		log.Info("ThunderID pod is unhealthy", "pod", pod.Name, "reason", issue.reason)

		message := issue.message
		if extra := diagnoseMissingSecret(ctx, r.Client, instance, issue); extra != "" {
			message = extra
		}
		if obj.Status.Phase != phaseError {
			obj.Status.Phase = phaseError
			obj.Status.Synced = false
			_ = r.Status().Update(ctx, obj)
		}
		if message != "" {
			r.Recorder.Event(instance, corev1.EventTypeWarning, issue.reason, message)
		}
		// CrashLoopBackOff is the only reason with real container logs worth fetching; every
		// other reason (a config-resolution failure, or a crash not yet escalated to
		// CrashLoopBackOff) keeps polling instead.
		if issue.reason != reasonCrashLoopBackOff {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		r.fetchAndEmitPodErrors(ctx, instance, instance.Namespace, pod.Name)
		return ctrl.Result{}, nil
	}

	// No crash found - mark Ready if not already.
	if obj.Status.Phase != phaseReady || !obj.Status.Synced {
		r.Recorder.Event(obj, corev1.EventTypeNormal, "Ready", "ThunderID is healthy; resource is active")
		obj.Status.Phase = phaseReady
		obj.Status.Synced = true
		if err := r.Status().Update(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("ThunderID healthy, resource ready", "name", obj.Name)
	}
	return ctrl.Result{}, nil
}

// fetchAndEmitPodErrors fetches podName's crashed container logs (preferring the previous
// instance's logs, falling back to the current one) and emits up to 5 ERROR-level lines as
// ThunderIDError events, or — if none matched — the last 5 lines of output as ThunderIDCrashed
// events instead, all on target (see checkThunderIDHealth's doc comment for why never a specific
// ThunderIDResource). target is client.Object rather than the concrete ThunderIDInstance type
// since Recorder.Event is the only thing done with it here.
func (r *ThunderIDResourceReconciler) fetchAndEmitPodErrors(ctx context.Context, target client.Object, namespace, podName string) {
	// Fetch the crashed container's logs, preferring the previous instance. Bounded on both axes
	// because the whole response is read into memory below: only the tail is ever inspected (the
	// last 5 lines, or the first 5 ERROR lines within that tail), so an unbounded request would
	// pull a crash-looping container's entire log history — arbitrarily large — just to discard
	// nearly all of it.
	logOptions := func(previous bool) *corev1.PodLogOptions {
		tail, limit := int64(podLogTailLines), int64(podLogLimitBytes)
		return &corev1.PodLogOptions{
			Previous:   previous,
			Container:  thunderidContainerName,
			TailLines:  &tail,
			LimitBytes: &limit,
		}
	}
	req := r.KubeClient.CoreV1().Pods(namespace).GetLogs(podName, logOptions(true))
	rc, err := req.Stream(ctx)
	if err != nil {
		req = r.KubeClient.CoreV1().Pods(namespace).GetLogs(podName, logOptions(false))
		rc, err = req.Stream(ctx)
	}
	if err != nil {
		r.Recorder.Event(target, corev1.EventTypeWarning, "ThunderIDCrashed",
			fmt.Sprintf("pod %s is in CrashLoopBackOff (could not fetch logs: %v)", podName, err))
		return
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		r.Recorder.Event(target, corev1.EventTypeWarning, "ThunderIDCrashed",
			fmt.Sprintf("pod %s is in CrashLoopBackOff", podName))
		return
	}

	// Emit up to 5 ERROR-level lines as events.
	lines := strings.Split(string(data), "\n")
	errCount := 0
	for _, line := range lines {
		if line == "" {
			continue
		}
		if strings.Contains(line, "level=ERROR") || strings.Contains(line, `"level":"ERROR"`) {
			r.Recorder.Event(target, corev1.EventTypeWarning, "ThunderIDError", line)
			errCount++
			if errCount >= 5 {
				break
			}
		}
	}

	// No ERROR lines found - emit the last 5 lines instead.
	if errCount == 0 {
		var tail []string
		for i := len(lines) - 1; i >= 0 && len(tail) < 5; i-- {
			if lines[i] != "" {
				tail = append([]string{lines[i]}, tail...)
			}
		}
		for _, line := range tail {
			r.Recorder.Event(target, corev1.EventTypeWarning, "ThunderIDCrashed", line)
		}
	}
}

// rebuildResourcesConfigMap returns true if the ConfigMap content changed. It lists every
// ThunderIDResource whose spec.resource_type matches resourceType and is bound to instance, renders
// each, and writes them into the ConfigMap resourceTypeMeta maps that type to — the same
// ConfigMap the original per-kind controller for that type would have written.
func (r *ThunderIDResourceReconciler) rebuildResourcesConfigMap(ctx context.Context, instance *appsv1alpha1.ThunderIDInstance, resourceType, excludeUID string) (bool, error) {
	// Look up which ConfigMap/key this resource_type renders into.
	meta, ok := resourceTypeMeta[resourceType]
	if !ok {
		return false, fmt.Errorf("spec.resource_type %q is not a known ThunderID resource_type", resourceType)
	}

	// List and sort every ThunderIDResource of this type bound to instance.
	list := &appsv1alpha1.ThunderIDResourceList{}
	if err := r.List(ctx, list, client.InNamespace(instance.Namespace)); err != nil {
		return false, err
	}
	slices.SortFunc(list.Items, func(a, b appsv1alpha1.ThunderIDResource) int {
		return strings.Compare(a.Name, b.Name)
	})

	// Render each matching item into the ConfigMap content.
	var buf bytes.Buffer
	for _, o := range list.Items {
		itemType, itemData, err := parseResourceSpec(o.Spec)
		if err != nil {
			return false, fmt.Errorf("resource %s: %w", o.Name, err)
		}
		if itemType != resourceType {
			continue
		}
		if o.Labels[appsv1alpha1.InstanceRefLabel] != instance.Name {
			continue
		}
		if excludeUID != "" && string(o.UID) == excludeUID {
			continue
		}
		if !o.DeletionTimestamp.IsZero() {
			continue
		}
		buf.WriteString("---\n")
		buf.WriteString(r.buildResourceYAML(o, itemData))
	}
	content := buf.String()

	// Hash the content and skip if nothing changed.
	h := sha256.New()
	h.Write([]byte(content))
	version := hex.EncodeToString(h.Sum(nil))[:16]

	versionAnnotation := "thunderid.io/" + meta.configMapSuffix + "-version"
	if instance.Annotations[versionAnnotation] == version {
		return false, nil
	}

	// Get or create/update the ConfigMap.
	cmName := instance.Name + "-" + meta.configMapSuffix
	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: cmName}, cm)
	if errors.IsNotFound(err) {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmName,
				Namespace: instance.Namespace,
			},
			Data: map[string]string{meta.fileKey: content},
		}
		_ = ctrl.SetControllerReference(instance, cm, r.Scheme)
		if err := r.Create(ctx, cm); err != nil {
			return false, fmt.Errorf("failed to create %s ConfigMap: %w", meta.configMapSuffix, err)
		}
	} else if err != nil {
		return false, err
	} else {
		updateErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			latest := &corev1.ConfigMap{}
			if err := r.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: cmName}, latest); err != nil {
				return err
			}
			if latest.Data == nil {
				latest.Data = map[string]string{}
			}
			latest.Data[meta.fileKey] = content
			return r.Update(ctx, latest)
		})
		if updateErr != nil {
			return false, fmt.Errorf("failed to update %s ConfigMap: %w", meta.configMapSuffix, updateErr)
		}
	}

	// Record the new content hash on the instance.
	patch := client.MergeFrom(instance.DeepCopy())
	if instance.Annotations == nil {
		instance.Annotations = map[string]string{}
	}
	instance.Annotations[versionAnnotation] = version
	return true, r.Patch(ctx, instance, patch)
}

// renderResourceYAML renders data (a decoded JSON document — so only string/bool/nil/float64/
// map[string]any/[]any values appear in it) as a YAML document via gopkg.in/yaml.v3, the same
// library ThunderID's own declarative resource parsers (application, theme, flow, ou, role, ...)
// use to read it back, including \Uxxxxxxxx-escaped non-BMP runes (emoji) in marshaled strings,
// which ThunderID's parser decodes correctly. Marshal errors are not handled because data only
// ever contains values that came out of encoding/json.Unmarshal, all of which are directly
// encodable.
func renderResourceYAML(data map[string]any) string {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	_ = enc.Encode(data)
	_ = enc.Close()
	return buf.String()
}

// buildResourceYAML generates a YAML document for a single resource in the format ThunderID's
// declarative resource loader expects. data is o.Spec already decoded (see parseResourceSpec) —
// the caller parses once and passes it in rather than every item re-parsing its own spec twice.
func (r *ThunderIDResourceReconciler) buildResourceYAML(o appsv1alpha1.ThunderIDResource, data map[string]any) string {
	data["id"] = string(o.UID)
	return renderResourceYAML(data)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ThunderIDResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.ThunderIDResource{}).
		// No GenerationChangedPredicate here (unlike the ThunderIDInstance watch below):
		// generation only bumps on spec changes, but the whole point of watching Deployment is
		// to react to pod-health transitions (crash <-> healthy), which are status-only changes.
		// A GenerationChangedPredicate silently drops exactly those events, leaving Phase=Error
		// resources permanently wedged until an unrelated spec-changing rollout happens to land
		// while the pod's actually healthy. resourcesForDeployment already does its own filtering
		// (Error: always re-check; Ready/Pending: only if unavailable), so no event-level
		// predicate is needed on top.
		Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(r.resourcesForDeployment)).
		Watches(&appsv1alpha1.ThunderIDInstance{}, handler.EnqueueRequestsFromMapFunc(r.resourcesForInstance),
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named("resource").
		Complete(r)
}

// resourcesForDeployment maps a Deployment change to Resources that reference it.
// For Phase=Error resources: always re-trigger (pod may have recovered).
// For Phase=Ready resources: re-trigger only when the Deployment is unhealthy.
func (r *ThunderIDResourceReconciler) resourcesForDeployment(ctx context.Context, obj client.Object) []reconcile.Request {
	dep, ok := obj.(*appsv1.Deployment)
	if !ok {
		return nil
	}
	// List every ThunderIDResource in this namespace.
	list := &appsv1alpha1.ThunderIDResourceList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}
	var reqs []reconcile.Request
	for _, o := range list.Items {
		if o.Labels[appsv1alpha1.InstanceRefLabel] != dep.GetName() {
			continue
		}
		switch o.Status.Phase {
		case phaseError:
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: o.Name, Namespace: o.Namespace},
			})
		case phaseReady, phasePending:
			if dep.Status.UnavailableReplicas > 0 {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: o.Name, Namespace: o.Namespace},
				})
			}
		}
	}
	return reqs
}

// resourcesForInstance maps a ThunderIDInstance change to every ThunderIDResource bound to it.
func (r *ThunderIDResourceReconciler) resourcesForInstance(ctx context.Context, obj client.Object) []reconcile.Request {
	list := &appsv1alpha1.ThunderIDResourceList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}
	var reqs []reconcile.Request
	for _, o := range list.Items {
		if o.Labels[appsv1alpha1.InstanceRefLabel] == obj.GetName() {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: o.Name, Namespace: o.Namespace},
			})
		}
	}
	return reqs
}
