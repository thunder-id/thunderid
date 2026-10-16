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
	}, instance, controlledBy(instance, failedJob))
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
	}, instance, controlledBy(instance, staleJob))

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

// A bootstrap that can't reach its database must fail within a bounded time instead of leaving
// the Job active (and the failure unreported) forever.
func (suite *BootstrapJobInternalsTestSuite) TestCreateBootstrapJobHasDeadline() {
	instance := postgresInstance()
	r := newFakeReconciler(instance)

	suite.Require().NoError(r.createBootstrapJob(context.Background(), instance, "inst-bootstrap-seed1", "inst-security", 1))

	job := &batchv1.Job{}
	suite.Require().NoError(r.Get(context.Background(), client.ObjectKey{Name: "inst-bootstrap-seed1", Namespace: "default"}, job))
	suite.Require().NotNil(job.Spec.ActiveDeadlineSeconds)
	suite.Equal(int64(bootstrapJobDeadlineSeconds), *job.Spec.ActiveDeadlineSeconds)
}

// A container stopped by its startup probe while a database connection hangs leaves no
// termination message and no log line to match; the event must still say something useful.
func (suite *BootstrapJobInternalsTestSuite) TestCrashFallbackMessage() {
	pod := &corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
		Name: thunderidContainerName,
		LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
			ExitCode: 143, Reason: "Error",
		}},
	}}}}
	crashed := podIssue{reason: reasonCrashed}

	pg := crashFallbackMessage(pod, postgresInstance(), crashed)
	suite.Contains(pg, "code 143")
	suite.Contains(pg, "reachable from the cluster")

	sqlite := crashFallbackMessage(pod, &appsv1alpha1.ThunderIDInstance{}, crashed)
	suite.Contains(sqlite, "code 143")
	suite.NotContains(sqlite, "PostgreSQL")

	suite.Empty(crashFallbackMessage(pod, postgresInstance(), podIssue{reason: reasonCreateContainerConfigError}),
		"a container that never started has its own diagnosis")
}

// The log lines here are verbatim from live runs against a real PostgreSQL server.
func (suite *BootstrapJobInternalsTestSuite) TestMatchDatabaseError() {
	cases := []struct {
		name, logs, wantReasonContains, wantLine string
	}{
		{
			name:               "no route to host",
			logs:               "starting\ntime=x level=ERROR msg=\"Failed to initialize config database client\" error=\"failed to ping database config: dial tcp 172.19.208.1:5499: connect: no route to host\"\nexiting",
			wantReasonContains: "unreachable from the cluster",
			wantLine:           "time=x level=ERROR msg=\"Failed to initialize config database client\" error=\"failed to ping database config: dial tcp 172.19.208.1:5499: connect: no route to host\"",
		},
		{
			name:               "timeout",
			logs:               "error=\"dial tcp 10.0.0.9:5432: i/o timeout\"",
			wantReasonContains: "did not respond",
			wantLine:           "error=\"dial tcp 10.0.0.9:5432: i/o timeout\"",
		},
		{
			name:               "schema mismatch",
			logs:               "msg=\"Failed to get existing entity type\" error=\"failed to execute query: pq: column \\\"name\\\" does not exist\"",
			wantReasonContains: "/opt/thunderid/dbscripts",
		},
		{
			name:               "database missing, escaped quotes",
			logs:               "error=\"pq: database \\\"tid_missing\\\" does not exist\"",
			wantReasonContains: "postgres.name does not exist",
		},
		{
			name:               "role missing, plain quotes",
			logs:               "pq: role \"nobody\" does not exist",
			wantReasonContains: "postgres.username does not exist",
		},
		{
			name:               "wrong password",
			logs:               "pq: password authentication failed for user \"tid\"",
			wantReasonContains: "password",
		},
	}
	for _, c := range cases {
		reason, line := matchDatabaseError([]byte(c.logs))
		suite.Contains(reason, c.wantReasonContains, c.name)
		if c.wantLine != "" {
			suite.Equal(c.wantLine, line, c.name)
		}
	}

	reason, line := matchDatabaseError([]byte("server started\nlistening on :8090\n"))
	suite.Empty(reason)
	suite.Empty(line)
}
