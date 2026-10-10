// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

type ReconcileNetworkInternalsTestSuite struct {
	suite.Suite
}

func TestReconcileNetworkInternalsTestSuite(t *testing.T) {
	suite.Run(t, new(ReconcileNetworkInternalsTestSuite))
}

func (suite *ReconcileNetworkInternalsTestSuite) TestReconcileServiceGetErrorPropagates() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	wantErr := errors.New("simulated service get failure")
	r := newFakeReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*corev1.Service); ok {
				return wantErr
			}
			return c.Get(ctx, key, o, opts...)
		},
	}, instance)

	err := r.reconcileService(context.Background(), instance)
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *ReconcileNetworkInternalsTestSuite) TestReconcileServiceUpdatesOnPortDrift() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	existing := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "inst"},
			Ports:    []corev1.ServicePort{{Port: 9999, TargetPort: intstr.FromInt32(9999), Protocol: corev1.ProtocolTCP}},
		},
	}
	r := newFakeReconciler(instance, controlledBy(instance, existing))

	suite.Require().NoError(r.reconcileService(context.Background(), instance))

	svc := &corev1.Service{}
	suite.Require().NoError(r.Get(context.Background(), client.ObjectKey{Name: "inst", Namespace: "default"}, svc))
	suite.Equal(int32(8090), svc.Spec.Ports[0].Port, "drifted port must be corrected back to the default")
}

func (suite *ReconcileNetworkInternalsTestSuite) TestReconcileIngressWithTLSSecret() {
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec:       appsv1alpha1.ThunderIDInstanceSpec{Domain: "id.example.com", TLSSecret: "id-tls"},
	}
	r := newFakeReconciler(instance)

	suite.Require().NoError(r.reconcileIngress(context.Background(), instance))

	ing := &networkingv1.Ingress{}
	suite.Require().NoError(r.Get(context.Background(), client.ObjectKey{Name: "inst", Namespace: "default"}, ing))
	suite.Require().Len(ing.Spec.TLS, 1)
	suite.Equal("id-tls", ing.Spec.TLS[0].SecretName)
	suite.Equal([]string{"id.example.com"}, ing.Spec.TLS[0].Hosts)
}

func (suite *ReconcileNetworkInternalsTestSuite) TestReconcileIngressGetErrorPropagates() {
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec:       appsv1alpha1.ThunderIDInstanceSpec{Domain: "id.example.com"},
	}
	wantErr := errors.New("simulated ingress get failure")
	r := newFakeReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*networkingv1.Ingress); ok {
				return wantErr
			}
			return c.Get(ctx, key, o, opts...)
		},
	}, instance)

	err := r.reconcileIngress(context.Background(), instance)
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}

func (suite *ReconcileNetworkInternalsTestSuite) TestReconcileIngressUpdatesOnPortDrift() {
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec:       appsv1alpha1.ThunderIDInstanceSpec{Domain: "id.example.com"},
	}
	pathType := networkingv1.PathTypePrefix
	existing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: "id.example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{{
						Path: "/", PathType: &pathType,
						Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
							Name: "inst", Port: networkingv1.ServiceBackendPort{Number: 9999},
						}},
					}},
				}},
			}},
		},
	}
	r := newFakeReconciler(instance, controlledBy(instance, existing))

	suite.Require().NoError(r.reconcileIngress(context.Background(), instance))

	ing := &networkingv1.Ingress{}
	suite.Require().NoError(r.Get(context.Background(), client.ObjectKey{Name: "inst", Namespace: "default"}, ing))
	suite.Equal(int32(8090), ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number)
}

func (suite *ReconcileNetworkInternalsTestSuite) TestReconcileIngressUpdatesOnDomainChange() {
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec:       appsv1alpha1.ThunderIDInstanceSpec{Domain: "new.example.com"},
	}
	pathType := networkingv1.PathTypePrefix
	existing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: "old.example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{{
						Path: "/", PathType: &pathType,
						Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
							Name: "inst", Port: networkingv1.ServiceBackendPort{Number: 8090},
						}},
					}},
				}},
			}},
		},
	}
	r := newFakeReconciler(instance, controlledBy(instance, existing))

	suite.Require().NoError(r.reconcileIngress(context.Background(), instance))

	ing := &networkingv1.Ingress{}
	suite.Require().NoError(r.Get(context.Background(), client.ObjectKey{Name: "inst", Namespace: "default"}, ing))
	suite.Equal("new.example.com", ing.Spec.Rules[0].Host, "a domain change on an existing Ingress must be applied, not just port drift")
}

func (suite *ReconcileNetworkInternalsTestSuite) TestReconcileIngressAddsTLSToExisting() {
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec:       appsv1alpha1.ThunderIDInstanceSpec{Domain: "id.example.com", TLSSecret: "id-tls"},
	}
	pathType := networkingv1.PathTypePrefix
	existing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: networkingv1.IngressSpec{
			// No TLS block yet - as if spec.tlsSecret was unset when this Ingress was first created.
			Rules: []networkingv1.IngressRule{{
				Host: "id.example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{{
						Path: "/", PathType: &pathType,
						Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
							Name: "inst", Port: networkingv1.ServiceBackendPort{Number: 8090},
						}},
					}},
				}},
			}},
		},
	}
	r := newFakeReconciler(instance, controlledBy(instance, existing))

	suite.Require().NoError(r.reconcileIngress(context.Background(), instance))

	ing := &networkingv1.Ingress{}
	suite.Require().NoError(r.Get(context.Background(), client.ObjectKey{Name: "inst", Namespace: "default"}, ing))
	suite.Require().Len(ing.Spec.TLS, 1, "a tlsSecret added on an existing Ingress must be applied, not just port drift")
	suite.Equal("id-tls", ing.Spec.TLS[0].SecretName)
}

func (suite *ReconcileNetworkInternalsTestSuite) TestReconcileIngressDoesNotPanicOnMalformedRules() {
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec:       appsv1alpha1.ThunderIDInstanceSpec{Domain: "id.example.com"},
	}
	// An Ingress with no rules at all - e.g. hand-edited or left over from a race with another
	// controller. The old drift-check indexed Rules[0].HTTP.Paths[0] unconditionally and would
	// panic here; comparing the whole spec instead must not.
	existing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec:       networkingv1.IngressSpec{},
	}
	r := newFakeReconciler(instance, controlledBy(instance, existing))

	suite.NotPanics(func() {
		suite.Require().NoError(r.reconcileIngress(context.Background(), instance))
	})

	ing := &networkingv1.Ingress{}
	suite.Require().NoError(r.Get(context.Background(), client.ObjectKey{Name: "inst", Namespace: "default"}, ing))
	suite.Require().Len(ing.Spec.Rules, 1, "the malformed Ingress must be repaired back to the desired spec")
}

func (suite *ReconcileNetworkInternalsTestSuite) TestReconcileHPAGetErrorPropagates() {
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			AutoScaling: &appsv1alpha1.AutoScalingSpec{MinReplicas: 1, MaxReplicas: 3},
		},
	}
	wantErr := errors.New("simulated hpa get failure")
	r := newFakeReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, o client.Object, opts ...client.GetOption) error {
			if _, ok := o.(*autoscalingv2.HorizontalPodAutoscaler); ok {
				return wantErr
			}
			return c.Get(ctx, key, o, opts...)
		},
	}, instance)

	err := r.reconcileHPA(context.Background(), instance)
	suite.Require().Error(err)
	suite.ErrorIs(err, wantErr)
}
