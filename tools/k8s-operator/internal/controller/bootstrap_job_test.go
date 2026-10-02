// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

func postgresInstance() *appsv1alpha1.ThunderIDInstance {
	pg := appsv1alpha1.DatabaseBackendSpec{
		Type: "postgres", Postgres: &appsv1alpha1.PostgresBackendSpec{
			Hostname: "h", Username: "u", Name: "n",
			PasswordRef: &appsv1alpha1.SecretKeyRef{Name: "db-secret", Key: "password"},
		},
	}
	return &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			Image: "thunderid:latest",
			Config: appsv1alpha1.AppConfigSpec{Database: &appsv1alpha1.DatabaseSpec{
				Config: pg, RuntimeTransient: pg, Entity: pg, RuntimePersistent: pg,
			}},
		},
	}
}

type BootstrapJobTestSuite struct {
	suite.Suite
}

func TestBootstrapJobTestSuite(t *testing.T) {
	suite.Run(t, new(BootstrapJobTestSuite))
}

func (suite *BootstrapJobTestSuite) TestReconcileBootstrapJobSkippedWithoutPostgres() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	r := newFakeReconciler(instance)
	suite.NoError(r.reconcileBootstrapJob(context.Background(), instance, "seed1", "inst-security"))

	var jobs batchv1.JobList
	suite.Require().NoError(r.List(context.Background(), &jobs))
	suite.Empty(jobs.Items, "no Job should be created when not using postgres")
}

func (suite *BootstrapJobTestSuite) TestReconcileBootstrapJobStillRunningIsLeftAlone() {
	instance := postgresInstance()
	existing := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: "inst-bootstrap-seed1", Namespace: "default",
		Labels: map[string]string{"app": "inst", "thunderid.io/role": "bootstrap", bootstrapAttemptLabel: "1"},
	}}
	r := newFakeReconciler(instance, existing)

	suite.Require().NoError(r.reconcileBootstrapJob(context.Background(), instance, "seed1", "inst-security"))

	var after batchv1.Job
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst-bootstrap-seed1", Namespace: "default"}, &after))
	suite.Equal(existing.ResourceVersion, after.ResourceVersion, "a still-running Job must not be touched")
}

func (suite *BootstrapJobTestSuite) TestReconcileBootstrapJobPermanentFailureStopsRetrying() {
	instance := postgresInstance()
	failedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-bootstrap-seed1", Namespace: "default",
			Labels: map[string]string{"app": "inst", "thunderid.io/role": "bootstrap", bootstrapAttemptLabel: "1"},
		},
		Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}},
	}
	bootstrapPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "inst-bootstrap-seed1-xyz", Namespace: "default",
		Labels: map[string]string{"app": "inst", "thunderid.io/role": "bootstrap", bootstrapAttemptLabel: "1"},
	}}
	r := newFakeReconciler(instance, failedJob)
	fr := record.NewFakeRecorder(20)
	r.Recorder = fr
	cs := k8sfake.NewSimpleClientset(bootstrapPod)
	cs.PrependReactor("get", "pods", podLogsReactor(`pq: password authentication failed for user "u"`, nil))
	r.KubeClient = cs

	suite.Require().NoError(r.reconcileBootstrapJob(context.Background(), instance, "seed1", "inst-security"))

	suite.Require().NotEmpty(fr.Events)
	suite.Contains(<-fr.Events, "will not be retried automatically")

	var still batchv1.Job
	suite.NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst-bootstrap-seed1", Namespace: "default"}, &still),
		"a permanently-failed Job is left in place for inspection, not deleted")
}

func (suite *BootstrapJobTestSuite) TestReconcileBootstrapJobRetryLimitReached() {
	instance := postgresInstance()
	failedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-bootstrap-seed1", Namespace: "default",
			Labels: map[string]string{"app": "inst", "thunderid.io/role": "bootstrap", bootstrapAttemptLabel: "3"},
		},
		Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}},
	}
	r := newFakeReconciler(instance, failedJob)
	fr := record.NewFakeRecorder(20)
	r.Recorder = fr
	r.KubeClient = k8sfake.NewSimpleClientset() // no matching pod -> bootstrapPermanentFailureReason returns ""

	suite.Require().NoError(r.reconcileBootstrapJob(context.Background(), instance, "seed1", "inst-security"))

	suite.Require().NotEmpty(fr.Events)
	suite.Contains(<-fr.Events, "reached the retry limit")
}

func (suite *BootstrapJobTestSuite) TestReconcileBootstrapJobRetriesBelowLimit() {
	instance := postgresInstance()
	failedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-bootstrap-seed1", Namespace: "default",
			Labels: map[string]string{"app": "inst", "thunderid.io/role": "bootstrap", bootstrapAttemptLabel: "1"},
		},
		Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}},
	}
	r := newFakeReconciler(instance, failedJob)
	r.KubeClient = k8sfake.NewSimpleClientset() // no matching pod -> not a permanent failure

	suite.Require().NoError(r.reconcileBootstrapJob(context.Background(), instance, "seed1", "inst-security"))

	var jobs batchv1.JobList
	suite.Require().NoError(r.List(context.Background(), &jobs))
	suite.Require().Len(jobs.Items, 1, "old Job deleted, exactly one replacement created")
	suite.Equal("2", jobs.Items[0].Labels[bootstrapAttemptLabel])
}

func (suite *BootstrapJobTestSuite) TestReconcileBootstrapJobHashChangeReplacesStaleJob() {
	instance := postgresInstance()
	staleJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: "inst-bootstrap-old-seed", Namespace: "default",
		Labels: map[string]string{"app": "inst", "thunderid.io/role": "bootstrap", bootstrapAttemptLabel: "1"},
	}}
	r := newFakeReconciler(instance, staleJob)

	suite.Require().NoError(r.reconcileBootstrapJob(context.Background(), instance, "new-seed", "inst-security"))

	var jobs batchv1.JobList
	suite.Require().NoError(r.List(context.Background(), &jobs))
	suite.Require().Len(jobs.Items, 1)
	suite.Equal("inst-bootstrap-new-seed", jobs.Items[0].Name)
}

func (suite *BootstrapJobTestSuite) TestBootstrapPermanentFailureReasonNoMatchingPod() {
	instance := postgresInstance()
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: "j", Namespace: "default",
		Labels: map[string]string{"app": "inst", bootstrapAttemptLabel: "1"},
	}}
	r := newFakeReconciler(instance)
	r.KubeClient = k8sfake.NewSimpleClientset()

	suite.Empty(r.bootstrapPermanentFailureReason(context.Background(), job))
}

func (suite *BootstrapJobTestSuite) TestBootstrapPermanentFailureReasonNoPatternMatch() {
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
	cs.PrependReactor("get", "pods", podLogsReactor("some transient timeout, nothing recognizable", nil))
	r.KubeClient = cs

	suite.Empty(r.bootstrapPermanentFailureReason(context.Background(), job))
}

func (suite *BootstrapJobTestSuite) TestBootstrapPermanentFailureReasonMatchesKnownPattern() {
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
	cs.PrependReactor("get", "pods", podLogsReactor("dial tcp: lookup db.example.com: no such host", nil))
	r.KubeClient = cs

	reason := r.bootstrapPermanentFailureReason(context.Background(), job)
	suite.Contains(reason, "does not resolve")
}

func (suite *BootstrapJobTestSuite) TestAppendResourceConfigMapMissingConfigMapIsEmpty() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	r := newFakeReconciler(instance)

	content, err := r.appendResourceConfigMap(context.Background(), instance, "-agenttypes", "agenttypes.yaml")
	suite.Require().NoError(err)
	suite.Empty(content)
}

func (suite *BootstrapJobTestSuite) TestAppendResourceConfigMapReturnsContentWithLeadingNewline() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-layouts", Namespace: "default"},
		Data:       map[string]string{"layouts.yaml": "---\nname: 'admin'\n"},
	}
	r := newFakeReconciler(instance, cm)

	content, err := r.appendResourceConfigMap(context.Background(), instance, "-layouts", "layouts.yaml")
	suite.Require().NoError(err)
	suite.Equal("\n---\nname: 'admin'\n", content)
}

func (suite *BootstrapJobTestSuite) TestAppendResourceConfigMapEmptyKeyIsEmpty() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "inst-connections", Namespace: "default"}}
	r := newFakeReconciler(instance, cm)

	content, err := r.appendResourceConfigMap(context.Background(), instance, "-connections", "connections.yaml")
	suite.Require().NoError(err)
	suite.Empty(content)
}
