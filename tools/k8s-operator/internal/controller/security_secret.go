// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

// certOrgUnit is the self-signed certs' Subject.OrganizationalUnit, generated below to match what
// ThunderID's own setup.sh produces via openssl.
const certOrgUnit = "ThunderID"

const (
	securityKeyServerCert    = "server.cert"
	securityKeyServerKey     = "server.key"
	securityKeySigningCert   = "signing.cert"
	securityKeySigningKey    = "signing.key"
	securityKeyECDSACert     = "ecdsa-signing.cert"
	securityKeyECDSAKey      = "ecdsa-signing.key"
	securityKeyCryptoKey     = "crypto.key"
	securityKeyDirectAuth    = "direct_auth_secret"
	securityKeyAdminPassword = "admin-password"
)

// securityCertMountKeys are mounted read-only, individually by key name (via subPath), onto
// /opt/thunderid/config/certs/<key>. setup.sh only (re)generates a cert/key pair when it finds
// the file missing, so pre-supplying these from a stable Secret is what keeps them identical
// across pod restarts and across replicas.
var securityCertMountKeys = []string{
	securityKeyServerCert, securityKeyServerKey,
	securityKeySigningCert, securityKeySigningKey,
	securityKeyECDSACert, securityKeyECDSAKey,
	securityKeyCryptoKey,
}

// securityAllKeys is every key a security Secret — generated or user-supplied — must contain.
var securityAllKeys = append(append([]string{}, securityCertMountKeys...), securityKeyDirectAuth, securityKeyAdminPassword)

// reconcileSecuritySecret ensures a Secret holding ThunderID's TLS/JWT-signing/encryption key
// material and bootstrap admin password exists for this instance, generating it once on first
// create. It is deliberately
// never updated afterwards: setup.sh inside the ThunderID container only regenerates a
// cert/key pair when the file is absent, so keeping this Secret's content stable across
// reconciles is what keeps the JWT signing key (and therefore already-issued session tokens)
// stable across pod restarts and across replicas.
func (r *ThunderIDInstanceReconciler) reconcileSecuritySecret(
	ctx context.Context, instance *appsv1alpha1.ThunderIDInstance) (string, error) {
	log := logf.FromContext(ctx)
	if instance.Spec.SecuritySecret != "" && instance.Spec.AdminPasswordSecret != nil {
		return "", fmt.Errorf(
			"spec.securitySecret and spec.adminPasswordSecret are mutually exclusive: "+
				"spec.securitySecret %q already supplies admin-password itself", instance.Spec.SecuritySecret)
	}
	// A user-supplied Secret is used as-is and never generated or modified by the operator —
	// but it must carry every key our volume mounts reference by name, or the pod will fail to
	// start with an opaque "references non-existent secret key" event.
	if instance.Spec.SecuritySecret != "" {
		provided := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{
			Name: instance.Spec.SecuritySecret, Namespace: instance.Namespace,
		}, provided); err != nil {
			if errors.IsNotFound(err) {
				return "", fmt.Errorf("spec.securitySecret %q not found", instance.Spec.SecuritySecret)
			}
			return "", err
		}
		var missing []string
		for _, key := range securityAllKeys {
			if len(provided.Data[key]) == 0 {
				missing = append(missing, key)
			}
		}
		if len(missing) > 0 {
			return "", fmt.Errorf("spec.securitySecret %q is missing required key(s): %s",
				instance.Spec.SecuritySecret, strings.Join(missing, ", "))
		}
		log.V(1).Info("using user-supplied security Secret as-is", "name", instance.Spec.SecuritySecret)
		return instance.Spec.SecuritySecret, nil
	}

	secretName := instance.Name + "-security"

	existing := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: instance.Namespace}, existing)
	if err == nil {
		// Mounted into the pod as TLS/signing key material - reusing a Secret this instance doesn't
		// control would hand it another workload's keys.
		if err := requireOwnedBy(r.Recorder, instance, existing, "Secret"); err != nil {
			return "", err
		}
		log.V(1).Info("security Secret already exists, reusing (never regenerated - see reconcileSecuritySecret's own doc comment)", "name", secretName)
		return secretName, nil
	}
	if !errors.IsNotFound(err) {
		return "", err
	}

	log.Info("generating new security Secret (self-signed certs, JWT signing keys, encryption key) - one-time, will not be regenerated on future reconciles", "name", secretName)
	serverCert, serverKey, err := generateSelfSignedCert("rsa")
	if err != nil {
		return "", fmt.Errorf("generating server TLS cert: %w", err)
	}
	signingCert, signingKey, err := generateSelfSignedCert("rsa")
	if err != nil {
		return "", fmt.Errorf("generating JWT signing cert: %w", err)
	}
	ecdsaCert, ecdsaKey, err := generateSelfSignedCert("ecdsa")
	if err != nil {
		return "", fmt.Errorf("generating ECDSA JWT signing cert: %w", err)
	}
	cryptoKey, err := randomHexSecret()
	if err != nil {
		return "", fmt.Errorf("generating encryption key: %w", err)
	}
	directAuthSecret, err := randomHexSecret()
	if err != nil {
		return "", fmt.Errorf("generating direct auth secret: %w", err)
	}
	adminPassword, err := r.resolveAdminPassword(ctx, instance)
	if err != nil {
		return "", err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: instance.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			securityKeyServerCert:    serverCert,
			securityKeyServerKey:     serverKey,
			securityKeySigningCert:   signingCert,
			securityKeySigningKey:    signingKey,
			securityKeyECDSACert:     ecdsaCert,
			securityKeyECDSAKey:      ecdsaKey,
			securityKeyCryptoKey:     cryptoKey,
			securityKeyDirectAuth:    directAuthSecret,
			securityKeyAdminPassword: adminPassword,
		},
	}
	_ = ctrl.SetControllerReference(instance, secret, r.Scheme)
	return secretName, r.Create(ctx, secret)
}

// resolveAdminPassword returns instance.Spec.AdminPasswordSecret's value when set, otherwise a
// fresh random one, matching ThunderID's own setup.sh: ADMIN_PASSWORD supplied by the operator's
// caller when given, generated when not.
func (r *ThunderIDInstanceReconciler) resolveAdminPassword(
	ctx context.Context, instance *appsv1alpha1.ThunderIDInstance) ([]byte, error) {
	ref := instance.Spec.AdminPasswordSecret
	if ref == nil {
		password, err := randomHexSecret()
		if err != nil {
			return nil, fmt.Errorf("generating admin password: %w", err)
		}
		return password, nil
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: instance.Namespace}, secret); err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("spec.adminPasswordSecret %q not found", ref.Name)
		}
		return nil, err
	}
	password, ok := secret.Data[ref.Key]
	if !ok || len(password) == 0 {
		return nil, fmt.Errorf("spec.adminPasswordSecret %q has no key %q", ref.Name, ref.Key)
	}
	return password, nil
}

// generateSelfSignedCert mirrors the self-signed certs ThunderID's own setup.sh generates via
// openssl, including the on-disk key encoding each openssl invocation produces: PKCS8 "PRIVATE
// KEY" for RSA (openssl req -newkey rsa:...), SEC1 "EC PRIVATE KEY" for ECDSA (openssl ecparam
// -genkey). Matching these formats means ThunderID's own PEM parsing needs no changes to accept
// operator-supplied key material in place of setup.sh-generated material.
func generateSelfSignedCert(algo string) (certPEM, keyPEM []byte, err error) {
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:       []string{"WSO2"},
			OrganizationalUnit: []string{certOrgUnit},
			CommonName:         "localhost",
		},
		NotBefore:             time.Now(),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true,
	}

	var signer crypto.Signer
	var pub any
	var keyBlock *pem.Block
	if algo == "ecdsa" {
		template.NotAfter = time.Now().AddDate(10, 0, 0)
		priv, genErr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if genErr != nil {
			return nil, nil, genErr
		}
		der, marshalErr := x509.MarshalECPrivateKey(priv)
		if marshalErr != nil {
			return nil, nil, marshalErr
		}
		signer, pub = priv, &priv.PublicKey
		keyBlock = &pem.Block{Type: "EC PRIVATE KEY", Bytes: der}
	} else {
		template.NotAfter = time.Now().AddDate(1, 0, 0)
		priv, genErr := rsa.GenerateKey(rand.Reader, 2048)
		if genErr != nil {
			return nil, nil, genErr
		}
		der, marshalErr := x509.MarshalPKCS8PrivateKey(priv)
		if marshalErr != nil {
			return nil, nil, marshalErr
		}
		signer, pub = priv, &priv.PublicKey
		keyBlock = &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, pub, signer)
	if err != nil {
		return nil, nil, err
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM = pem.EncodeToMemory(keyBlock)
	return certPEM, keyPEM, nil
}

// randomHexSecret matches setup.sh's `openssl rand -hex 32`, written without a trailing
// newline (setup.sh writes these with `printf '%s'`).
func randomHexSecret() ([]byte, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	return []byte(hex.EncodeToString(buf)), nil
}
