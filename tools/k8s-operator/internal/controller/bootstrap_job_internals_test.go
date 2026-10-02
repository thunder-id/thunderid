// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

type BootstrapJobInternalsTestSuite struct {
	suite.Suite
}

func TestBootstrapJobInternalsTestSuite(t *testing.T) {
	suite.Run(t, new(BootstrapJobInternalsTestSuite))
}

func (suite *BootstrapJobInternalsTestSuite) TestReconcileBootstrapJobInitialGetErrorPropagates() {
	instance := postgresInstance()
	wantErr := errors.New("simulated job get failure")
	r := newFakeReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*batchv1.Job); ok {
				return wantErr
			}
			return c.Get(ctx, key, o, opts...)
		},
	}, instance)

	err := r.reconcileBootstrapJob(context.Background(), instance, "seed1", "inst-security")
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *BootstrapJobInternalsTestSuite) TestReconcileBootstrapJobRetryDeleteErrorPropagates() {
	instance := postgresInstance()
	failedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-bootstrap-seed1", Namespace: "default",
			Labels: map[string]string{"app": "inst", "thunderid.io/role": "bootstrap", bootstrapAttemptLabel: "1"},
		},
		Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}},
	}
	wantErr := errors.New("simulated delete failure")
	r := newFakeReconcilerWithInterceptor(interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.DeleteOption) error {
			if _, ok := o.(*batchv1.Job); ok {
				return wantErr
			}
			return c.Delete(ctx, o, opts...)
		},
	}, instance, failedJob)
	r.KubeClient = k8sfake.NewSimpleClientset() // no matching pod -> not a permanent failure, tries to delete+retry

	err := r.reconcileBootstrapJob(context.Background(), instance, "seed1", "inst-security")
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *BootstrapJobInternalsTestSuite) TestReconcileBootstrapJobStaleListErrorPropagates() {
	instance := postgresInstance()
	wantErr := errors.New("simulated list failure")
	r := newFakeReconcilerWithInterceptor(interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, l client.ObjectList, opts ...client.ListOption) error {
			if _, ok := l.(*batchv1.JobList); ok {
				return wantErr
			}
			return c.List(ctx, l, opts...)
		},
	}, instance)

	err := r.reconcileBootstrapJob(context.Background(), instance, "seed1", "inst-security")
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *BootstrapJobInternalsTestSuite) TestReconcileBootstrapJobStaleDeleteErrorPropagates() {
	instance := postgresInstance()
	staleJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: "inst-bootstrap-old-seed", Namespace: "default",
		Labels: map[string]string{"app": "inst", "thunderid.io/role": "bootstrap", bootstrapAttemptLabel: "1"},
	}}
	wantErr := errors.New("simulated delete failure")
	r := newFakeReconcilerWithInterceptor(interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.DeleteOption) error {
			if _, ok := o.(*batchv1.Job); ok {
				return wantErr
			}
			return c.Delete(ctx, o, opts...)
		},
	}, instance, staleJob)

	err := r.reconcileBootstrapJob(context.Background(), instance, "new-seed", "inst-security")
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *BootstrapJobInternalsTestSuite) TestBootstrapPermanentFailureReasonStreamErrorReturnsEmpty() {
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: "j", Namespace: "default",
		Labels: map[string]string{"app": "inst", "thunderid.io/role": "bootstrap", bootstrapAttemptLabel: "1"},
	}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "j-pod", Namespace: "default",
		Labels: map[string]string{"app": "inst", "thunderid.io/role": "bootstrap", bootstrapAttemptLabel: "1"},
	}}
	r := newFakeReconciler()
	cs := k8sfake.NewSimpleClientset(pod)
	cs.PrependReactor("get", "pods", podLogsReactor("", errStreamFailed))
	r.KubeClient = cs

	suite.Empty(r.bootstrapPermanentFailureReason(context.Background(), job))
}

func (suite *BootstrapJobInternalsTestSuite) TestCreateBootstrapJobEmailAuthMountsEmailVolume() {
	instance := postgresInstance()
	instance.Spec.Config.Email = &appsv1alpha1.EmailSpec{
		SMTP: appsv1alpha1.SMTPConfigSpec{
			Host: "smtp.example.com", Port: 587, FromAddress: "noreply@example.com",
			EnableAuthentication: true,
			SecretRef:            &appsv1alpha1.SecretKeyRef{Name: "email-secret", Key: "password"},
		},
	}
	r := newFakeReconciler(instance)

	suite.Require().NoError(r.createBootstrapJob(context.Background(), instance, "inst-bootstrap-seed1", "inst-security", 1))

	job := &batchv1.Job{}
	suite.Require().NoError(r.Get(context.Background(), client.ObjectKey{Name: "inst-bootstrap-seed1", Namespace: "default"}, job))
	container := job.Spec.Template.Spec.Containers[0]

	var foundMount, foundVolume bool
	for _, vm := range container.VolumeMounts {
		if vm.Name == "email" {
			foundMount = true
		}
	}
	for _, v := range job.Spec.Template.Spec.Volumes {
		if v.Name == "email" {
			foundVolume = true
			suite.Equal("email-secret", v.Secret.SecretName)
		}
	}
	suite.True(foundMount, "expected an email VolumeMount")
	suite.True(foundVolume, "expected an email Volume")
}
