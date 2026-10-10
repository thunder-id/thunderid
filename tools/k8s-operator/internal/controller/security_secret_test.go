// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

type SecuritySecretTestSuite struct {
	suite.Suite
}

func TestSecuritySecretTestSuite(t *testing.T) {
	suite.Run(t, new(SecuritySecretTestSuite))
}

func (suite *SecuritySecretTestSuite) TestGenerateSelfSignedCertRSA() {
	certPEM, keyPEM, err := generateSelfSignedCert("rsa")
	suite.Require().NoError(err)

	certBlock, _ := pem.Decode(certPEM)
	suite.Require().NotNil(certBlock)
	suite.Equal("CERTIFICATE", certBlock.Type)
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	suite.Require().NoError(err)
	suite.Equal("localhost", cert.Subject.CommonName)

	keyBlock, _ := pem.Decode(keyPEM)
	suite.Require().NotNil(keyBlock)
	suite.Equal("PRIVATE KEY", keyBlock.Type)
	_, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	suite.NoError(err, "generated RSA key must parse as PKCS8")
}

func (suite *SecuritySecretTestSuite) TestGenerateSelfSignedCertECDSA() {
	certPEM, keyPEM, err := generateSelfSignedCert("ecdsa")
	suite.Require().NoError(err)

	certBlock, _ := pem.Decode(certPEM)
	suite.Require().NotNil(certBlock)
	_, err = x509.ParseCertificate(certBlock.Bytes)
	suite.NoError(err)

	keyBlock, _ := pem.Decode(keyPEM)
	suite.Require().NotNil(keyBlock)
	suite.Equal("EC PRIVATE KEY", keyBlock.Type)
	_, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	suite.NoError(err, "generated ECDSA key must parse as SEC1")
}

func (suite *SecuritySecretTestSuite) TestRandomHexSecret() {
	a, err := randomHexSecret()
	suite.Require().NoError(err)
	_, err = hex.DecodeString(string(a))
	suite.NoError(err, "randomHexSecret must be valid hex")
	suite.Len(a, 64, "32 bytes hex-encoded")
	suite.False(hasSuffixNewline(a), "matches setup.sh's printf '%%s' - no trailing newline")

	b, err := randomHexSecret()
	suite.Require().NoError(err)
	suite.NotEqual(string(a), string(b))
}

func hasSuffixNewline(b []byte) bool {
	return len(b) > 0 && b[len(b)-1] == '\n'
}

func (suite *SecuritySecretTestSuite) TestReconcileSecuritySecretGeneratesOnce() {
	r := newFakeReconciler()
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}

	name, err := r.reconcileSecuritySecret(context.Background(), instance)
	suite.Require().NoError(err)
	suite.Equal("inst-security", name)

	secret := &corev1.Secret{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: name, Namespace: "default"}, secret))
	for _, key := range securityAllKeys {
		suite.NotEmpty(secret.Data[key], "key %q", key)
	}
	firstResourceVersion := secret.ResourceVersion

	// Second reconcile must not touch the Secret: the JWT signing key (and already-issued
	// tokens) must stay stable across reconciles, not just across pod restarts.
	name2, err := r.reconcileSecuritySecret(context.Background(), instance)
	suite.Require().NoError(err)
	suite.Equal(name, name2)

	secret2 := &corev1.Secret{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: name, Namespace: "default"}, secret2))
	suite.Equal(firstResourceVersion, secret2.ResourceVersion, "Secret must not be modified on second reconcile")
}

// A pre-existing <name>-security Secret the instance didn't create must not be reused - it would
// mount another workload's TLS/signing keys into this instance's pod.
func (suite *SecuritySecretTestSuite) TestReconcileSecuritySecretForeignSecretRejected() {
	foreign := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-security", Namespace: "default"},
		Data:       map[string][]byte{securityKeyServerKey: []byte("someone-elses-key")},
	}
	r := newFakeReconciler(foreign)
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}

	_, err := r.reconcileSecuritySecret(context.Background(), instance)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not controlled by ThunderIDInstance")
}

func (suite *SecuritySecretTestSuite) TestReconcileSecuritySecretUserSuppliedValid() {
	data := map[string][]byte{}
	for _, key := range securityAllKeys {
		data[key] = []byte("provided-" + key)
	}
	provided := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-security", Namespace: "default"},
		Data:       data,
	}
	r := newFakeReconciler(provided)
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec:       appsv1alpha1.ThunderIDInstanceSpec{SecuritySecret: "my-security"},
	}

	name, err := r.reconcileSecuritySecret(context.Background(), instance)
	suite.Require().NoError(err)
	suite.Equal("my-security", name, "used as-is, not regenerated")
}

func (suite *SecuritySecretTestSuite) TestReconcileSecuritySecretUserSuppliedMissingKey() {
	provided := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "incomplete-security", Namespace: "default"},
		Data:       map[string][]byte{securityKeyServerCert: []byte("x")},
	}
	r := newFakeReconciler(provided)
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec:       appsv1alpha1.ThunderIDInstanceSpec{SecuritySecret: "incomplete-security"},
	}

	_, err := r.reconcileSecuritySecret(context.Background(), instance)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "missing required key")
}

func (suite *SecuritySecretTestSuite) TestReconcileSecuritySecretUserSuppliedNotFound() {
	r := newFakeReconciler()
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec:       appsv1alpha1.ThunderIDInstanceSpec{SecuritySecret: "does-not-exist"},
	}

	_, err := r.reconcileSecuritySecret(context.Background(), instance)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not found")
}

func (suite *SecuritySecretTestSuite) TestReconcileSecuritySecretUserSuppliedGetErrorPropagates() {
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec:       appsv1alpha1.ThunderIDInstanceSpec{SecuritySecret: "my-security"},
	}
	wantErr := errors.New("simulated get failure")
	r := newFakeReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*corev1.Secret); ok {
				return wantErr
			}
			return c.Get(ctx, key, o, opts...)
		},
	}, instance)

	_, err := r.reconcileSecuritySecret(context.Background(), instance)
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *SecuritySecretTestSuite) TestReconcileSecuritySecretAutoGeneratedGetErrorPropagates() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	wantErr := errors.New("simulated get failure")
	r := newFakeReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*corev1.Secret); ok {
				return wantErr
			}
			return c.Get(ctx, key, o, opts...)
		},
	}, instance)

	_, err := r.reconcileSecuritySecret(context.Background(), instance)
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *SecuritySecretTestSuite) TestReconcileSecuritySecretAdminPasswordSecretUsed() {
	provided := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-admin-password", Namespace: "default"},
		Data:       map[string][]byte{"password": []byte("hunter2")},
	}
	r := newFakeReconciler(provided)
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			AdminPasswordSecret: &appsv1alpha1.SecretKeyRef{Name: "my-admin-password", Key: "password"},
		},
	}

	name, err := r.reconcileSecuritySecret(context.Background(), instance)
	suite.Require().NoError(err)

	secret := &corev1.Secret{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: name, Namespace: "default"}, secret))
	suite.Equal([]byte("hunter2"), secret.Data[securityKeyAdminPassword])
	// Every other key is still generated, not left empty.
	suite.NotEmpty(secret.Data[securityKeyServerCert])
}

func (suite *SecuritySecretTestSuite) TestReconcileSecuritySecretAdminPasswordSecretNotFound() {
	r := newFakeReconciler()
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			AdminPasswordSecret: &appsv1alpha1.SecretKeyRef{Name: "does-not-exist", Key: "password"},
		},
	}

	_, err := r.reconcileSecuritySecret(context.Background(), instance)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not found")
}

func (suite *SecuritySecretTestSuite) TestReconcileSecuritySecretAdminPasswordSecretMissingKey() {
	provided := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-admin-password", Namespace: "default"},
		Data:       map[string][]byte{"wrong-key": []byte("hunter2")},
	}
	r := newFakeReconciler(provided)
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			AdminPasswordSecret: &appsv1alpha1.SecretKeyRef{Name: "my-admin-password", Key: "password"},
		},
	}

	_, err := r.reconcileSecuritySecret(context.Background(), instance)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "no key")
}

func (suite *SecuritySecretTestSuite) TestReconcileSecuritySecretBothSecurityAndAdminPasswordRejected() {
	r := newFakeReconciler()
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			SecuritySecret:      "my-security",
			AdminPasswordSecret: &appsv1alpha1.SecretKeyRef{Name: "my-admin-password", Key: "password"},
		},
	}

	_, err := r.reconcileSecuritySecret(context.Background(), instance)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "mutually exclusive")
}
