// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

const (
	// emailSecretKeyPassword is the fixed local filename spec.config.email.secretRef's (arbitrary)
	// key is projected to inside the email Volume — same role as databaseSecretKeyPassword plays
	// for the database scopes' Volumes.
	emailSecretKeyPassword = "password"

	// emailPasswordMountPath is where the password key of spec.config.email.secretRef is mounted into
	// the container. Referenced via a file:// URI, the same pattern deployment.yaml already uses
	// for direct_auth_secret, crypto.key, and the database password.
	emailPasswordMountPath = "config/secrets/email_password"

	// emailBlockMarkerLine is the placeholder line in deployment-config.yaml that
	// reconcileConfigMap replaces with the resolved email: section, or removes entirely when
	// spec.config.email is unset — matching the image's own out-of-the-box behavior of no email config.
	emailBlockMarkerLine = `__EMAIL_BLOCK__: placeholder # replaced whole-line by reconcileConfigMap with the email: section, or removed if spec.config.email is unset`
)

// resolveEmailConfig returns the YAML to substitute for emailBlockMarkerLine — "" when
// spec.config.email is unset, so the template ends up with no email: section at all. The password (when
// spec.config.email.smtp.enableAuthentication is set) is kept out of the ConfigMap and resolved at
// runtime via a file:// reference, the same way resolveDatabaseConfig handles the database password.
func (r *ThunderIDInstanceReconciler) resolveEmailConfig(
	ctx context.Context, instance *appsv1alpha1.ThunderIDInstance) (block string, err error) {
	email := instance.Spec.Config.Email
	if email == nil {
		return "", nil
	}

	password := ""
	if email.SMTP.EnableAuthentication {
		secretRef := email.SMTP.SecretRef
		if secretRef == nil || secretRef.Name == "" || secretRef.Key == "" {
			return "", fmt.Errorf("spec.config.email.smtp.enableAuthentication is true but spec.config.email.smtp.secretRef.name/secretRef.key is not set")
		}
		secret := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{
			Name: secretRef.Name, Namespace: instance.Namespace,
		}, secret); err != nil {
			return "", fmt.Errorf("spec.config.email.smtp.secretRef %q: %w", secretRef.Name, err)
		}
		if string(secret.Data[secretRef.Key]) == "" {
			return "", fmt.Errorf("spec.config.email.smtp.secretRef: Secret %q is missing required key %q", secretRef.Name, secretRef.Key)
		}
		password = "file://" + emailPasswordMountPath
	}

	block = fmt.Sprintf(`email:
  smtp:
    host: %q
    port: %d
    username: %q
    password: %q
    from_address: %q
    enable_start_tls: %t
    enable_authentication: %t`,
		email.SMTP.Host, email.SMTP.Port, email.SMTP.Username, password, email.SMTP.FromAddress,
		email.SMTP.EnableStartTLS, email.SMTP.EnableAuthentication)

	return block, nil
}
