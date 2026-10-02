// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

type EnvSecretTestSuite struct {
	suite.Suite
}

func TestEnvSecretTestSuite(t *testing.T) {
	suite.Run(t, new(EnvSecretTestSuite))
}

func (suite *EnvSecretTestSuite) TestResolveEnvSecretHashUnset() {
	r := newFakeReconciler()
	instance := &appsv1alpha1.ThunderIDInstance{}
	hash, err := r.resolveEnvSecretHash(context.Background(), instance)
	suite.Require().NoError(err)
	suite.Empty(hash)
}

func (suite *EnvSecretTestSuite) TestResolveEnvSecretHashMissingSecret() {
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
	}
	env := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{
			Name: "missing-secret", Namespace: "default",
			Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
	}
	r := newFakeReconciler(env)
	_, err := r.resolveEnvSecretHash(context.Background(), instance)
	suite.Error(err)
}

func (suite *EnvSecretTestSuite) TestResolveEnvSecretHashMultipleEnvironmentsErrors() {
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
	}
	envA := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{
			Name: "env-a", Namespace: "default",
			Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
	}
	envB := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{
			Name: "env-b", Namespace: "default",
			Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
	}
	r := newFakeReconciler(envA, envB)
	_, err := r.resolveEnvSecretHash(context.Background(), instance)
	suite.Error(err)
}

func (suite *EnvSecretTestSuite) TestResolveEnvSecretHashStableFormat() {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "env-secret", Namespace: "default"},
		Data:       map[string][]byte{"A": []byte("1"), "B": []byte("2")},
	}
	env := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{
			Name: "env-secret", Namespace: "default",
			Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
	}
	r := newFakeReconciler(secret, env)
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
	}
	hash, err := r.resolveEnvSecretHash(context.Background(), instance)
	suite.Require().NoError(err)
	suite.Regexp(regexp.MustCompile(`^[0-9a-f]{16}$`), hash)
}

func (suite *EnvSecretTestSuite) TestResolveEnvSecretHashChangesWithContent() {
	build := func(data map[string][]byte) *ThunderIDInstanceReconciler {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "env-secret", Namespace: "default"},
			Data:       data,
		}
		env := &appsv1alpha1.EnvironmentValues{
			ObjectMeta: metav1.ObjectMeta{
				Name: "env-secret", Namespace: "default",
				Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
			},
		}
		return newFakeReconciler(secret, env)
	}
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
	}

	r1 := build(map[string][]byte{"KEY": []byte("value1")})
	hash1, err := r1.resolveEnvSecretHash(context.Background(), instance)
	suite.Require().NoError(err)

	r2 := build(map[string][]byte{"KEY": []byte("value2")})
	hash2, err := r2.resolveEnvSecretHash(context.Background(), instance)
	suite.Require().NoError(err)

	suite.NotEqual(hash1, hash2)

	r3 := build(map[string][]byte{"KEY": []byte("value1")})
	hash3, err := r3.resolveEnvSecretHash(context.Background(), instance)
	suite.Require().NoError(err)
	suite.Equal(hash1, hash3)
}
