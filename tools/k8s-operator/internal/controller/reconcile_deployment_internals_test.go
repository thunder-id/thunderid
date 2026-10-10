// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

type ReconcileDeploymentInternalsTestSuite struct {
	suite.Suite
}

func TestReconcileDeploymentInternalsTestSuite(t *testing.T) {
	suite.Run(t, new(ReconcileDeploymentInternalsTestSuite))
}

func (suite *ReconcileDeploymentInternalsTestSuite) TestEnvironmentLabelPopulatesEnvFrom() {
	instance := baselineSQLiteInstance()
	env := &appsv1alpha1.EnvironmentValues{
		ObjectMeta: metav1.ObjectMeta{
			Name: "env-secret", Namespace: "default",
			Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
	}
	r := newFakeReconciler(instance, env)

	suite.Require().NoError(r.reconcileDeployment(context.Background(), instance, "hash1", false, "inst-security"))

	dep := &appsv1.Deployment{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst", Namespace: "default"}, dep))
	suite.Require().Len(dep.Spec.Template.Spec.Containers[0].EnvFrom, 1)
	suite.Equal("env-secret", dep.Spec.Template.Spec.Containers[0].EnvFrom[0].SecretRef.Name)
}

func (suite *ReconcileDeploymentInternalsTestSuite) TestSQLiteWithResourcesUsesSetupAndResourcesArg() {
	instance := baselineSQLiteInstance()
	r := newFakeReconciler(instance)

	suite.Require().NoError(r.reconcileDeployment(context.Background(), instance, "hash1", true, "inst-security"))

	dep := &appsv1.Deployment{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst", Namespace: "default"}, dep))
	cmd := dep.Spec.Template.Spec.Containers[0].Command[2]
	suite.Contains(cmd, "setup.sh")
	suite.Contains(cmd, "start.sh /opt/thunderid/resources.yaml")
}

func (suite *ReconcileDeploymentInternalsTestSuite) TestPostgresWithResourcesUsesPlainServeArg() {
	instance := postgresInstance()
	r := newFakeReconciler(instance)

	suite.Require().NoError(r.reconcileDeployment(context.Background(), instance, "hash1", true, "inst-security"))

	dep := &appsv1.Deployment{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst", Namespace: "default"}, dep))
	cmd := dep.Spec.Template.Spec.Containers[0].Command[2]
	suite.NotContains(cmd, "setup.sh")
	suite.Contains(cmd, "start.sh /opt/thunderid/resources.yaml")

	var foundResourcesMount bool
	for _, vm := range dep.Spec.Template.Spec.Containers[0].VolumeMounts {
		if vm.MountPath == "/opt/thunderid/resources.yaml" {
			foundResourcesMount = true
		}
	}
	suite.True(foundResourcesMount)
}

func (suite *ReconcileDeploymentInternalsTestSuite) TestEmailAuthMountsEmailVolume() {
	instance := baselineSQLiteInstance()
	instance.Spec.Config.Email = &appsv1alpha1.EmailSpec{
		SMTP: appsv1alpha1.SMTPConfigSpec{
			Host: "smtp.example.com", Port: 587, FromAddress: "noreply@example.com",
			EnableAuthentication: true,
			SecretRef:            &appsv1alpha1.SecretKeyRef{Name: "email-secret", Key: "password"},
		},
	}
	r := newFakeReconciler(instance)

	suite.Require().NoError(r.reconcileDeployment(context.Background(), instance, "hash1", false, "inst-security"))

	dep := &appsv1.Deployment{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst", Namespace: "default"}, dep))

	var foundMount, foundVolume bool
	for _, vm := range dep.Spec.Template.Spec.Containers[0].VolumeMounts {
		if vm.Name == "email" {
			foundMount = true
		}
	}
	for _, v := range dep.Spec.Template.Spec.Volumes {
		if v.Name == "email" {
			foundVolume = true
		}
	}
	suite.True(foundMount)
	suite.True(foundVolume)
}

func (suite *ReconcileDeploymentInternalsTestSuite) TestInitialGetErrorPropagates() {
	instance := baselineSQLiteInstance()
	wantErr := errors.New("simulated deployment get failure")
	r := newFakeReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*appsv1.Deployment); ok {
				return wantErr
			}
			return c.Get(ctx, key, o, opts...)
		},
	}, instance)

	err := r.reconcileDeployment(context.Background(), instance, "hash1", false, "inst-security")
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *ReconcileDeploymentInternalsTestSuite) TestStaticReplicasUpdatedOnDrift() {
	instance := baselineSQLiteInstance()
	instance.Spec.Replicas = 3
	r := newFakeReconciler(instance)

	suite.Require().NoError(r.reconcileDeployment(context.Background(), instance, "hash1", false, "inst-security"))

	instance.Spec.Replicas = 5
	suite.Require().NoError(r.reconcileDeployment(context.Background(), instance, "hash1", false, "inst-security"))

	dep := &appsv1.Deployment{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst", Namespace: "default"}, dep))
	suite.Equal(int32(5), *dep.Spec.Replicas)
}

func (suite *ReconcileDeploymentInternalsTestSuite) TestNilTemplateAnnotationsInitialized() {
	instance := baselineSQLiteInstance()
	// Simulates a Deployment created before this operator started stamping the config-hash
	// annotation, or otherwise missing Template.Annotations entirely.
	existing := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "inst"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "inst"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "thunderid", Image: "old-image"}},
				},
			},
		},
	}
	r := newFakeReconciler(instance, controlledBy(instance, existing))

	suite.Require().NoError(r.reconcileDeployment(context.Background(), instance, "hash1", false, "inst-security"))

	dep := &appsv1.Deployment{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst", Namespace: "default"}, dep))
	suite.Equal("hash1", dep.Spec.Template.Annotations["thunderid.io/config-hash"])
}

// An instance named after a Deployment it didn't create must not take that Deployment over.
func (suite *ReconcileDeploymentInternalsTestSuite) TestForeignDeploymentIsNotTakenOver() {
	instance := baselineSQLiteInstance()
	foreign := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "other"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "other"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "other", Image: "someone-elses-image"}},
				},
			},
		},
	}
	r := newFakeReconciler(instance, foreign)
	recorder := record.NewFakeRecorder(5)
	r.Recorder = recorder

	err := r.reconcileDeployment(context.Background(), instance, "hash1", false, "inst-security")
	suite.Require().Error(err)

	dep := &appsv1.Deployment{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst", Namespace: "default"}, dep))
	suite.Equal("someone-elses-image", dep.Spec.Template.Spec.Containers[0].Image, "a foreign Deployment must be left untouched")
	suite.Contains(<-recorder.Events, reasonOwnershipConflict)
}

func (suite *ReconcileDeploymentInternalsTestSuite) TestUpdateRetryGetErrorPropagates() {
	instance := baselineSQLiteInstance()
	r := newFakeReconciler(instance)
	suite.Require().NoError(r.reconcileDeployment(context.Background(), instance, "hash1", false, "inst-security"))

	wantErr := errors.New("simulated retry-get failure")
	getCount := 0
	// Wraps the SAME already-populated client from the first (successful) call above, rather
	// than seeding a fresh one - interceptor.NewClient is built for exactly this: layering a
	// call-failure onto an existing client without rebuilding its state.
	interceptedClient := interceptor.NewClient(r.Client.(client.WithWatch), interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*appsv1.Deployment); ok {
				getCount++
				if getCount == 2 {
					return wantErr
				}
			}
			return c.Get(ctx, key, o, opts...)
		},
	})
	r2 := &ThunderIDInstanceReconciler{Client: interceptedClient, Scheme: r.Scheme}

	// hash2 forces a real diff so the Update path (and its retry-closure Get) actually runs.
	err := r2.reconcileDeployment(context.Background(), instance, "hash2", false, "inst-security")
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}
