// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

type EmailConfigTestSuite struct {
	suite.Suite
}

func TestEmailConfigTestSuite(t *testing.T) {
	suite.Run(t, new(EmailConfigTestSuite))
}

func (suite *EmailConfigTestSuite) TestResolveEmailConfigUnset() {
	r := newFakeReconciler()
	instance := &appsv1alpha1.ThunderIDInstance{}
	block, err := r.resolveEmailConfig(context.Background(), instance)
	suite.Require().NoError(err)
	suite.Empty(block)
}

func (suite *EmailConfigTestSuite) TestResolveEmailConfigNoAuthentication() {
	r := newFakeReconciler()
	instance := &appsv1alpha1.ThunderIDInstance{Spec: appsv1alpha1.ThunderIDInstanceSpec{
		Config: appsv1alpha1.AppConfigSpec{Email: &appsv1alpha1.EmailSpec{
			SMTP: appsv1alpha1.SMTPConfigSpec{Host: "smtp.example.com", Port: 587, FromAddress: "noreply@example.com"},
		}},
	}}
	block, err := r.resolveEmailConfig(context.Background(), instance)
	suite.Require().NoError(err)
	suite.Contains(block, `host: "smtp.example.com"`)
	suite.Contains(block, `port: 587`)
	suite.Contains(block, `password: ""`)
	suite.Contains(block, "enable_authentication: false")
}

func (suite *EmailConfigTestSuite) TestResolveEmailConfigAuthenticationRequiresSecretRef() {
	r := newFakeReconciler()
	instance := &appsv1alpha1.ThunderIDInstance{Spec: appsv1alpha1.ThunderIDInstanceSpec{
		Config: appsv1alpha1.AppConfigSpec{Email: &appsv1alpha1.EmailSpec{
			SMTP: appsv1alpha1.SMTPConfigSpec{
				Host: "smtp.example.com", Port: 587, FromAddress: "noreply@example.com",
				EnableAuthentication: true,
			},
		}},
	}}
	_, err := r.resolveEmailConfig(context.Background(), instance)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "secretRef")
}

func (suite *EmailConfigTestSuite) TestResolveEmailConfigAuthenticationSecretNotFound() {
	r := newFakeReconciler() // no Secret seeded at all
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			Config: appsv1alpha1.AppConfigSpec{Email: &appsv1alpha1.EmailSpec{
				SMTP: appsv1alpha1.SMTPConfigSpec{
					Host: "smtp.example.com", Port: 587, FromAddress: "noreply@example.com",
					EnableAuthentication: true,
					SecretRef:            &appsv1alpha1.SecretKeyRef{Name: "does-not-exist", Key: "password"},
				},
			}},
		},
	}
	_, err := r.resolveEmailConfig(context.Background(), instance)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "does-not-exist")
}

func (suite *EmailConfigTestSuite) TestResolveEmailConfigAuthenticationMissingPasswordKey() {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "email-secret", Namespace: "default"},
		Data:       map[string][]byte{"not-password": []byte("x")},
	}
	r := newFakeReconciler(secret)
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			Config: appsv1alpha1.AppConfigSpec{Email: &appsv1alpha1.EmailSpec{
				SMTP: appsv1alpha1.SMTPConfigSpec{
					Host: "smtp.example.com", Port: 587, FromAddress: "noreply@example.com",
					EnableAuthentication: true,
					SecretRef:            &appsv1alpha1.SecretKeyRef{Name: "email-secret", Key: "password"},
				},
			}},
		},
	}
	_, err := r.resolveEmailConfig(context.Background(), instance)
	suite.Require().Error(err)
	suite.Contains(err.Error(), `missing required key "password"`)
}

func (suite *EmailConfigTestSuite) TestResolveEmailConfigAuthenticationSuccess() {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "email-secret", Namespace: "default"},
		Data:       map[string][]byte{"password": []byte("hunter2")},
	}
	r := newFakeReconciler(secret)
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			Config: appsv1alpha1.AppConfigSpec{Email: &appsv1alpha1.EmailSpec{
				SMTP: appsv1alpha1.SMTPConfigSpec{
					Host: "smtp.example.com", Port: 587, Username: "svc", FromAddress: "noreply@example.com",
					EnableAuthentication: true, EnableStartTLS: true,
					SecretRef: &appsv1alpha1.SecretKeyRef{Name: "email-secret", Key: "password"},
				},
			}},
		},
	}
	block, err := r.resolveEmailConfig(context.Background(), instance)
	suite.Require().NoError(err)
	// The literal password never appears - only the file:// reference to its mount path.
	suite.NotContains(block, "hunter2")
	suite.Contains(block, `password: "file://`+emailPasswordMountPath+`"`)
	suite.Contains(block, "enable_authentication: true")
	suite.Contains(block, "enable_start_tls: true")
}

// TestResolveEmailConfigSameSecretDifferentKeyAsDatabase proves email.smtp.secretRef can point at
// the same Secret object a database scope's postgres.passwordRef already uses, just a different key —
// no separate Secret required.
func (suite *EmailConfigTestSuite) TestResolveEmailConfigSameSecretDifferentKeyAsDatabase() {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-secret", Namespace: "default"},
		Data:       map[string][]byte{"db-password": []byte("dbpass"), "smtp-password": []byte("smtppass")},
	}
	r := newFakeReconciler(secret)
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			Config: appsv1alpha1.AppConfigSpec{Email: &appsv1alpha1.EmailSpec{
				SMTP: appsv1alpha1.SMTPConfigSpec{
					Host: "smtp.example.com", Port: 587, FromAddress: "noreply@example.com",
					EnableAuthentication: true,
					SecretRef:            &appsv1alpha1.SecretKeyRef{Name: "shared-secret", Key: "smtp-password"},
				},
			}},
		},
	}
	block, err := r.resolveEmailConfig(context.Background(), instance)
	suite.Require().NoError(err)
	suite.Contains(block, `password: "file://`+emailPasswordMountPath+`"`)
}
