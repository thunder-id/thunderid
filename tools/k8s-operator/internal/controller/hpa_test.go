// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

type HPATestSuite struct {
	suite.Suite
}

func TestHPATestSuite(t *testing.T) {
	suite.Run(t, new(HPATestSuite))
}

func (suite *HPATestSuite) TestBuildMetricsCPUOnly() {
	metrics := buildMetrics(&appsv1alpha1.AutoScalingSpec{TargetCPUUtilizationPercentage: 70})
	suite.Require().Len(metrics, 1)
	suite.Equal(corev1.ResourceCPU, metrics[0].Resource.Name)
	suite.Equal(int32(70), *metrics[0].Resource.Target.AverageUtilization)
}

func (suite *HPATestSuite) TestBuildMetricsMemoryOnly() {
	metrics := buildMetrics(&appsv1alpha1.AutoScalingSpec{TargetMemoryUtilizationPercentage: 65})
	suite.Require().Len(metrics, 1)
	suite.Equal(corev1.ResourceMemory, metrics[0].Resource.Name)
	suite.Equal(int32(65), *metrics[0].Resource.Target.AverageUtilization)
}

func (suite *HPATestSuite) TestBuildMetricsBoth() {
	metrics := buildMetrics(&appsv1alpha1.AutoScalingSpec{
		TargetCPUUtilizationPercentage:    70,
		TargetMemoryUtilizationPercentage: 65,
	})
	suite.Require().Len(metrics, 2)
}

func (suite *HPATestSuite) TestBuildMetricsDefaultsToEightyPercentCPU() {
	metrics := buildMetrics(&appsv1alpha1.AutoScalingSpec{})
	suite.Require().Len(metrics, 1)
	suite.Equal(corev1.ResourceCPU, metrics[0].Resource.Name)
	suite.Equal(int32(80), *metrics[0].Resource.Target.AverageUtilization)
}

func (suite *HPATestSuite) TestReconcileHPANilAutoScalingNoExistingIsNoop() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	r := newFakeReconciler(instance)
	suite.NoError(r.reconcileHPA(context.Background(), instance))
}

func (suite *HPATestSuite) TestReconcileHPANilAutoScalingDeletesExisting() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	existing := controlledBy(instance, &autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}})
	r := newFakeReconciler(instance, existing)

	suite.Require().NoError(r.reconcileHPA(context.Background(), instance))

	err := r.Get(context.Background(), types.NamespacedName{Name: "inst", Namespace: "default"}, &autoscalingv2.HorizontalPodAutoscaler{})
	suite.Error(err, "HPA should have been deleted once autoscaling was turned off")
}

func (suite *HPATestSuite) TestReconcileHPACreatesWhenMissing() {
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			AutoScaling: &appsv1alpha1.AutoScalingSpec{MinReplicas: 2, MaxReplicas: 5},
		},
	}
	r := newFakeReconciler(instance)

	suite.Require().NoError(r.reconcileHPA(context.Background(), instance))

	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst", Namespace: "default"}, hpa))
	suite.Equal(int32(2), *hpa.Spec.MinReplicas)
	suite.Equal(int32(5), hpa.Spec.MaxReplicas)
	suite.Equal("inst", hpa.Spec.ScaleTargetRef.Name)
	suite.Equal("Deployment", hpa.Spec.ScaleTargetRef.Kind)
}

func (suite *HPATestSuite) TestReconcileHPAPatchesWhenSpecDrifted() {
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			AutoScaling: &appsv1alpha1.AutoScalingSpec{MinReplicas: 2, MaxReplicas: 5},
		},
	}
	oldMax := int32(3)
	existing := controlledBy(instance, &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{APIVersion: "apps/v1", Kind: "Deployment", Name: "inst"},
			MinReplicas:    &oldMax,
			MaxReplicas:    3,
		},
	})
	r := newFakeReconciler(instance, existing)

	suite.Require().NoError(r.reconcileHPA(context.Background(), instance))

	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst", Namespace: "default"}, hpa))
	suite.Equal(int32(5), hpa.Spec.MaxReplicas, "drifted MaxReplicas must be patched back to spec")
}

func (suite *HPATestSuite) TestReconcileHPAIdempotentWhenSpecMatches() {
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			AutoScaling: &appsv1alpha1.AutoScalingSpec{MinReplicas: 2, MaxReplicas: 5},
		},
	}
	r := newFakeReconciler(instance)
	suite.Require().NoError(r.reconcileHPA(context.Background(), instance))

	var before autoscalingv2.HorizontalPodAutoscaler
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst", Namespace: "default"}, &before))

	suite.Require().NoError(r.reconcileHPA(context.Background(), instance))

	var after autoscalingv2.HorizontalPodAutoscaler
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst", Namespace: "default"}, &after))
	suite.Equal(before.ResourceVersion, after.ResourceVersion, "unchanged spec must not trigger a patch")
}
