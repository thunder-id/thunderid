//go:build integration

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

var _ = Describe("ThunderIDInstance Controller", func() {
	const resourceNamespace = "default"

	var (
		resourceName       string
		typeNamespacedName types.NamespacedName
	)

	BeforeEach(func() {
		resourceName = fmt.Sprintf("test-instance-%d", GinkgoRandomSeed())
		typeNamespacedName = types.NamespacedName{Name: resourceName, Namespace: resourceNamespace}
	})

	AfterEach(func() {
		resource := &appsv1alpha1.ThunderIDInstance{}
		if err := k8sClient.Get(ctx, typeNamespacedName, resource); err == nil {
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		}
	})

	newReconciler := func() *ThunderIDInstanceReconciler {
		return &ThunderIDInstanceReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Recorder: record.NewFakeRecorder(20),
		}
	}

	Context("when spec.dataVolumeSize is set (SQLite, PVC-backed)", func() {
		BeforeEach(func() {
			instance := &appsv1alpha1.ThunderIDInstance{
				ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: resourceNamespace},
				Spec: appsv1alpha1.ThunderIDInstanceSpec{
					Image:          "thunderid:latest",
					Domain:         "test.example.com",
					DataVolumeSize: "1Gi",
				},
			}
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())
		})

		It("reconciles the full set of owned objects and reports Running", func() {
			r := newReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			cm := &corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName + "-config", Namespace: resourceNamespace}, cm)).To(Succeed())
			Expect(cm.Data).To(HaveKey("deployment.yaml"))

			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, dep)).To(Succeed())
			Expect(dep.Spec.Template.Spec.Containers[0].Image).To(Equal("thunderid:latest"))
			Expect(dep.Spec.Strategy.Type).To(Equal(appsv1.RecreateDeploymentStrategyType))

			svc := &corev1.Service{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, svc)).To(Succeed())

			ing := &networkingv1.Ingress{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, ing)).To(Succeed())
			Expect(ing.Spec.Rules[0].Host).To(Equal("test.example.com"))

			pvc := &corev1.PersistentVolumeClaim{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName + "-data", Namespace: resourceNamespace}, pvc)).To(Succeed())

			updated := &appsv1alpha1.ThunderIDInstance{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())
			Expect(updated.Status.Phase).To(Equal("Running"))
			Expect(updated.Status.URL).To(Equal("https://test.example.com"))
		})

		It("is idempotent: a second reconcile makes no further Deployment update", func() {
			r := newReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			var before appsv1.Deployment
			Expect(k8sClient.Get(ctx, typeNamespacedName, &before)).To(Succeed())

			_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			var after appsv1.Deployment
			Expect(k8sClient.Get(ctx, typeNamespacedName, &after)).To(Succeed())
			Expect(after.ResourceVersion).To(Equal(before.ResourceVersion))
		})

		It("cleans up a stale bootstrap volume and a stale security volume reference", func() {
			// Reconcile diffs the full Volumes list against the desired state, so a Deployment
			// carrying stale bootstrap/security volumes self-heals instead of staying wedged.
			r := newReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			var dep appsv1.Deployment
			Expect(k8sClient.Get(ctx, typeNamespacedName, &dep)).To(Succeed())
			dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, corev1.Volume{
				Name:         "bootstrap",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			})
			for i, v := range dep.Spec.Template.Spec.Volumes {
				if v.Name == "security" {
					dep.Spec.Template.Spec.Volumes[i].Secret.SecretName = "some-other-secret"
				}
			}
			Expect(k8sClient.Update(ctx, &dep)).To(Succeed())

			_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			var healed appsv1.Deployment
			Expect(k8sClient.Get(ctx, typeNamespacedName, &healed)).To(Succeed())
			names := map[string]bool{}
			for _, v := range healed.Spec.Template.Spec.Volumes {
				names[v.Name] = true
				if v.Name == "security" {
					Expect(v.Secret.SecretName).To(Equal(resourceName + "-security"))
				}
			}
			Expect(names).NotTo(HaveKey("bootstrap"))
		})

		It("removes the Ingress once spec.domain is cleared", func() {
			r := newReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			instance := &appsv1alpha1.ThunderIDInstance{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, instance)).To(Succeed())
			instance.Spec.Domain = ""
			Expect(k8sClient.Update(ctx, instance)).To(Succeed())

			_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			ing := &networkingv1.Ingress{}
			err = k8sClient.Get(ctx, typeNamespacedName, ing)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})

	Context("when postgres is configured (no PVC)", func() {
		BeforeEach(func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: resourceName + "-db", Namespace: resourceNamespace},
				StringData: map[string]string{"password": "hunter2"},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			pg := appsv1alpha1.DatabaseBackendSpec{
				Type: "postgres",
				Postgres: &appsv1alpha1.PostgresBackendSpec{
					Hostname: "db.example.com", Username: "thunderid", Name: "thunderid",
					PasswordRef: &appsv1alpha1.SecretKeyRef{Name: resourceName + "-db", Key: "password"},
				},
			}
			instance := &appsv1alpha1.ThunderIDInstance{
				ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: resourceNamespace},
				Spec: appsv1alpha1.ThunderIDInstanceSpec{
					Image: "thunderid:latest",
					Config: appsv1alpha1.AppConfigSpec{
						Database: &appsv1alpha1.DatabaseSpec{
							Config: pg, RuntimeTransient: pg, Entity: pg, RuntimePersistent: pg,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())
		})

		AfterEach(func() {
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: resourceName + "-db", Namespace: resourceNamespace}}
			_ = k8sClient.Delete(ctx, secret)
		})

		It("is idempotent: a second reconcile makes no further Deployment update", func() {
			// This is the path reconcileDeployment leaves Strategy at its Go zero value (only
			// the PVC/SQLite path sets it explicitly to Recreate) — the real API server defaults
			// that to the fully-defaulted RollingUpdate on write, which is exactly the case
			// normalizeDeploymentStrategyForDiff exists to compare correctly instead of forcing
			// an update (and thus an immediate re-reconcile) every single time.
			r := newReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			var before appsv1.Deployment
			Expect(k8sClient.Get(ctx, typeNamespacedName, &before)).To(Succeed())
			Expect(before.Spec.Strategy.Type).To(Equal(appsv1.RollingUpdateDeploymentStrategyType))

			_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			var after appsv1.Deployment
			Expect(k8sClient.Get(ctx, typeNamespacedName, &after)).To(Succeed())
			Expect(after.ResourceVersion).To(Equal(before.ResourceVersion))
		})
	})

	Context("when neither dataVolumeSize nor postgres is configured", func() {
		BeforeEach(func() {
			instance := &appsv1alpha1.ThunderIDInstance{
				ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: resourceNamespace},
				Spec: appsv1alpha1.ThunderIDInstanceSpec{
					Image: "thunderid:latest",
				},
			}
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())
		})

		It("rejects the spec with an error instead of panicking", func() {
			r := newReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.dataVolumeSize must be set"))
		})
	})

	Context("when the instance no longer exists", func() {
		It("returns cleanly without error", func() {
			r := newReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "does-not-exist", Namespace: resourceNamespace},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
