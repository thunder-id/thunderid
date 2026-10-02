// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

// Shared fixtures for every *_test.go file in this package that only needs an in-memory fake
// client (no real API server / envtest) - fast, no external binaries, one scheme builder per
// reconciler type instead of each test file inventing its own.

func newFakeReconcilerScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = autoscalingv2.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = networkingv1.AddToScheme(scheme)
	_ = appsv1alpha1.AddToScheme(scheme)
	return scheme
}

func newFakeReconciler(objs ...runtime.Object) *ThunderIDInstanceReconciler {
	scheme := newFakeReconcilerScheme()
	return &ThunderIDInstanceReconciler{
		// WithStatusSubresource is required for a CRD type: without it the fake client's
		// Status().Update() doesn't track the status subresource separately and errors instead
		// of applying the update.
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appsv1alpha1.ThunderIDInstance{}).
			WithRuntimeObjects(objs...).Build(),
		Scheme: scheme,
	}
}

// newFakeReconcilerWithInterceptor is newFakeReconciler plus the ability to make a specific
// client call fail on command (see sigs.k8s.io/controller-runtime/pkg/client/interceptor) — for
// exercising a Reconcile branch that only fires when a write genuinely fails, which the plain
// fake client never does on its own for a well-formed request.
func newFakeReconcilerWithInterceptor(funcs interceptor.Funcs, objs ...runtime.Object) *ThunderIDInstanceReconciler {
	scheme := newFakeReconcilerScheme()
	return &ThunderIDInstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appsv1alpha1.ThunderIDInstance{}).
			WithInterceptorFuncs(funcs).WithRuntimeObjects(objs...).Build(),
		Scheme: scheme,
	}
}

func newFakeResourceReconcilerScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = appsv1alpha1.AddToScheme(scheme)
	return scheme
}

func newFakeThunderIDResourceReconciler(objs ...runtime.Object) *ThunderIDResourceReconciler {
	scheme := newFakeResourceReconcilerScheme()
	return &ThunderIDResourceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appsv1alpha1.ThunderIDResource{}).
			WithRuntimeObjects(objs...).Build(),
		Scheme: scheme,
	}
}

// newFakeThunderIDResourceReconcilerWithInterceptor is newFakeThunderIDResourceReconciler plus the ability to make
// a specific client call fail on command — see newFakeReconcilerWithInterceptor's doc comment.
func newFakeThunderIDResourceReconcilerWithInterceptor(funcs interceptor.Funcs, objs ...runtime.Object) *ThunderIDResourceReconciler {
	scheme := newFakeResourceReconcilerScheme()
	return &ThunderIDResourceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appsv1alpha1.ThunderIDResource{}).
			WithInterceptorFuncs(funcs).WithRuntimeObjects(objs...).Build(),
		Scheme: scheme,
	}
}

func newFakeEnvironmentReconcilerScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1alpha1.AddToScheme(scheme)
	return scheme
}

func newFakeEnvironmentReconciler(objs ...runtime.Object) *EnvironmentValuesReconciler {
	scheme := newFakeEnvironmentReconcilerScheme()
	return &EnvironmentValuesReconciler{
		// WithStatusSubresource is required for a CRD type: without it the fake client's
		// Status().Update() doesn't track the status subresource separately and errors instead
		// of applying the update.
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appsv1alpha1.EnvironmentValues{}).
			WithRuntimeObjects(objs...).Build(),
		Scheme: scheme,
	}
}

// newFakeEnvironmentReconcilerWithInterceptor is newFakeEnvironmentReconciler plus the ability to
// make a specific client call fail on command — see newFakeReconcilerWithInterceptor's doc comment.
func newFakeEnvironmentReconcilerWithInterceptor(funcs interceptor.Funcs, objs ...runtime.Object) *EnvironmentValuesReconciler {
	scheme := newFakeEnvironmentReconcilerScheme()
	return &EnvironmentValuesReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appsv1alpha1.EnvironmentValues{}).
			WithInterceptorFuncs(funcs).WithRuntimeObjects(objs...).Build(),
		Scheme: scheme,
	}
}

func fakeRecorder() record.EventRecorder {
	return record.NewFakeRecorder(20)
}

func reconcileRequest(name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: "default"}}
}

func rawSpec(json string) runtime.RawExtension {
	return runtime.RawExtension{Raw: []byte(json)}
}
