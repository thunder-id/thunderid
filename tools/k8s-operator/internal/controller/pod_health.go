// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

// configErrorReasons are container Waiting reasons that mean the container never started because
// Kubernetes couldn't resolve something the pod spec referenced — a missing (or wrong-namespace)
// Secret/ConfigMap, or an image that can't be pulled. None of these ever produce a termination
// state, since the container never ran, so the crash checks below never catch them on their own.

// reasonCrashLoopBackOff is the container Waiting reason Kubernetes uses once it's given up
// retrying a repeatedly-crashing container — checked separately from configErrorReasons since it
// means "started and crashed," not "never started."
const reasonCrashLoopBackOff = "CrashLoopBackOff"

// reasonCrashed is a podIssue.reason value of our own making (Kubernetes has no such container
// Waiting/Terminated reason) - set when diagnosePod finds a recent non-zero exit via
// LastTerminationState, rather than CrashLoopBackOff's own Waiting reason.
const reasonCrashed = "Crashed"

const reasonCreateContainerConfigError = "CreateContainerConfigError"

var configErrorReasons = map[string]bool{
	reasonCreateContainerConfigError: true,
	"CreateContainerError":           true,
	"ImagePullBackOff":               true,
	"ErrImagePull":                   true,
	"InvalidImageName":               true,
}

// podIssue is what diagnosePod found wrong with the thunderid container.
type podIssue struct {
	reason  string
	message string
}

// diagnosePod inspects pod's thunderid container for two disjoint failure shapes: one that
// started and then crashed (CrashLoopBackOff, or terminated non-zero since its last Ready — see
// the !cs.Ready comment below for why that specific check is gated the way it is), and one that
// never started at all because Kubernetes couldn't resolve something in the pod spec
// (configErrorReasons). The second shape has no termination state to inspect, so it needs its own
// branch entirely separate from the crash logic — a pod stuck on a missing envFrom Secret, for
// example, sits in Waiting with no LastTerminationState.Terminated ever set.
func diagnosePod(pod *corev1.Pod) (unhealthy bool, issue podIssue) {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name != thunderidContainerName {
			continue
		}
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			if reason == reasonCrashLoopBackOff || configErrorReasons[reason] {
				return true, podIssue{reason: reason, message: cs.State.Waiting.Message}
			}
		}
		// LastTerminationState reflects the most recent termination ever, not the current state —
		// it stays populated forever after a single past crash, even once the container has been
		// Ready and healthy for hours since. Gating on !cs.Ready keeps this catching the narrow
		// window right after a crash but before Kubernetes has escalated to CrashLoopBackOff,
		// without permanently flagging a fully-recovered container as still crashed.
		if !cs.Ready && cs.LastTerminationState.Terminated != nil && cs.LastTerminationState.Terminated.ExitCode != 0 {
			return true, podIssue{reason: reasonCrashed, message: cs.LastTerminationState.Terminated.Message}
		}
	}
	return false, podIssue{}
}

// crashFallbackMessage describes a crashed thunderid container when nothing more specific is
// known: no termination message, and no recognized error in its log. That's typical when ThunderID
// is waiting on a database connection that hangs instead of failing (the server drops packets
// rather than refusing them): nothing is logged before the startup probe kills the container. On
// PostgreSQL the message therefore points at database reachability. Returns "" for any other
// issue.
func crashFallbackMessage(pod *corev1.Pod, instance *appsv1alpha1.ThunderIDInstance, issue podIssue) string {
	if issue.reason != reasonCrashed && issue.reason != reasonCrashLoopBackOff {
		return ""
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name != thunderidContainerName || cs.LastTerminationState.Terminated == nil {
			continue
		}
		t := cs.LastTerminationState.Terminated
		msg := fmt.Sprintf("ThunderID exited with code %d (%s) before becoming ready, and its log names no known cause.",
			t.ExitCode, t.Reason)
		if usingPostgres(instance) {
			msg += " It uses PostgreSQL: check that every spec.config.database.<scope>.postgres hostname and port is" +
				" reachable from the cluster. A connection that hangs instead of failing logs nothing before the" +
				" startup probe stops the container."
		}
		return msg
	}
	return ""
}

// diagnoseMissingSecret looks for the specific, easy-to-hit cause behind a CreateContainerConfigError:
// a Secret name relevant to instance (the one named by whichever EnvironmentValues is labeled for it,
// any config.database.<scope>.postgres.passwordRef, config.email.smtp.secretRef) that exists somewhere in the cluster, just
// not in instance's namespace. Kubelet's own "secret \"X\" not found" message only ever means
// "not found in this pod's namespace" — it has no way to know whether X exists elsewhere, so this
// is the only place that can tell a genuinely-missing Secret apart from one that's simply in the
// wrong namespace.
func diagnoseMissingSecret(ctx context.Context, c client.Client, instance *appsv1alpha1.ThunderIDInstance, issue podIssue) string {
	if issue.reason != reasonCreateContainerConfigError {
		return ""
	}

	var names []string
	if env, err := findEnvironment(ctx, c, instance); err == nil && env != nil {
		names = append(names, env.Name)
	}
	for _, ss := range collectDatabaseScopeSecrets(instance) {
		names = append(names, ss.ref.Name)
	}
	if instance.Spec.Config.Email != nil && instance.Spec.Config.Email.SMTP.SecretRef != nil {
		names = append(names, instance.Spec.Config.Email.SMTP.SecretRef.Name)
	}

	for _, name := range names {
		local := &corev1.Secret{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: name}, local); err == nil {
			continue // exists right where it should - not the problem
		}
		all := &corev1.SecretList{}
		if err := c.List(ctx, all); err != nil {
			continue
		}
		for _, s := range all.Items {
			if s.Name == name && s.Namespace != instance.Namespace {
				return fmt.Sprintf(
					"Secret %q not found in namespace %q, but a Secret with that name exists in namespace %q. Secrets must be in the same namespace as the instance (check the EnvironmentValues labeled thunderid.io/instance=%s, database.<scope>.postgres.passwordRef, email.smtp.secretRef)",
					name, instance.Namespace, s.Namespace, instance.Name)
			}
		}
	}
	return ""
}
