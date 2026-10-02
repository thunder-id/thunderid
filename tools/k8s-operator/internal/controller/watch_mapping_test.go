// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	stderrors "errors"

	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

type WatchMappingTestSuite struct {
	suite.Suite
}

func TestWatchMappingTestSuite(t *testing.T) {
	suite.Run(t, new(WatchMappingTestSuite))
}

func (suite *WatchMappingTestSuite) TestInstancesForEnvSecretNoMatchingEnvironment() {
	r := newFakeReconciler()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "env-secret", Namespace: "default"}}

	reqs := r.instancesForEnvSecret(context.Background(), secret)
	suite.Empty(reqs)
}

func (suite *WatchMappingTestSuite) TestInstancesForEnvSecretUnlabeledEnvironmentMapsNothing() {
	env := &appsv1alpha1.EnvironmentValues{ObjectMeta: metav1.ObjectMeta{Name: "env-secret", Namespace: "default"}}
	r := newFakeReconciler(env)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "env-secret", Namespace: "default"}}

	reqs := r.instancesForEnvSecret(context.Background(), secret)
	suite.Empty(reqs)
}

func (suite *WatchMappingTestSuite) TestInstancesForEnvSecretMatchesByLabelOnSameNamedEnvironment() {
	env := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{
			Name: "env-secret", Namespace: "default",
			Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "matching"},
		},
	}
	r := newFakeReconciler(env)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "env-secret", Namespace: "default"}}

	reqs := r.instancesForEnvSecret(context.Background(), secret)
	suite.Require().Len(reqs, 1)
	suite.Equal("matching", reqs[0].Name)
}

func (suite *WatchMappingTestSuite) TestInstancesForEnvSecretGetErrorReturnsNil() {
	wantErr := stderrors.New("simulated get failure")
	r := newFakeReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*appsv1alpha1.EnvironmentValues); ok {
				return wantErr
			}
			return c.Get(ctx, key, o, opts...)
		},
	})
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "env-secret", Namespace: "default"}}

	reqs := r.instancesForEnvSecret(context.Background(), secret)
	suite.Nil(reqs)
}

func (suite *WatchMappingTestSuite) TestInstancesForEnvironmentUnlabeledMapsNothing() {
	r := newFakeReconciler()
	env := &appsv1alpha1.EnvironmentValues{ObjectMeta: metav1.ObjectMeta{Name: "env-secret", Namespace: "default"}}

	reqs := r.instancesForEnvironment(context.Background(), env)
	suite.Empty(reqs)
}

func (suite *WatchMappingTestSuite) TestInstancesForEnvironmentLabeledMapsToInstance() {
	r := newFakeReconciler()
	env := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{
			Name: "env-secret", Namespace: "default",
			Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
	}

	reqs := r.instancesForEnvironment(context.Background(), env)
	suite.Require().Len(reqs, 1)
	suite.Equal(types.NamespacedName{Name: "inst", Namespace: "default"}, reqs[0].NamespacedName)
}

func (suite *WatchMappingTestSuite) TestMapServingPodToInstanceIgnoresNonPodObjects() {
	r := newFakeReconciler()
	reqs := r.mapServingPodToInstance(context.Background(), &corev1.Secret{})
	suite.Nil(reqs)
}

func (suite *WatchMappingTestSuite) TestMapServingPodToInstanceIgnoresPodWithoutAppLabel() {
	r := newFakeReconciler()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"}}
	reqs := r.mapServingPodToInstance(context.Background(), pod)
	suite.Nil(reqs)
}

func (suite *WatchMappingTestSuite) TestMapServingPodToInstanceNoBootstrapJobJustMapsRequest() {
	r := newFakeReconciler()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default", Labels: map[string]string{"app": "inst"}}}

	reqs := r.mapServingPodToInstance(context.Background(), pod)
	suite.Require().Len(reqs, 1)
	suite.Equal(types.NamespacedName{Name: "inst", Namespace: "default"}, reqs[0].NamespacedName)
}

func (suite *WatchMappingTestSuite) TestMapServingPodToInstanceLeavesHealthyBootstrapJobAlone() {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-bootstrap-abc", Namespace: "default",
			Labels: map[string]string{"app": "inst", "thunderid.io/role": "bootstrap"},
		},
	}
	r := newFakeReconciler(job)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default", Labels: map[string]string{"app": "inst"}}}

	reqs := r.mapServingPodToInstance(context.Background(), pod)
	suite.Require().Len(reqs, 1)

	err := r.Get(context.Background(), types.NamespacedName{Name: "inst-bootstrap-abc", Namespace: "default"}, &batchv1.Job{})
	suite.NoError(err, "a Job that isn't Failed must not be deleted")
}

func (suite *WatchMappingTestSuite) TestMapServingPodToInstanceDeletesFailedBootstrapJob() {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-bootstrap-abc", Namespace: "default",
			Labels: map[string]string{"app": "inst", "thunderid.io/role": "bootstrap"},
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}},
		},
	}
	r := newFakeReconciler(job)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default", Labels: map[string]string{"app": "inst"}}}

	reqs := r.mapServingPodToInstance(context.Background(), pod)
	suite.Require().Len(reqs, 1, "still returns the reconcile request even though the Job is being cleaned up")

	err := r.Get(context.Background(), types.NamespacedName{Name: "inst-bootstrap-abc", Namespace: "default"}, &batchv1.Job{})
	suite.True(errors.IsNotFound(err), "a Failed bootstrap Job for the pod's instance must be deleted so the next reconcile retries fresh")
}

func (suite *WatchMappingTestSuite) TestBootstrapAttempt() {
	cases := []struct {
		name  string
		label string
		want  int
	}{
		{"missing label defaults to 1", "", 1},
		{"parses a valid attempt", "3", 3},
		{"unparseable falls back to 1", "abc", 1},
		{"zero falls back to 1", "0", 1},
		{"negative falls back to 1", "-2", 1},
	}
	for _, c := range cases {
		suite.Run(c.name, func() {
			job := &batchv1.Job{}
			if c.label != "" {
				job.Labels = map[string]string{bootstrapAttemptLabel: c.label}
			}
			suite.Equal(c.want, bootstrapAttempt(job))
		})
	}
}

func (suite *WatchMappingTestSuite) TestBootstrapJobFailed() {
	cases := []struct {
		name string
		job  *batchv1.Job
		want bool
	}{
		{"no conditions", &batchv1.Job{}, false},
		{"failed true", &batchv1.Job{Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{
			{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
		}}}, true},
		{"failed false", &batchv1.Job{Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{
			{Type: batchv1.JobFailed, Status: corev1.ConditionFalse},
		}}}, false},
		{"complete, not failed", &batchv1.Job{Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{
			{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
		}}}, false},
	}
	for _, c := range cases {
		suite.Run(c.name, func() {
			suite.Equal(c.want, bootstrapJobFailed(c.job))
		})
	}
}
