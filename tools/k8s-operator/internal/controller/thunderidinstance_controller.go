// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

//go:embed deployment-config.yaml
var deploymentConfigTemplate string

// initContainerImage is used for init containers that only need plain POSIX utilities
// (cp, chmod, stat, chown) — not the ThunderID app itself. Pinned to a specific tag, unlike
// instance.Spec.Image which is typically ":latest": a floating tag forces imagePullPolicy to
// default to Always, so every container built from it re-checks the registry on every pod
// start. Pinning here lets these two containers safely use IfNotPresent instead, while the
// main "thunderid" container keeps its own image and Always semantics untouched.
const initContainerImage = "busybox:1.36"

// ThunderIDInstanceReconciler reconciles a ThunderIDInstance object
type ThunderIDInstanceReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	KubeClient kubernetes.Interface
}

// RBAC annotations for the controller
// +kubebuilder:rbac:groups=apps.thunderid.io,resources=thunderidinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.thunderid.io,resources=thunderidinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.thunderid.io,resources=thunderidinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps;services;secrets;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete

// Reconcile brings every object a ThunderIDInstance owns — its config ConfigMap, security Secret,
// bootstrap Job, data PersistentVolumeClaim, Deployment, Service, Ingress, and HPA — in line with
// its spec, then updates the instance's own status. This is the controller's single entry point.
func (r *ThunderIDInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	// Fetch the instance.
	instance := &appsv1alpha1.ThunderIDInstance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Re-injected into ctx so every sub-function below gets this instance's identity on its own
	// logf.FromContext(ctx) calls for free, without threading instance.Name through each of them.
	log = log.WithValues("thunderIDInstance", instance.Name, "namespace", instance.Namespace)
	ctx = logf.IntoContext(ctx, log)

	// Require the operator to persist state one way or the other.
	if instance.Spec.DataVolumeSize == "" && !usingPostgres(instance) {
		err := fmt.Errorf("spec.dataVolumeSize must be set, or every spec.config.database scope's type set to postgres")
		log.Error(err, "invalid ThunderIDInstance spec")
		r.Recorder.Event(instance, corev1.EventTypeWarning, "InvalidSpec", err.Error())
		return ctrl.Result{}, err
	}

	// spec.config.port defaults to 8090 — also used for the container port, all three probes, the
	// Service, and the Ingress backend (see reconcileDeployment/reconcileService/reconcileIngress).
	port := instance.Spec.Config.Port
	if port == 0 {
		port = 8090
	}

	// No spec.domain means no Ingress (see reconcileIngress) — ThunderID is reached via
	// kubectl port-forward instead, which always targets localhost:<port>.
	publicURL := fmt.Sprintf("https://localhost:%d", port)
	if instance.Spec.Domain != "" {
		publicURL = "https://" + instance.Spec.Domain
	}

	// Reconcile the config ConfigMap.
	resourcesContent, baseResourcesContent, databaseBlock, emailBlock, err := r.reconcileConfigMap(ctx, instance, publicURL)
	if err != nil {
		log.Error(err, "failed to reconcile ConfigMap")
		r.Recorder.Event(instance, corev1.EventTypeWarning, "ConfigMapError", err.Error())
		return ctrl.Result{}, err
	}

	// Resolve the env Secret hash so a Secret-only change still rolls the pod.
	envSecretHash, err := r.resolveEnvSecretHash(ctx, instance)
	if err != nil {
		log.Error(err, "failed to resolve env Secret")
		r.Recorder.Event(instance, corev1.EventTypeWarning, "InvalidSpec", err.Error())
		return ctrl.Result{}, err
	}

	// Reconcile the HPA.
	if err := r.reconcileHPA(ctx, instance); err != nil {
		log.Error(err, "failed to reconcile HPA")
		return ctrl.Result{}, err
	}
	// Hash includes every resource kind's version annotation (from resourceKinds — the single
	// registry in resource_controller.go, see its doc comment) so the pod rolls when any of
	// them changes. resourceKinds is a fixed-order slice, not a map, so this hash is stable
	// across calls instead of depending on Go's randomized map iteration order.
	configHash := func() string {
		h := sha256.New()
		h.Write([]byte(publicURL + strconv.Itoa(int(port)) + instance.Spec.ResourcesConfigMap + databaseBlock + emailBlock + envSecretHash +
			instance.Spec.Config.TLS.MinVersion + instance.Spec.Config.JWT.PreferredKeyID +
			strings.Join(instance.Spec.Config.Passkey.ExtraAllowedOrigins, ",")))
		for _, k := range resourceKinds {
			h.Write([]byte(instance.Annotations["thunderid.io/"+k.configMapSuffix+"-version"]))
		}
		return hex.EncodeToString(h.Sum(nil))[:16]
	}()

	// Narrower than configHash: start.sh --bootstrap only ever creates the baseline default
	// resources (default OU, admin user, etc.) from spec.resourcesConfigMap's own raw content —
	// it never touches the per-entity ConfigMaps that make up the rest of configHash. Keying the
	// bootstrap Job on the full configHash instead would re-run bootstrap every time an unrelated
	// Role/Flow/User/etc. CR changes, even though nothing bootstrap actually reads changed.
	seedHash := func() string {
		h := sha256.New()
		h.Write([]byte(baseResourcesContent + databaseBlock))
		return hex.EncodeToString(h.Sum(nil))[:16]
	}()

	// Reconcile the security Secret.
	securitySecretName, err := r.reconcileSecuritySecret(ctx, instance)
	if err != nil {
		log.Error(err, "failed to reconcile security Secret")
		r.Recorder.Event(instance, corev1.EventTypeWarning, "SecuritySecretError", err.Error())
		return ctrl.Result{}, err
	}

	// Reconcile the bootstrap Job.
	if err := r.reconcileBootstrapJob(ctx, instance, seedHash, securitySecretName); err != nil {
		log.Error(err, "failed to reconcile bootstrap Job")
		r.Recorder.Event(instance, corev1.EventTypeWarning, "BootstrapJobError", err.Error())
		return ctrl.Result{}, err
	}

	// Reconcile the data PersistentVolumeClaim.
	if err := r.reconcileDataVolume(ctx, instance); err != nil {
		log.Error(err, "failed to reconcile data PersistentVolumeClaim")
		r.Recorder.Event(instance, corev1.EventTypeWarning, "DataVolumeError", err.Error())
		return ctrl.Result{}, err
	}

	// Reconcile the Deployment.
	if err := r.reconcileDeployment(ctx, instance, configHash, resourcesContent != "", securitySecretName); err != nil {
		log.Error(err, "failed to reconcile Deployment")
		return ctrl.Result{}, err
	}

	// Reconcile the Service.
	if err := r.reconcileService(ctx, instance); err != nil {
		log.Error(err, "failed to reconcile Service")
		return ctrl.Result{}, err
	}

	// Reconcile the Ingress.
	if err := r.reconcileIngress(ctx, instance); err != nil {
		log.Error(err, "failed to reconcile Ingress")
		return ctrl.Result{}, err
	}

	// Poll the serving pod's own container status, so a pod wedged on e.g. a missing env Secret
	// (CreateContainerConfigError, never a crash/termination) doesn't report Phase=Running forever.
	// See pod_health.go for what diagnosePod/diagnoseMissingSecret actually detect.
	phase := "Running"
	var healthReason, healthMessage string
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(instance.Namespace), client.MatchingLabels{appLabelKey: instance.Name}); err != nil {
		log.Error(err, "failed to list pods for health check")
	} else {
		for _, pod := range podList.Items {
			if unhealthy, issue := diagnosePod(&pod); unhealthy {
				phase = phaseError
				healthReason, healthMessage = issue.reason, issue.message
				if extra := diagnoseMissingSecret(ctx, r.Client, instance, issue); extra != "" {
					healthMessage = extra
				}
				break
			}
		}
	}

	// Re-fetch before status update — annotations may have been patched by sub-controllers.
	// Skip the write if status is already correct to avoid triggering a reconcile loop.
	statusErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := &appsv1alpha1.ThunderIDInstance{}
		if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
			return err
		}
		if latest.Status.URL == publicURL && latest.Status.Phase == phase {
			return nil
		}
		latest.Status.URL = publicURL
		latest.Status.Phase = phase
		return r.Status().Update(ctx, latest)
	})
	if statusErr != nil {
		log.Error(statusErr, "failed to update status")
		return ctrl.Result{}, statusErr
	}

	if phase == phaseError {
		if healthMessage != "" {
			r.Recorder.Event(instance, corev1.EventTypeWarning, healthReason, healthMessage)
		}
		log.Info("ThunderIDInstance pod unhealthy", "reason", healthReason, "message", healthMessage)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	log.Info("reconciled ThunderIDInstance", "url", publicURL)
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ThunderIDInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.ThunderIDInstance{}, builder.WithPredicates(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
		))).
		Owns(&appsv1.Deployment{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Service{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&networkingv1.Ingress{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.PersistentVolumeClaim{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		// Watches, not Owns: pods are owned by the Deployment's ReplicaSet, not directly by
		// ThunderIDInstance, so Owns() (which only follows one ownership hop) can't see them.
		// CreateFunc only — a brand-new serving pod object is exactly "the pod restarted,"
		// regardless of why (manual delete, crash-loop, rollout); routine status updates on an
		// existing pod fire UpdateFunc constantly and aren't a restart, so they're ignored here.
		// See mapServingPodToInstance for what this actually does with that signal.
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(r.mapServingPodToInstance),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					labels := e.Object.GetLabels()
					_, hasApp := labels[appLabelKey]
					return hasApp && labels[bootstrapRoleLabelKey] != bootstrapRoleLabelValue
				},
				UpdateFunc:  func(event.UpdateEvent) bool { return false },
				DeleteFunc:  func(event.DeleteEvent) bool { return false },
				GenericFunc: func(event.GenericEvent) bool { return false },
			})).
		// No GenerationChangedPredicate here deliberately: a Job's transition to Failed/Complete
		// only updates status, which never bumps generation. reconcileBootstrapJob needs to see
		// that transition to retry a failed bootstrap automatically.
		Owns(&batchv1.Job{}).
		// Not Owns(): the instance doesn't own its env Secret (an EnvironmentValues authors it, or a
		// human hand-writes it directly — see EnvironmentValues's doc comment), so there's no owner
		// reference to follow. Editing the Secret's content alone doesn't touch either the
		// EnvironmentValues or the ThunderIDInstance object — without this, resolveEnvSecretHash's new
		// value would only ever be picked up by some unrelated reconcile trigger, not promptly
		// when the Secret you actually changed changes.
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.instancesForEnvSecret)).
		// The label itself (not just the Secret's content) is what wires an EnvironmentValues into an
		// instance's envFrom — adding, removing, or repointing the thunderid.io/instance label
		// needs to reach the instance just as promptly as a content change does. Update events
		// are mapped for both the old and new object, so repointing the label from instance A to
		// B correctly reconciles both.
		Watches(&appsv1alpha1.EnvironmentValues{}, handler.EnqueueRequestsFromMapFunc(r.instancesForEnvironment)).
		Named("thunderidinstance").
		Complete(r)
}

// instancesForEnvironment maps an EnvironmentValues to the ThunderIDInstance named by its
// thunderid.io/instance label, if any.
func (r *ThunderIDInstanceReconciler) instancesForEnvironment(_ context.Context, obj client.Object) []reconcile.Request {
	instanceName, ok := obj.GetLabels()[appsv1alpha1.InstanceRefLabel]
	if !ok || instanceName == "" {
		return nil
	}
	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: instanceName, Namespace: obj.GetNamespace()}},
	}
}

// instancesForEnvSecret maps a Secret change to the ThunderIDInstance labeled on the EnvironmentValues
// of the same name — an EnvironmentValues's Secret is always same-named as it (see
// environment_controller.go), so this just re-Gets that EnvironmentValues and reuses
// instancesForEnvironment's label lookup. A hand-written Secret with no matching EnvironmentValues
// object maps to nothing: without an EnvironmentValues to carry the label, there's no instance to
// discover it from a bare Secret change alone (see EnvironmentValues's doc comment on adopting one).
func (r *ThunderIDInstanceReconciler) instancesForEnvSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	env := &appsv1alpha1.EnvironmentValues{}
	if err := r.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, env); err != nil {
		return nil
	}
	return r.instancesForEnvironment(ctx, env)
}

// mapServingPodToInstance handles a freshly (re)created serving pod: if this instance's
// bootstrap Job is currently sitting Failed (whether it gave up after bootstrapMaxAttempts or
// hit a bootstrapPermanentFailureReason), the pod coming back is treated as a real chance
// something changed, and the Job is deleted right here — not just requeued for evaluation. That
// makes the reconcile this returns see "Job not found" and take reconcileBootstrapJob's normal
// first-time-bootstrap path, starting a fresh attempt=1..bootstrapMaxAttempts cycle, with no
// separate "should I retry" state to track. A pod restart for a totally unrelated reason (OOM,
// node eviction) still only re-runs bootstrap's idempotent upserts — wasted work, not harmful —
// so this doesn't need to be more selective about why the pod restarted.
func (r *ThunderIDInstanceReconciler) mapServingPodToInstance(ctx context.Context, obj client.Object) []reconcile.Request {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil
	}
	// Read the owning instance's name off the pod's app label.
	instanceName, ok := pod.Labels["app"]
	if !ok {
		return nil
	}
	log := logf.FromContext(ctx)

	// Find and delete this instance's bootstrap Job, if it's currently failed.
	jobs := &batchv1.JobList{}
	if err := r.List(ctx, jobs, client.InNamespace(pod.Namespace),
		client.MatchingLabels{appLabelKey: instanceName, bootstrapRoleLabelKey: bootstrapRoleLabelValue}); err == nil {
		for i := range jobs.Items {
			if bootstrapJobFailed(&jobs.Items[i]) {
				log.Info("serving pod restarted; deleting failed bootstrap Job so bootstrap retries fresh",
					"pod", pod.Name, "job", jobs.Items[i].Name)
				if err := r.Delete(ctx, &jobs.Items[i], client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !errors.IsNotFound(err) {
					log.Error(err, "failed to delete failed bootstrap Job", "job", jobs.Items[i].Name)
				}
			}
		}
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: instanceName, Namespace: pod.Namespace}}}
}

// Markers substituted into deploymentConfigTemplate by reconcileConfigMap. serverPortMarker/
// publicURLMarker/tlsMinVersionMarker/jwtPreferredKeyIDMarker are plain strings.ReplaceAll
// targets; passkeyExtraOriginsMarkerLine/emailBlockMarkerLine are whole-line markers (the full
// line, comment included) swapped out via strings.Replace. The database: section has no marker
// at all — deployment-config.yaml carries the real sqlite content directly; see
// staticSQLiteDatabaseSection in database_config.go for how postgres overrides it.
const (
	serverPortMarker        = "__SERVER_PORT__"
	publicURLMarker         = "__PUBLIC_URL__"
	tlsMinVersionMarker     = "__TLS_MIN_VERSION__"
	jwtPreferredKeyIDMarker = "__JWT_PREFERRED_KEY_ID__"

	passkeyExtraOriginsMarkerLine = `    - "__PASSKEY_EXTRA_ORIGINS__" # replaced whole-line by reconcileConfigMap with 0+ additional "- \"origin\"" lines, or removed if spec.config.passkey.extraAllowedOrigins is unset`
)

// reconcileConfigMap returns resourcesContent (the full resources.yaml — base seed plus every
// sub-controller's appended resources, what actually gets mounted for the serving pod),
// baseResourcesContent (just the base seed, spec.resourcesConfigMap's raw content, before the
// entity-append loop — the bootstrap Job's seed hash is computed from this narrower value, since
// entity CRs like Role/Flow/User have nothing to do with what start.sh --bootstrap creates),
// databaseBlock, and emailBlock.
func (r *ThunderIDInstanceReconciler) reconcileConfigMap(
	ctx context.Context, instance *appsv1alpha1.ThunderIDInstance, publicURL string) (
	resourcesContent, baseResourcesContent, databaseBlock, emailBlock string, err error) {
	log := logf.FromContext(ctx)

	// Resolve the database config block.
	databaseBlock, _, err = r.resolveDatabaseConfig(ctx, instance)
	if err != nil {
		return "", "", "", "", err
	}
	// Resolve the email config block.
	emailBlock, err = r.resolveEmailConfig(ctx, instance)
	if err != nil {
		return "", "", "", "", err
	}

	// Resolve the remaining config defaults.
	port := instance.Spec.Config.Port
	if port == 0 {
		port = 8090
	}
	tlsMinVersion := instance.Spec.Config.TLS.MinVersion
	if tlsMinVersion == "" {
		tlsMinVersion = "1.3"
	}
	jwtPreferredKeyID := instance.Spec.Config.JWT.PreferredKeyID
	if jwtPreferredKeyID == "" {
		jwtPreferredKeyID = "default-key"
	}
	// Build the passkey extra-origins YAML lines.
	passkeyExtraOriginLines := make([]string, len(instance.Spec.Config.Passkey.ExtraAllowedOrigins))
	for i, o := range instance.Spec.Config.Passkey.ExtraAllowedOrigins {
		passkeyExtraOriginLines[i] = fmt.Sprintf("    - %q", o)
	}

	// Substitute the resolved values into the deployment.yaml template.
	configContent := deploymentConfigTemplate
	configContent = strings.ReplaceAll(configContent, serverPortMarker, strconv.Itoa(int(port)))
	configContent = strings.ReplaceAll(configContent, publicURLMarker, publicURL)
	configContent = strings.ReplaceAll(configContent, tlsMinVersionMarker, tlsMinVersion)
	configContent = strings.ReplaceAll(configContent, jwtPreferredKeyIDMarker, jwtPreferredKeyID)
	configContent = strings.Replace(configContent, passkeyExtraOriginsMarkerLine,
		strings.Join(passkeyExtraOriginLines, "\n"), 1)
	configContent = strings.Replace(configContent, emailBlockMarkerLine, emailBlock, 1)
	if databaseBlock != "" {
		// postgres: swap out the template's static sqlite section for the dynamic one. sqlite
		// (databaseBlock == "") needs no substitution — the template already has the right content.
		configContent = strings.Replace(configContent, staticSQLiteDatabaseSection, databaseBlock, 1)
	}

	// Load base resources from external ConfigMap.
	if instance.Spec.ResourcesConfigMap != "" {
		externalCM := &corev1.ConfigMap{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      instance.Spec.ResourcesConfigMap,
			Namespace: instance.Namespace,
		}, externalCM); err != nil {
			return "", "", "", "", err
		}
		resourcesContent = externalCM.Data["resources.yaml"]
	}
	baseResourcesContent = resourcesContent

	// Append custom resources from each resource kind's ConfigMap into the same resources.yaml
	// so ThunderID loads them in a single pass on startup — resourceKinds (resource_controller.go)
	// is the single registry of which ConfigMap/key each resource_type lives in, and its order.
	for _, k := range resourceKinds {
		appended, err := r.appendResourceConfigMap(ctx, instance, "-"+k.configMapSuffix, k.fileKey)
		if err != nil {
			return "", "", "", "", err
		}
		resourcesContent += appended
	}

	// Assemble the ConfigMap's data.
	// First add deployment config.
	data := map[string]string{
		deploymentYAMLKey: configContent,
	}
	// Then add resources.yaml if non-empty
	if resourcesContent != "" {
		data["resources.yaml"] = resourcesContent
	}

	// Fetch the existing config ConfigMap, or create it if absent.
	cm := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: instance.Name + "-config", Namespace: instance.Namespace}, cm)
	if errors.IsNotFound(err) {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.Name + "-config",
				Namespace: instance.Namespace,
			},
			Data: data,
		}
		_ = ctrl.SetControllerReference(instance, cm, r.Scheme)
		log.Info("creating config ConfigMap", "name", cm.Name)
		return resourcesContent, baseResourcesContent, databaseBlock, emailBlock, r.Create(ctx, cm)
	}
	if err != nil {
		return "", "", "", "", err
	}
	// Skip update if content is unchanged to avoid triggering unnecessary reconciles.
	if cm.Data[deploymentYAMLKey] == data[deploymentYAMLKey] && cm.Data["resources.yaml"] == data["resources.yaml"] {
		log.V(1).Info("config ConfigMap unchanged, skipping update", "name", cm.Name)
		return resourcesContent, baseResourcesContent, databaseBlock, emailBlock, nil
	}
	log.Info("config ConfigMap content changed, updating", "name", cm.Name)
	updateErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := &corev1.ConfigMap{}
		if err := r.Get(ctx, types.NamespacedName{Name: instance.Name + "-config", Namespace: instance.Namespace}, latest); err != nil {
			return err
		}
		latest.Data = data
		return r.Update(ctx, latest)
	})
	return resourcesContent, baseResourcesContent, databaseBlock, emailBlock, updateErr
}

// appendResourceConfigMap reads one sub-controller's per-resource_type ConfigMap (named
// instance.Name+suffix) and returns its content at key, prefixed with a leading newline ready to
// concatenate onto resources.yaml — or "" if that ConfigMap doesn't exist yet (no resource of that
// type has been created) or its key is empty.
func (r *ThunderIDInstanceReconciler) appendResourceConfigMap(
	ctx context.Context, instance *appsv1alpha1.ThunderIDInstance, suffix, key string) (string, error) {
	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      instance.Name + suffix,
		Namespace: instance.Namespace,
	}, cm); err != nil {
		if errors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	if content := cm.Data[key]; content != "" {
		return "\n" + content, nil
	}
	return "", nil
}

// reconcileDataVolume ensures the PersistentVolumeClaim backing setup.sh's once-only marker
// (see reconcileDeployment) exists when spec.dataVolumeSize is set. The claim's storage
// request is not reconciled after creation — resizing an existing instance's volume is left
// to the user (edit the PVC directly, subject to what the StorageClass allows).
func (r *ThunderIDInstanceReconciler) reconcileDataVolume(
	ctx context.Context, instance *appsv1alpha1.ThunderIDInstance) error {
	// Nothing to persist locally when postgres holds the state — see reconcileDeployment.
	if instance.Spec.DataVolumeSize == "" || usingPostgres(instance) {
		return nil
	}
	// Already exists — nothing to do.
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name + "-data", Namespace: instance.Namespace}, pvc)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}
	// Parse the requested storage size and create the PVC.
	qty, err := resource.ParseQuantity(instance.Spec.DataVolumeSize)
	if err != nil {
		return fmt.Errorf("invalid spec.dataVolumeSize %q: %w", instance.Spec.DataVolumeSize, err)
	}
	pvc = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-data",
			Namespace: instance.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: qty},
			},
		},
	}
	_ = ctrl.SetControllerReference(instance, pvc, r.Scheme)
	return r.Create(ctx, pvc)
}

// bootstrapAttemptLabel/bootstrapMaxAttempts track and cap reconcileBootstrapJob's automatic
// retry of a failed bootstrap Job (see its own doc comment). bootstrapRoleLabelKey/Value identify
// a Job (and its pod) as this bootstrap Job rather than the serving Deployment's own pod — used
// both to find stale Jobs to delete and, in mapServingPodToInstance, to exclude bootstrap pods
// from the "serving pod restarted" signal. bootstrapContainerName names the single container in
// that pod, used to fetch its logs in bootstrapPermanentFailureReason.
const (
	bootstrapAttemptLabel = "thunderid.io/bootstrap-attempt"
	bootstrapMaxAttempts  = 3

	bootstrapRoleLabelKey   = "thunderid.io/role"
	bootstrapRoleLabelValue = "bootstrap"
	bootstrapContainerName  = "bootstrap"
)

// Volume/VolumeMount/container names shared across reconcileDeployment and createBootstrapJob -
// both build near-identical mounts for the same underlying Secrets/ConfigMap. The database
// scopes' own Volume names come from databaseVolumeName (database_config.go) instead — one per
// scope, not a single fixed name here.
const (
	volumeNameConfig   = "config"
	volumeNameSecurity = "security"
	volumeNameSecrets  = "secrets"
	volumeNameEmail    = "email"
	volumeNameData     = "data"

	deploymentYAMLKey = "deployment.yaml"
	secretsMountPath  = "/opt/thunderid/config/secrets"

	shBinPath   = "/bin/sh"
	bashBinPath = "/bin/bash"

	adminUsernameEnvKey = "ADMIN_USERNAME"
	adminPasswordEnvKey = "ADMIN_PASSWORD"

	// defaultAdminUsername is the bootstrap admin username ThunderID's own setup.sh creates when
	// ADMIN_USERNAME isn't overridden. Not a secret, so it stays a literal here - the password
	// half comes from the security Secret's securityKeyAdminPassword instead (see
	// reconcileSecuritySecret), never a hardcoded literal.
	defaultAdminUsername = "admin"
)

// reconcileBootstrapJob imports resources.yaml into the postgres database via a one-shot Job,
// decoupled from the serving pod's own lifecycle (see the postgres branch of reconcileDeployment's
// startCmd). The Job's name is suffixed with seedHash — a narrower hash than the Deployment's own
// configHash, covering only spec.resourcesConfigMap's raw content and the DB connection target,
// since those are the only things start.sh --bootstrap actually reads. It deliberately excludes
// the per-entity ConfigMaps (Role/Flow/User/etc.) that make up the rest of configHash: bootstrap
// never touches those, so a Role/Flow/User CR changing shouldn't re-run it. An ordinary pod
// restart, HPA scale-out, or an unrelated entity CR change all reconcile with the same seedHash,
// find the Job already exists, and do nothing — avoiding the redundant (and, under concurrent
// scale-out, potentially racy) DB writes that running bootstrap from every pod's own boot command
// would cause. A genuine change to the base seed or DB target produces a new hash and thus a new
// Job — at which point any previous bootstrap Job(s) for this instance are deleted, since they're
// no longer the "did we already import the current config" marker.
//
// Deliberately no TTLSecondsAfterFinished here: relying on Kubernetes' TTL controller to garbage
// collect a finished Job would make the "does a Job already exist for this hash" check below only
// as durable as the TTL window — once it expired, the very next routine reconcile (e.g. any of the
// several 30s health-check loops already running elsewhere in this codebase) would see "not found"
// and silently re-run the whole bootstrap import, indefinitely, for a config that never changed.
// That defeats the entire point of this Job existing. Cleanup is instead handled explicitly, below,
// exactly when it's actually safe to do — the moment a new hash makes the old Job obsolete.
//
// A Job that reaches the Failed condition (its own BackoffLimit exhausted — e.g. the DB was
// temporarily unreachable) is retried automatically up to bootstrapMaxAttempts times: the attempt
// count travels on the replacement Job's own bootstrapAttemptLabel, so it survives across
// reconciles without needing separate state on the ThunderIDInstance. Past the cap, the Job is
// left as-is (Failed, inspectable via its logs) and reconcile stops touching it — avoiding an
// endless recreate loop against a genuinely broken config (e.g. a permanently wrong password).
func (r *ThunderIDInstanceReconciler) reconcileBootstrapJob(
	ctx context.Context, instance *appsv1alpha1.ThunderIDInstance, seedHash string,
	securitySecretName string) error {
	if !usingPostgres(instance) {
		return nil
	}
	log := logf.FromContext(ctx)

	jobName := fmt.Sprintf("%s-bootstrap-%s", instance.Name, seedHash)
	existing := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: instance.Namespace}, existing)
	if err == nil {
		if !bootstrapJobFailed(existing) {
			// Still running, or succeeded — nothing to do.
			return nil
		}
		if reason := r.bootstrapPermanentFailureReason(ctx, existing); reason != "" {
			// A misconfiguration (wrong password, bad host, wrong database/role name) — retrying
			// won't fix this, only burns time against a config that will never work. Stop
			// immediately regardless of attempt count and point at the actual cause.
			log.Info("bootstrap Job failure looks permanent, will not retry", "job", jobName, "reason", reason)
			r.Recorder.Eventf(instance, corev1.EventTypeWarning, "BootstrapJobFailed",
				"bootstrap Job %s failed and will not be retried automatically: %s — "+
					"fix spec.config.database (or its Secret), then delete the Job to retry", jobName, reason)
			return nil
		}
		attempt := bootstrapAttempt(existing)
		if attempt >= bootstrapMaxAttempts {
			log.Info("bootstrap Job reached its retry limit, giving up", "job", jobName, "attempts", attempt)
			r.Recorder.Eventf(instance, corev1.EventTypeWarning, "BootstrapJobFailed",
				"bootstrap Job %s has failed %d time(s) and reached the retry limit; "+
					"inspect its logs, then delete it to retry", jobName, attempt)
			return nil
		}
		log.Info("bootstrap Job failed, retrying", "job", jobName, "nextAttempt", attempt+1, "maxAttempts", bootstrapMaxAttempts)
		if err := r.Delete(ctx, existing, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !errors.IsNotFound(err) {
			return err
		}
		return r.createBootstrapJob(ctx, instance, jobName, securitySecretName, attempt+1)
	}
	if !errors.IsNotFound(err) {
		return err
	}

	// The config actually changed (or this is the first-ever bootstrap) — any bootstrap Job(s)
	// left over from a previous hash are now stale. Delete them before creating the new one, so
	// exactly one exists per instance at a time instead of accumulating forever.
	staleJobs := &batchv1.JobList{}
	if err := r.List(ctx, staleJobs, client.InNamespace(instance.Namespace),
		client.MatchingLabels{appLabelKey: instance.Name, bootstrapRoleLabelKey: bootstrapRoleLabelValue}); err != nil {
		return err
	}
	if len(staleJobs.Items) > 0 {
		log.Info("resources/database config changed, deleting stale bootstrap Job(s)", "count", len(staleJobs.Items))
	}
	for _, j := range staleJobs.Items {
		if err := r.Delete(ctx, &j, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	log.Info("creating bootstrap Job", "job", jobName)
	return r.createBootstrapJob(ctx, instance, jobName, securitySecretName, 1)
}

// bootstrapJobFailed reports whether job has permanently failed (its BackoffLimit exhausted),
// as opposed to merely running or having succeeded.
func bootstrapJobFailed(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// bootstrapAttempt reads how many attempts job itself represents, defaulting to 1 if the
// label is missing or unparseable.
func bootstrapAttempt(job *batchv1.Job) int {
	n, err := strconv.Atoi(job.Labels[bootstrapAttemptLabel])
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// bootstrapPermanentErrorPatterns maps known-unfixable-by-retrying failure signatures (observed
// verbatim from the pq driver and Go's net package — e.g. "pq: password authentication failed
// for user 'x'", "dial tcp: lookup x: no such host") to a human-readable reason. Anything not
// matching one of these is treated as possibly transient (cold-start timeout, network blip) and
// left to the normal attempt-count retry instead.
var bootstrapPermanentErrorPatterns = []struct {
	pattern *regexp.Regexp
	reason  string
}{
	{regexp.MustCompile(`password authentication failed`), "wrong spec.config.database.<scope>.postgres.username or the Secret's password"},
	{regexp.MustCompile(`no such host`), "spec.config.database.<scope>.postgres.hostname does not resolve"},
	{regexp.MustCompile(`database "[^"]*" does not exist`), "spec.config.database.<scope>.postgres.name does not exist on the server"},
	{regexp.MustCompile(`role "[^"]*" does not exist`), "spec.config.database.<scope>.postgres.username does not exist on the server"},
	{regexp.MustCompile(`connection refused`), "spec.config.database.<scope>.postgres.hostname/port is unreachable — nothing is listening there"},
}

// bootstrapPermanentFailureReason fetches the failed bootstrap pod's logs and checks them
// against bootstrapPermanentErrorPatterns. Returns "" (meaning: might be transient, worth
// retrying) whenever the pod, its logs, or a matching pattern can't be found — this only ever
// short-circuits a retry when it has positive evidence the failure is a real misconfiguration.
//
// Looks the pod up via KubeClient (a direct API read) rather than r.List (the manager's cached
// client): against a fast-failing DB (e.g. local Postgres rejecting bad auth in milliseconds,
// versus a remote one's multi-second round trip), the pod can fail and this function can run
// before the watch-fed cache has caught up, making a cached lookup see zero pods and silently
// fall back to "unknown, maybe transient" — masking a real, immediately-diagnosable error.
func (r *ThunderIDInstanceReconciler) bootstrapPermanentFailureReason(ctx context.Context, job *batchv1.Job) string {
	log := logf.FromContext(ctx)
	// Find the bootstrap Job's pod. The Job itself has no direct pointer to its pod, only label
	// matching does; the attempt label pins this to the pod from *this* attempt, not a stale one
	// left behind by an earlier retry of the same Job.
	selector := fmt.Sprintf("app=%s,thunderid.io/role=bootstrap,%s=%s",
		job.Labels["app"], bootstrapAttemptLabel, job.Labels[bootstrapAttemptLabel])
	pods, err := r.KubeClient.CoreV1().Pods(job.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil || len(pods.Items) == 0 {
		log.V(1).Info("could not find the bootstrap Job's pod to check for a permanent failure reason, treating as possibly transient", "job", job.Name, "error", err)
		return ""
	}

	// Fetch and read its logs. GetLogs only builds the request; Stream is what actually calls the
	// API server and hands back a readable stream, which ReadAll then drains into memory below.
	req := r.KubeClient.CoreV1().Pods(job.Namespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{Container: bootstrapContainerName})
	rc, err := req.Stream(ctx)
	if err != nil {
		log.V(1).Info("could not fetch bootstrap pod logs; retrying", "job", job.Name, "pod", pods.Items[0].Name, "error", err)
		return ""
	}
	defer func() { _ = rc.Close() }()
	// Read the logs stream into []byte so we can run regexes against it. This is safe because the bootstrap pod is a one-shot Job that runs setup.sh and exits, so its logs are bounded in size (a few KB at most).
	data, err := io.ReadAll(rc)
	if err != nil {
		log.V(1).Info("could not read bootstrap pod logs; retrying", "job", job.Name, "pod", pods.Items[0].Name, "error", err)
		return ""
	}

	// Check the logs against known permanent-failure patterns.
	for _, p := range bootstrapPermanentErrorPatterns {
		if p.pattern.Match(data) {
			return p.reason
		}
	}
	log.V(1).Info("no known permanent-failure pattern matched; could be transient; retrying", "job", job.Name, "pod", pods.Items[0].Name)
	return ""
}

// createBootstrapJob builds and creates the one-shot Job (see reconcileBootstrapJob, its only
// caller) that runs setup.sh against postgres for the given jobName/attempt, mounting the same
// config/security/database (and, if enabled, email) material the serving Deployment mounts.
func (r *ThunderIDInstanceReconciler) createBootstrapJob(
	ctx context.Context, instance *appsv1alpha1.ThunderIDInstance, jobName string,
	securitySecretName string, attempt int) error {
	// This Job  itself is already the "run exactly once per resource-hash-change" gate, so setup.sh here
	// only ever runs when reconcileBootstrapJob decided a (re)bootstrap is actually needed, never
	// on an ordinary pod restart or HPA scale-out of the serving Deployment.
	//
	// setup.sh's own cert/key/direct_auth_secret generation is a no-op here: certs are pre-supplied
	// (below, same as the serving container) so generate_x509_cert finds them already present, and
	// direct_auth_secret is pre-seeded into a writable location by the init container below so
	// configure_direct_auth_secret reuses the existing value instead of generating a new one. Its
	// unconditional "chmod 600" on that file (even on the reuse path) is exactly why it can't be
	// mounted straight from the security Secret like the certs are — Secret volumes mount read-only,
	// and that chmod would fail under setup.sh's "set -e" and abort the whole script.
	//
	// setup.sh finishes by calling "start.sh --bootstrap" itself, which never forwards a
	// positional resources.yaml argument (only BOOTSTRAP_EXTRA_ARGS) — so this Job only ever
	// imports spec.resourcesConfigMap's base seed into the database. Declarative resources reach
	// ThunderID via the serving pod's own resources.yaml mount instead (see reconcileDeployment),
	// not this Job.
	bootstrapCmd := "./setup.sh"

	// Build the volume mounts.
	volumeMounts := []corev1.VolumeMount{
		{Name: volumeNameConfig, MountPath: "/opt/thunderid/deployment.yaml", SubPath: deploymentYAMLKey},
	}
	// Same cert mounts as the serving container's postgres branch (reconcileDeployment): certs are
	// always pre-supplied read-only. direct_auth_secret is handled separately below — it needs a
	// writable location (see the bootstrapCmd comment above), unlike these.
	for _, f := range securityCertMountKeys {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name: volumeNameSecurity, MountPath: "/opt/thunderid/config/certs/" + f, SubPath: f,
		})
	}
	// Add the volume mount for the writable secrets directory.
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      volumeNameSecrets,
		MountPath: secretsMountPath,
	})
	// Add one volume mount per postgres scope's password (createBootstrapJob only ever runs when
	// usingPostgres(instance) is true — see reconcileBootstrapJob — so every scope has one).
	for _, ss := range collectDatabaseScopeSecrets(instance) {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      databaseVolumeName(ss.scope),
			MountPath: "/opt/thunderid/" + databasePasswordMountPath(ss.scope),
			SubPath:   databaseSecretKeyPassword,
			ReadOnly:  true,
		})
	}
	// Add the volume mount for the email password secret, if email auth is enabled.
	if instance.Spec.Config.Email != nil && instance.Spec.Config.Email.SMTP.EnableAuthentication {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeNameEmail,
			MountPath: "/opt/thunderid/" + emailPasswordMountPath,
			SubPath:   databaseSecretKeyPassword,
			ReadOnly:  true,
		})
	}

	// Construct the list of volume sources to mount into the Job's pod.
	defaultMode := int32(0644)
	volumes := []corev1.Volume{
		{
			Name: volumeNameConfig,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: instance.Name + "-config"},
					DefaultMode:          &defaultMode,
				},
			},
		},
		{
			Name: volumeNameSecurity,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: securitySecretName, DefaultMode: &defaultMode},
			},
		},
		{
			// Writable landing spot for direct_auth_secret, seeded from the read-only "security"
			// Secret by the seed-direct-auth-secret init container below. emptyDir is enough here
			// (unlike the serving Deployment's PVC-backed equivalent): this Job runs once and exits,
			// so there's nothing that needs to survive beyond its own pod's lifetime.
			Name:         volumeNameSecrets,
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
	}
	for _, ss := range collectDatabaseScopeSecrets(instance) {
		volumes = append(volumes, corev1.Volume{
			Name: databaseVolumeName(ss.scope),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  ss.ref.Name,
					Items:       []corev1.KeyToPath{{Key: ss.ref.Key, Path: databaseSecretKeyPassword}},
					DefaultMode: &defaultMode,
				},
			},
		})
	}
	if instance.Spec.Config.Email != nil && instance.Spec.Config.Email.SMTP.EnableAuthentication {
		volumes = append(volumes, corev1.Volume{
			Name: volumeNameEmail,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  instance.Spec.Config.Email.SMTP.SecretRef.Name,
					Items:       []corev1.KeyToPath{{Key: instance.Spec.Config.Email.SMTP.SecretRef.Key, Path: emailSecretKeyPassword}},
					DefaultMode: &defaultMode,
				},
			},
		})
	}

	// Same seeding trick as the serving Deployment's PVC path (reconcileDeployment): copy
	// direct_auth_secret out of the read-only security Secret into the writable "secrets" emptyDir
	// so setup.sh's unconditional "chmod 600" on it succeeds instead of failing under set -e.
	// RunAsUser must match the "bootstrap" container's own uid (10001, from the thunderid image's
	// default): busybox defaults to root, so without this the file gets created root-owned, and
	// chmod 600 then locks the "bootstrap" container (uid 10001) out with "permission denied".
	thunderidUID := int64(10001)
	seedDirectAuthSecretCmd := fmt.Sprintf(
		"cp /run/security-seed/%[1]s /opt/thunderid/config/secrets/%[1]s && chmod 600 /opt/thunderid/config/secrets/%[1]s",
		securityKeyDirectAuth)
	initContainers := []corev1.Container{
		{
			Name:            "seed-direct-auth-secret",
			Image:           initContainerImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{shBinPath, "-c", seedDirectAuthSecretCmd},
			SecurityContext: &corev1.SecurityContext{RunAsUser: &thunderidUID},
			VolumeMounts: []corev1.VolumeMount{
				{Name: volumeNameSecurity, MountPath: "/run/security-seed", ReadOnly: true},
				{Name: volumeNameSecrets, MountPath: secretsMountPath},
			},
		},
	}

	// Labels every object in this Job's pod carries.
	labels := map[string]string{
		appLabelKey:           instance.Name,
		bootstrapRoleLabelKey: bootstrapRoleLabelValue,
		bootstrapAttemptLabel: strconv.Itoa(attempt),
	}

	// BackoffLimit 0: reconcileBootstrapJob's own attempt-count retry (createBootstrapJob's
	// caller) already handles recreation across attempts, so a Job-internal retry loop on top
	// would multiply total pod attempts before giving up — needlessly slow. RestartPolicy: Never
	// pairs with this deliberately, not OnFailure: OnFailure restarts the container in place, and
	// the pod-replacement that triggers cleans up the old failed pod almost immediately — often
	// before bootstrapPermanentFailureReason gets a chance to read its logs. With BackoffLimit: 0
	// there's only ever one pod per Job anyway, so there's nothing to restart in place — Never
	// avoids the replacement-driven cleanup and leaves that one failed pod inspectable.
	backoffLimit := int32(0)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyNever,
					InitContainers: initContainers,
					Containers: []corev1.Container{
						{
							Name:    bootstrapContainerName,
							Image:   instance.Spec.Image,
							Command: []string{bashBinPath, "-c", "cd /opt/thunderid && " + bootstrapCmd},
							Env: []corev1.EnvVar{
								{Name: adminUsernameEnvKey, Value: defaultAdminUsername},
								{Name: adminPasswordEnvKey, ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: securitySecretName},
										Key:                  securityKeyAdminPassword,
									},
								}},
							},
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
	_ = ctrl.SetControllerReference(instance, job, r.Scheme)
	return r.Create(ctx, job)
}

// thunderidStartupProbe gives the container room to finish booting before liveness/readiness
// start enforcing: while a startupProbe is configured, kubelet runs only this probe and holds off
// on liveness/readiness until it succeeds once. Boot time scales with how much resources.yaml
// there is to process on startup (declarative resources), which a fixed short liveness delay alone
// can't account for — with liveness's InitialDelaySeconds=1 and just 3 more 3s periods (~10s total)
// as the only grace period, a pod with a large enough resources.yaml gets killed by the liveness
// probe before it ever finishes starting, and never gets a chance to come up at all. FailureThreshold
// of 60 at PeriodSeconds=2 gives ~2 minutes of startup grace before that liveness enforcement begins.
func thunderidStartupProbe(port int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health/liveness",
				Port:   intstr.FromInt32(port),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 1,
		PeriodSeconds:       2,
		FailureThreshold:    60,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
	}
}

func thunderidLivenessProbe(port int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health/liveness",
				Port:   intstr.FromInt32(port),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 1,
		PeriodSeconds:       3,
		FailureThreshold:    3,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
	}
}

func thunderidReadinessProbe(port int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health/readiness",
				Port:   intstr.FromInt32(port),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 1,
		PeriodSeconds:       2,
		FailureThreshold:    6,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
	}
}

// reconcileDeployment creates or updates the serving Deployment: its start command (postgres vs.
// SQLite/PVC, with or without declarative resources), init containers (permission-fixing,
// direct_auth_secret seeding), volumes/mounts for config/security/database/email/data, probes, and
// the configHash annotation that triggers a rolling restart when any of that changes.
func (r *ThunderIDInstanceReconciler) reconcileDeployment(
	ctx context.Context, instance *appsv1alpha1.ThunderIDInstance, configHash string, hasResources bool,
	securitySecretName string) error {
	log := logf.FromContext(ctx)
	replicas := instance.Spec.Replicas
	configMapName := instance.Name + "-config"
	// Resolve the port.
	port := instance.Spec.Config.Port
	if port == 0 {
		port = 8090
	}

	// With postgres, all state lives in the external database — there are no local SQLite files
	// to persist, so spec.dataVolumeSize (and the once-only setup.sh gating below) is ignored.
	hasDataVolume := instance.Spec.DataVolumeSize != "" && !usingPostgres(instance)

	// Every key in the Secret named by whichever EnvironmentValues is labeled for this instance (see
	// findEnvironment) becomes an env var on the container — resolving whatever {{.VAR}} tokens a
	// ThunderIDResource's spec holds is entirely ThunderID's own job at boot, not the operator's. Reconcile
	// already called findEnvironment once via resolveEnvSecretHash and would have returned early
	// on an ambiguous label match, so any error here can't actually happen in practice; it's
	// still handled rather than ignored since reconcileDeployment already returns error.
	var envFrom []corev1.EnvFromSource
	if env, err := findEnvironment(ctx, r.Client, instance); err != nil {
		return err
	} else if env != nil {
		envFrom = []corev1.EnvFromSource{
			{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: env.Name}}},
		}
	}

	// The thunderid image runs as a non-root user (uid/gid 10001, see the image's Dockerfile).
	// A freshly provisioned PVC is mounted root-owned, so without fsGroup the container can't
	// write its "setup ran once" marker to it — setup.sh succeeds but the following touch fails
	// with "Permission denied", and the container exits non-zero and crash-loops. ConfigMap/Secret/
	// emptyDir volumes don't need this: kubelet mounts those world-writable by default.
	var podSecurityContext *corev1.PodSecurityContext
	if hasDataVolume {
		thunderidGID := int64(10001)
		podSecurityContext = &corev1.PodSecurityContext{FSGroup: &thunderidGID}
	}

	// With a persistent data volume, repository/database survives restarts, so setup.sh's cert
	// generation, secret configuration, and (critically) its DB-bootstrap step only need to run
	// once: gate it behind a marker file written on that same volume.
	//
	// ThunderID's SQLite schema is created at image-build time (via dbscripts/*/sqlite.sql),
	// not at runtime — the image's own database/*.db files already have schema and seed data
	// (default OU, base user types, etc). The PVC starts empty, so on first boot only, seed it
	// from those files before setup.sh connects; otherwise setup.sh's bootstrap step fails with
	// "no such table" against a schema-less database.
	var startCmd string
	if hasDataVolume {
		setupCmd := "[ -f repository/database/configdb.db ] || cp /opt/thunderid/database/*.db repository/database/; " +
			"[ -f repository/database/.setup-complete ] || { ./setup.sh && touch repository/database/.setup-complete; }"
		startCmd = fmt.Sprintf("cd /opt/thunderid && %s && exec ./start.sh", setupCmd)
		if hasResources {
			startCmd = fmt.Sprintf("cd /opt/thunderid && %s && exec ./start.sh /opt/thunderid/resources.yaml", setupCmd)
		}
	} else {
		// No PVC to persist onto, and postgres already holds all state externally, so there's
		// nothing setup.sh's once-only gating above would buy us. Importing resources.yaml into
		// the DB is handled by a separate, hash-gated bootstrap Job instead (see
		// reconcileBootstrapJob) — one that only runs when the resources actually changed, not on
		// every pod start. That keeps this container's own boot path a plain, serve-only
		// start.sh: no repeated DB-import cost on ordinary restarts or HPA scale-out, and (unlike
		// the PVC path above, where bootstrap only ever runs on the very first boot) CR changes
		// still reach the database — just via the Job picking up the new hash, not via this
		// container re-running anything.
		serveCmd := "./start.sh"
		if hasResources {
			serveCmd += " /opt/thunderid/resources.yaml"
		}
		startCmd = fmt.Sprintf("cd /opt/thunderid && exec %s", serveCmd)
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      volumeNameConfig,
			MountPath: "/opt/thunderid/deployment.yaml",
			SubPath:   deploymentYAMLKey,
		},
	}
	if hasResources {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeNameConfig,
			MountPath: "/opt/thunderid/resources.yaml",
			SubPath:   "resources.yaml",
		})
	}

	// Mounted individually (not as a whole directory) onto /opt/thunderid/config/{certs,secrets}
	// so we don't clobber the image's other baked-in files under /opt/thunderid/config (e.g.
	// default.json, resources/templates/*). setup.sh only (re)generates a cert/key pair when it
	// finds the file missing, so pre-supplying all of them here — stable across reconciles, see
	// reconcileSecuritySecret — keeps the JWT signing key (and TLS/encryption keys) identical
	// across pod restarts and across replicas.
	for _, f := range securityCertMountKeys {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeNameSecurity,
			MountPath: "/opt/thunderid/config/certs/" + f,
			SubPath:   f,
		})
	}
	// direct_auth_secret is handled separately from the cert/key files above. With a PVC, setup.sh
	// still runs (see startCmd above) and unconditionally chmod's/rewrites this file every time it
	// runs — Secret-backed volumes are always mounted read-only, so it needs a writable location,
	// seeded by an init container. Without a PVC, startCmd runs plain start.sh (see the else
	// branch below), which only ever reads this file (via deployment.yaml's file:// reference) —
	// so it can be mounted directly, read-only, right alongside the certs below.
	if hasDataVolume {
		// Backed by the PVC (subPath keeps it a distinct directory from the SQLite files), not an
		// emptyDir: this makes seeding a true one-time cost, since the file persists across
		// restarts instead of vanishing with the pod like an emptyDir would.
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeNameData,
			MountPath: secretsMountPath,
			SubPath:   "secrets",
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeNameData,
			MountPath: "/opt/thunderid/repository/database",
		})
	} else {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeNameSecurity,
			MountPath: "/opt/thunderid/config/secrets/" + securityKeyDirectAuth,
			SubPath:   securityKeyDirectAuth,
			ReadOnly:  true,
		})
	}
	// One mount per postgres scope that actually has a passwordRef — not gated on usingPostgres(instance)
	// (all four scopes postgres): a mixed sqlite/postgres config needs exactly the postgres
	// scopes' passwords mounted, no more, no less. Nested under the config/secrets directory
	// mount above — Kubernetes bind-mounts each volumeMount independently, so a single-file mount
	// here doesn't conflict with the directory mount it sits inside. Only the password is
	// Secret-sourced: the rest of the postgres connection details are inlined directly into
	// deployment.yaml (see resolveDatabaseConfig), matching how ThunderID resolves file:// values
	// for its other secrets (direct_auth_secret, crypto.key).
	for _, ss := range collectDatabaseScopeSecrets(instance) {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      databaseVolumeName(ss.scope),
			MountPath: "/opt/thunderid/" + databasePasswordMountPath(ss.scope),
			SubPath:   databaseSecretKeyPassword,
			ReadOnly:  true,
		})
	}
	if instance.Spec.Config.Email != nil && instance.Spec.Config.Email.SMTP.EnableAuthentication {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeNameEmail,
			MountPath: "/opt/thunderid/" + emailPasswordMountPath,
			SubPath:   databaseSecretKeyPassword,
			ReadOnly:  true,
		})
	}

	defaultMode := int32(0644)
	volumes := []corev1.Volume{
		{
			Name: volumeNameConfig,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
					DefaultMode:          &defaultMode,
				},
			},
		},
		{
			Name: volumeNameSecurity,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  securitySecretName,
					DefaultMode: &defaultMode,
				},
			},
		},
	}
	if hasDataVolume {
		volumes = append(volumes, corev1.Volume{
			Name: volumeNameData,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: instance.Name + "-data",
				},
			},
		})
	}
	for _, ss := range collectDatabaseScopeSecrets(instance) {
		volumes = append(volumes, corev1.Volume{
			Name: databaseVolumeName(ss.scope),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  ss.ref.Name,
					Items:       []corev1.KeyToPath{{Key: ss.ref.Key, Path: databaseSecretKeyPassword}},
					DefaultMode: &defaultMode,
				},
			},
		})
	}
	if instance.Spec.Config.Email != nil && instance.Spec.Config.Email.SMTP.EnableAuthentication {
		volumes = append(volumes, corev1.Volume{
			Name: volumeNameEmail,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  instance.Spec.Config.Email.SMTP.SecretRef.Name,
					Items:       []corev1.KeyToPath{{Key: instance.Spec.Config.Email.SMTP.SecretRef.Key, Path: emailSecretKeyPassword}},
					DefaultMode: &defaultMode,
				},
			},
		})
	}

	thunderidUID := int64(10001)
	var initContainers []corev1.Container
	if hasDataVolume {
		// fsGroup (set below on the pod's SecurityContext) isn't honored by every volume
		// backend — notably plain hostPath PVs, which is what minikube's default storage
		// class provisions. Chown explicitly, as root, so the non-root "thunderid" container
		// can write its setup-complete marker regardless of what's backing the PVC.
		//
		// The recursive chown only needs to happen once per PVC: everything the "thunderid"
		// container (uid 10001) creates afterwards is already owned by 10001. On every restart
		// after the first, skip the walk — which only gets slower as WAL/journal files
		// accumulate — by checking the top-level dir's owner first.
		//
		// This must run before seed-direct-auth-secret below: on a fresh PVC the top-level dir
		// is root-owned, and seed-direct-auth-secret (running as uid 10001, not root) can't
		// create a file under a directory it doesn't yet own.
		root := int64(0)
		initContainers = append(initContainers, corev1.Container{
			Name:            "fix-data-volume-permissions",
			Image:           initContainerImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command: []string{shBinPath, "-c",
				// mkdir+chown here is a cheap, non-recursive, always-run step — kept separate
				// from the owner check below so it isn't skipped on a PVC whose top-level dir
				// already shows owner 10001 but doesn't yet have this secrets subdirectory.
				`mkdir -p /opt/thunderid/repository/database/secrets && ` +
					`chown 10001:10001 /opt/thunderid/repository/database/secrets; ` +
					`owner=$(stat -c %u /opt/thunderid/repository/database 2>/dev/null || echo -1); ` +
					`[ "$owner" = "10001" ] || chown -R 10001:10001 /opt/thunderid/repository/database`},
			SecurityContext: &corev1.SecurityContext{RunAsUser: &root},
			VolumeMounts: []corev1.VolumeMount{
				{Name: volumeNameData, MountPath: "/opt/thunderid/repository/database"},
			},
		})
	}

	// Only needed with a PVC: setup.sh there still chmod's this file every time it runs (see
	// startCmd above), which needs a writable location since Secret volumes are always read-only.
	// Without a PVC, direct_auth_secret is mounted directly, read-only, from the security Secret
	// (see volumeMounts above), so nothing needs a writable copy.
	if hasDataVolume {
		seedDirectAuthSecretCmd := fmt.Sprintf(
			"[ -f /opt/thunderid/config/secrets/%[1]s ] || "+
				"{ cp /run/security-seed/%[1]s /opt/thunderid/config/secrets/%[1]s && chmod 600 /opt/thunderid/config/secrets/%[1]s; }",
			securityKeyDirectAuth)
		initContainers = append(initContainers, corev1.Container{
			// RunAsUser must match the "thunderid" container's own uid (10001, from its image's
			// default): busybox defaults to root, so without this the file gets created root-owned,
			// and chmod 600 then locks the "thunderid" container (uid 10001) out with "permission
			// denied" — chmod 600 strips group/other bits entirely, so the pod's FSGroup can't paper
			// over a uid mismatch here the way it can for group-readable files.
			Name:            "seed-direct-auth-secret",
			Image:           initContainerImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{shBinPath, "-c", seedDirectAuthSecretCmd},
			SecurityContext: &corev1.SecurityContext{RunAsUser: &thunderidUID},
			VolumeMounts: []corev1.VolumeMount{
				{Name: volumeNameSecurity, MountPath: "/run/security-seed", ReadOnly: true},
				{Name: volumeNameData, MountPath: secretsMountPath, SubPath: "secrets"},
			},
		})
	}

	// The data PVC is ReadWriteOnce, so a surging RollingUpdate (the default) would strand the
	// new pod Pending trying to mount a volume the old pod still holds. Recreate tears the old
	// pod down first.
	strategy := appsv1.DeploymentStrategy{}
	if hasDataVolume {
		strategy.Type = appsv1.RecreateDeploymentStrategyType
	}

	// Get or create the Deployment.
	dep := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, dep)
	if errors.IsNotFound(err) {
		if replicas == 0 {
			// Always start at 1, even with autoscaling on — creating with 0 replicas
			// makes the HPA treat the Deployment as intentionally paused and it will
			// never scale it up. The HPA takes over cleanly from 1 replica onward.
			replicas = 1
		}
		dep = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.Name,
				Namespace: instance.Namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Strategy: strategy,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{appLabelKey: instance.Name},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      map[string]string{appLabelKey: instance.Name},
						Annotations: map[string]string{"thunderid.io/config-hash": configHash},
					},
					Spec: corev1.PodSpec{
						SecurityContext: podSecurityContext,
						InitContainers:  initContainers,
						Containers: []corev1.Container{
							{
								Name:    thunderidContainerName,
								Image:   instance.Spec.Image,
								Command: []string{bashBinPath, "-c", startCmd},
								Env: []corev1.EnvVar{
									{Name: adminUsernameEnvKey, Value: defaultAdminUsername},
									{Name: adminPasswordEnvKey, ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: securitySecretName},
											Key:                  securityKeyAdminPassword,
										},
									}},
								},
								EnvFrom:        envFrom,
								Ports:          []corev1.ContainerPort{{ContainerPort: port, Protocol: corev1.ProtocolTCP}},
								VolumeMounts:   volumeMounts,
								Resources:      instance.Spec.Resources,
								StartupProbe:   thunderidStartupProbe(port),
								LivenessProbe:  thunderidLivenessProbe(port),
								ReadinessProbe: thunderidReadinessProbe(port),
							},
						},
						Volumes: volumes,
					},
				},
			},
		}
		_ = ctrl.SetControllerReference(instance, dep, r.Scheme)
		log.Info("creating Deployment", "replicas", replicas, "image", instance.Spec.Image)
		return r.Create(ctx, dep)
	}
	if err != nil {
		return err
	}

	// Snapshot the whole object before mutating, so the diff below covers every field this
	// function manages — adding a new field to the Deployment template later is automatically
	// picked up, instead of needing a matching addition to a hand-picked comparison list. A
	// stale "bootstrap" volume, or a security volume pointing at the wrong Secret, are just
	// ordinary diffs against this snapshot — Volumes is replaced wholesale below either way.
	original := dep.DeepCopy()

	if instance.Spec.AutoScaling == nil && instance.Spec.Replicas > 0 {
		r := instance.Spec.Replicas
		dep.Spec.Replicas = &r
	}
	dep.Spec.Strategy = strategy

	dep.Spec.Template.Spec.Containers[0].Image = instance.Spec.Image
	dep.Spec.Template.Spec.Containers[0].Command = []string{bashBinPath, "-c", startCmd}
	dep.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{Name: adminUsernameEnvKey, Value: defaultAdminUsername},
		{Name: adminPasswordEnvKey, ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: securitySecretName},
				Key:                  securityKeyAdminPassword,
			},
		}},
	}
	dep.Spec.Template.Spec.Containers[0].EnvFrom = envFrom
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
	dep.Spec.Template.Spec.Containers[0].Resources = instance.Spec.Resources
	dep.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{{ContainerPort: port, Protocol: corev1.ProtocolTCP}}
	dep.Spec.Template.Spec.Containers[0].StartupProbe = thunderidStartupProbe(port)
	dep.Spec.Template.Spec.Containers[0].LivenessProbe = thunderidLivenessProbe(port)
	dep.Spec.Template.Spec.Containers[0].ReadinessProbe = thunderidReadinessProbe(port)
	dep.Spec.Template.Spec.Volumes = volumes
	dep.Spec.Template.Spec.InitContainers = initContainers
	dep.Spec.Template.Spec.SecurityContext = podSecurityContext
	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = map[string]string{}
	}
	dep.Spec.Template.Annotations["thunderid.io/config-hash"] = configHash

	// original and dep both descend from the same fetch, so any field this function didn't
	// touch above is guaranteed identical on both sides already — the comparison only needs to
	// normalize the handful of fields replaced wholesale with a freshly built Go value (Strategy,
	// SecurityContext, InitContainers) rather than mutated field-by-field on the existing object:
	// those fresh literals leave fields at their Go zero value that the API server represents
	// differently once persisted (nil *PodSecurityContext round-trips to a non-nil empty one, an
	// unset Strategy round-trips to the fully-defaulted RollingUpdate/25%, a freshly built
	// container never sets terminationMessagePath/Policy) — comparing those raw would mismatch on
	// every single reconcile, forcing an update (and, since Deployment is Owned() with
	// GenerationChangedPredicate, an immediate re-reconcile) forever.
	if equality.Semantic.DeepEqual(normalizeDeploymentSpecForDiff(original.Spec), normalizeDeploymentSpecForDiff(dep.Spec)) {
		log.V(1).Info("Deployment unchanged, skipping update")
		return nil
	}
	log.Info("Deployment spec drifted, updating", "image", instance.Spec.Image)
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: dep.Name, Namespace: dep.Namespace}, latest); err != nil {
			return err
		}
		latest.Spec = dep.Spec
		return r.Update(ctx, latest)
	})

}

// normalizePodSecurityContext treats nil the same as an explicit empty PodSecurityContext: the API
// server round-trips a nil pointer submitted on create/update into a non-nil empty one on read
// back, so comparing raw pointers would always see a (spurious) difference.
func normalizePodSecurityContext(sc *corev1.PodSecurityContext) *corev1.PodSecurityContext {
	if sc == nil {
		return &corev1.PodSecurityContext{}
	}
	return sc
}

// normalizeContainersForDiff strips fields the API server defaults on every container
// (terminationMessagePath/Policy) that a freshly built literal never sets, so a raw comparison
// against a fetched "existing" container would always see a (spurious) difference.
func normalizeContainersForDiff(containers []corev1.Container) []corev1.Container {
	out := make([]corev1.Container, len(containers))
	for i, c := range containers {
		c.TerminationMessagePath = ""
		c.TerminationMessagePolicy = ""
		out[i] = c
	}
	return out
}

// normalizeDeploymentSpecForDiff normalizes the fields reconcileDeployment replaces wholesale
// with a freshly built Go value every reconcile (as opposed to mutating field-by-field on the
// already-fetched object) — Strategy and SecurityContext can be left at their Go zero value,
// and InitContainers is a completely fresh slice, so each needs the same "compare what the API
// server would actually persist" treatment as normalizePodSecurityContext/
// normalizeContainersForDiff above.
func normalizeDeploymentSpecForDiff(spec appsv1.DeploymentSpec) appsv1.DeploymentSpec {
	spec.Strategy = normalizeDeploymentStrategyForDiff(spec.Strategy)
	spec.Template.Spec.SecurityContext = normalizePodSecurityContext(spec.Template.Spec.SecurityContext)
	spec.Template.Spec.InitContainers = normalizeContainersForDiff(spec.Template.Spec.InitContainers)
	return spec
}

// normalizeDeploymentStrategyForDiff fills in apps/v1's own default RollingUpdate values
// (Type: RollingUpdate, MaxUnavailable/MaxSurge: 25%) for a zero-valued Strategy — every
// reconcile of a non-PVC instance leaves Strategy at its Go zero value, but the API server
// always persists that as the fully-defaulted RollingUpdate, never as empty.
func normalizeDeploymentStrategyForDiff(s appsv1.DeploymentStrategy) appsv1.DeploymentStrategy {
	if s.Type != "" {
		return s
	}
	maxUnavailable := intstr.FromString("25%")
	maxSurge := intstr.FromString("25%")
	return appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxUnavailable: &maxUnavailable,
			MaxSurge:       &maxSurge,
		},
	}
}

// reconcileService creates the ClusterIP Service fronting the serving pods, or updates its port
// if spec.config.port changed — the only field of this Service derived from the CR.
func (r *ThunderIDInstanceReconciler) reconcileService(
	ctx context.Context, instance *appsv1alpha1.ThunderIDInstance) error {
	log := logf.FromContext(ctx)
	// Resolve the port.
	port := instance.Spec.Config.Port
	if port == 0 {
		port = 8090
	}
	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, svc)
	if errors.IsNotFound(err) {
		log.Info("creating Service", "port", port)
		svc = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.Name,
				Namespace: instance.Namespace,
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{appLabelKey: instance.Name},
				Ports: []corev1.ServicePort{
					{
						Port:       port,
						TargetPort: intstr.FromInt32(port),
						Protocol:   corev1.ProtocolTCP,
					},
				},
			},
		}
		_ = ctrl.SetControllerReference(instance, svc, r.Scheme)
		return r.Create(ctx, svc)
	}
	if err != nil {
		return err
	}
	// spec.config.port is the only thing about this Service that ever changes after creation — no
	// other field is derived from the CR at all — so a plain equality check on it is enough to
	// decide whether an update is needed, unlike the Deployment's much larger diff.
	if len(svc.Spec.Ports) == 1 && svc.Spec.Ports[0].Port == port && svc.Spec.Ports[0].TargetPort == intstr.FromInt32(port) {
		return nil
	}
	log.Info("Service port drifted, updating", "port", port)
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Port:       port,
			TargetPort: intstr.FromInt32(port),
			Protocol:   corev1.ProtocolTCP,
		},
	}
	return r.Update(ctx, svc)
}

// reconcileIngress creates the Ingress when spec.domain is set (deleting one left over from a
// time it was, if it's since been cleared), and on an existing one keeps only its backend port in
// sync with spec.config.port — see the update path's own comment for what's deliberately not
// covered.
func (r *ThunderIDInstanceReconciler) reconcileIngress(
	ctx context.Context, instance *appsv1alpha1.ThunderIDInstance) error {
	log := logf.FromContext(ctx)
	// Resolve the port.
	port := instance.Spec.Config.Port
	if port == 0 {
		port = 8090
	}
	pathType := networkingv1.PathTypePrefix
	ing := &networkingv1.Ingress{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, ing)

	// No domain means no Ingress — the default, local-dev path (kubectl port-forward instead).
	// If one exists from a time when spec.domain was set, clean it up rather than leave it
	// orphaned pointing at a host nobody configured anymore.
	if instance.Spec.Domain == "" {
		if err == nil {
			log.Info("spec.domain cleared, deleting Ingress")
			return r.Delete(ctx, ing)
		}
		return client.IgnoreNotFound(err)
	}
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	// Built once for both the create and update paths, so a domain/tlsSecret/port change is
	// applied identically regardless of whether the Ingress already exists.
	desired := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
				"nginx.ingress.kubernetes.io/ssl-redirect":     "true",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: instance.Spec.Domain,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: instance.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: port,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	if instance.Spec.TLSSecret != "" {
		desired.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{instance.Spec.Domain},
				SecretName: instance.Spec.TLSSecret,
			},
		}
	}

	if errors.IsNotFound(err) {
		log.Info("creating Ingress", "domain", instance.Spec.Domain)
		_ = ctrl.SetControllerReference(instance, desired, r.Scheme)
		return r.Create(ctx, desired)
	}

	// Compare and replace the whole spec, rather than diffing individual fields (e.g. just the
	// backend port, as a prior version of this check did) — that missed domain/tlsSecret changes
	// entirely, and indexing into ing.Spec.Rules[0].HTTP.Paths[0] without checking their length
	// first would panic on every reconcile if the Ingress's rules/http block were ever edited or
	// removed out from under it (e.g. by hand, or by another controller).
	if equality.Semantic.DeepEqual(ing.Spec, desired.Spec) {
		return nil
	}
	log.Info("Ingress spec drifted, updating", "domain", instance.Spec.Domain, "port", port)
	patch := client.MergeFrom(ing.DeepCopy())
	ing.Spec = desired.Spec
	return r.Patch(ctx, ing, patch)
}

// reconcileHPA creates or updates the HorizontalPodAutoscaler to match spec.autoScaling, or
// deletes it once spec.autoScaling is cleared.
func (r *ThunderIDInstanceReconciler) reconcileHPA(ctx context.Context, instance *appsv1alpha1.ThunderIDInstance) error {
	log := logf.FromContext(ctx)

	// hpaKey is the HPA's own name/namespace - same as the instance's, and thus the same as the
	// Deployment it scales (see ScaleTargetRef below), so all three share one name to look up by.
	hpaKey := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}

	if instance.Spec.AutoScaling == nil {
		// No autoscaling desired — delete HPA if it exists.
		existing := &autoscalingv2.HorizontalPodAutoscaler{}
		if err := r.Get(ctx, hpaKey, existing); err == nil {
			log.Info("spec.autoScaling cleared, deleting HPA")
			return r.Delete(ctx, existing)
		}
		return nil
	}

	// Build the desired HPA spec.
	as := instance.Spec.AutoScaling
	// Convert the CPU/memory percentages into HPA MetricSpecs, defaulting to 80% CPU if neither is set.
	// These metrics are what k8s uses to decide when to scale up/down the Deployment, so they must be set even if the user didn't specify any.
	metrics := buildMetrics(as)

	desired := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       instance.Name,
			},
			MinReplicas: &as.MinReplicas,
			MaxReplicas: as.MaxReplicas,
			Metrics:     metrics,
		},
	}
	_ = ctrl.SetControllerReference(instance, desired, r.Scheme)

	// Get or create/update it.
	existing := &autoscalingv2.HorizontalPodAutoscaler{}
	err := r.Get(ctx, hpaKey, existing)
	if errors.IsNotFound(err) {
		log.Info("creating HPA", "minReplicas", as.MinReplicas, "maxReplicas", as.MaxReplicas)
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		return nil
	}
	log.Info("HPA spec drifted, updating", "minReplicas", as.MinReplicas, "maxReplicas", as.MaxReplicas)
	patch := client.MergeFrom(existing.DeepCopy())
	existing.Spec = desired.Spec
	return r.Patch(ctx, existing, patch)
}

// buildMetrics converts spec.autoScaling's CPU/memory utilization percentages into HPA
// MetricSpecs, defaulting to 80% CPU when neither is set.
func buildMetrics(as *appsv1alpha1.AutoScalingSpec) []autoscalingv2.MetricSpec {
	var metrics []autoscalingv2.MetricSpec

	// CPU metric, if requested.
	if as.TargetCPUUtilizationPercentage > 0 {
		pct := as.TargetCPUUtilizationPercentage
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: &pct,
				},
			},
		})
	}

	// Memory metric, if requested.
	if as.TargetMemoryUtilizationPercentage > 0 {
		pct := as.TargetMemoryUtilizationPercentage
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: &pct,
				},
			},
		})
	}

	// Default to 80% CPU if no metrics were specified.
	if len(metrics) == 0 {
		cpu := int32(80)
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: &cpu,
				},
			},
		})
	}

	return metrics
}
