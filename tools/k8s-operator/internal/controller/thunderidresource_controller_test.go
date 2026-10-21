// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

type ResourceControllerTestSuite struct {
	suite.Suite
}

func TestResourceControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceControllerTestSuite))
}

func (suite *ResourceControllerTestSuite) TestParseResourceSpecValid() {
	resourceType, data, err := parseResourceSpec(rawSpec(`{"resource_type":"application","name":"Console"}`))
	suite.Require().NoError(err)
	suite.Equal("application", resourceType)
	suite.Equal("Console", data["name"])
}

func (suite *ResourceControllerTestSuite) TestParseResourceSpecEmpty() {
	resourceType, data, err := parseResourceSpec(runtime.RawExtension{})
	suite.Require().NoError(err)
	suite.Empty(resourceType)
	suite.Empty(data)
}

func (suite *ResourceControllerTestSuite) TestParseResourceSpecMalformedJSON() {
	_, _, err := parseResourceSpec(rawSpec(`{not json`))
	suite.Error(err)
}

func (suite *ResourceControllerTestSuite) TestBuildResourceYAMLUsesK8sUIDForEveryResourceType() {
	r := newFakeThunderIDResourceReconciler()
	obj := appsv1alpha1.ThunderIDResource{ObjectMeta: metav1.ObjectMeta{UID: types.UID("11111111-1111-1111-1111-111111111111")}}

	// Every resource_type uses the k8s object's own UID as id - ThunderID's own ID validation
	// is version-agnostic, so there is no per-type special case.
	for _, resourceType := range []string{"application", "flow", "theme"} {
		got := r.buildResourceYAML(obj, map[string]any{"name": "x", "resource_type": resourceType})
		suite.Contains(got, "id: 11111111-1111-1111-1111-111111111111")
	}
}

func (suite *ResourceControllerTestSuite) TestRenderResourceYAML() {
	got := renderResourceYAML(map[string]any{
		"name": "console",
		"port": float64(8090),
		"tags": []any{"a", "b"},
		"nested": map[string]any{
			"inner": true,
		},
		"empty_map":  map[string]any{},
		"empty_list": []any{},
	})

	// Keys are written sorted, regardless of map iteration order.
	want := "empty_list: []\n" +
		"empty_map: {}\n" +
		"name: console\n" +
		"nested:\n" +
		"  inner: true\n" +
		"port: 8090\n" +
		"tags:\n" +
		"  - a\n" +
		"  - b\n"
	suite.Equal(want, got)
}

func (suite *ResourceControllerTestSuite) TestRenderResourceYAMLEmojiRoundTrips() {
	// The whole point of switching to a real YAML library (see renderResourceYAML's doc comment,
	// APP-1018) only worked once it was confirmed live that ThunderID's own parser can decode a
	// non-BMP rune (emoji) scalar today; this checks the operator's own render+parse round trip
	// stays exact regardless.
	in := "emoji:👨‍💻 and 'quotes'"
	doc := renderResourceYAML(map[string]any{"key": in})

	var parsed map[string]string
	suite.Require().NoError(yaml.Unmarshal([]byte(doc), &parsed))
	suite.Equal(in, parsed["key"])
}

func (suite *ResourceControllerTestSuite) TestRenderResourceYAMLScalarTypes() {
	got := renderResourceYAML(map[string]any{
		"str":             "abc",
		"boolTrue":        true,
		"boolFalse":       false,
		"nilVal":          nil,
		"wholeFloat":      float64(42),
		"negativeFloat":   float64(-7),
		"fractionalFloat": float64(3.5),
	})

	want := "boolFalse: false\n" +
		"boolTrue: true\n" +
		"fractionalFloat: 3.5\n" +
		"negativeFloat: -7\n" +
		"nilVal: null\n" +
		"str: abc\n" +
		"wholeFloat: 42\n"
	suite.Equal(want, got)
}

func (suite *ResourceControllerTestSuite) TestRenderResourceYAMLAmbiguousStringsRoundTrip() {
	// Strings that look like other YAML types must still decode back to strings.
	got := renderResourceYAML(map[string]any{
		"looksBool": "true",
		"looksNum":  "123",
		"looksNull": "null",
	})

	var parsed map[string]any
	suite.Require().NoError(yaml.Unmarshal([]byte(got), &parsed))
	suite.Equal("true", parsed["looksBool"])
	suite.Equal("123", parsed["looksNum"])
	suite.Equal("null", parsed["looksNull"])
}

func (suite *ResourceControllerTestSuite) TestRebuildResourcesConfigMapCreatesAndSkipsUnchanged() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	app := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-app", Namespace: "default",
			Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
		Spec: rawSpec(`{"resource_type":"application","name":"Console"}`),
	}
	r := newFakeThunderIDResourceReconciler(instance, app)

	changed, err := r.rebuildResourcesConfigMap(context.Background(), instance, "application", "")
	suite.Require().NoError(err)
	suite.True(changed, "first write must report a change")

	cm := &corev1.ConfigMap{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst-applications", Namespace: "default"}, cm))
	suite.Contains(cm.Data["applications.yaml"], "name: Console")

	// The version annotation landed on the instance, so a second call with identical input
	// is a no-op (this is what keeps an unrelated resource_type's reconcile from restarting
	// the pod - see rebuildResourcesConfigMap's doc comment).
	updated := &appsv1alpha1.ThunderIDInstance{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst", Namespace: "default"}, updated))

	changedAgain, err := r.rebuildResourcesConfigMap(context.Background(), updated, "application", "")
	suite.Require().NoError(err)
	suite.False(changedAgain, "second call with unchanged content must not report a change")
}

func (suite *ResourceControllerTestSuite) TestRebuildResourcesConfigMapUnknownType() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	r := newFakeThunderIDResourceReconciler(instance)

	_, err := r.rebuildResourcesConfigMap(context.Background(), instance, "not-a-real-type", "")
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not-a-real-type")
}

func (suite *ResourceControllerTestSuite) TestRebuildResourcesConfigMapExcludesDeletedAndOtherInstances() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	included := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "keep", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}},
		Spec:       rawSpec(`{"resource_type":"credential_configuration","name":"keep-me"}`),
	}
	otherInstance := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "some-other-instance"}},
		Spec:       rawSpec(`{"resource_type":"credential_configuration","name":"not-mine"}`),
	}
	otherType := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "wrong-type", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}},
		Spec:       rawSpec(`{"resource_type":"group","name":"not-a-role"}`),
	}
	r := newFakeThunderIDResourceReconciler(instance, included, otherInstance, otherType)

	_, err := r.rebuildResourcesConfigMap(context.Background(), instance, "credential_configuration", "")
	suite.Require().NoError(err)

	cm := &corev1.ConfigMap{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst-credentialconfigurations", Namespace: "default"}, cm))
	content := cm.Data["credentialconfigurations.yaml"]
	suite.Contains(content, "keep-me")
	suite.NotContains(content, "not-mine")
	suite.NotContains(content, "not-a-role")
}

func (suite *ResourceControllerTestSuite) TestRebuildResourcesConfigMapExcludesUID() {
	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	// Unlike a real API server, the fake client's WithRuntimeObjects doesn't auto-generate a UID
	// for a seeded object - set one explicitly so excludeUID has something real to match against
	// (this is the finalizer-cleanup path's own use, see Reconcile's deletion branch).
	beingDeleted := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "leaving", Namespace: "default", UID: "leaving-uid",
			Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"},
		},
		Spec: rawSpec(`{"resource_type":"user_type","name":"leaving-role"}`),
	}
	r := newFakeThunderIDResourceReconciler(instance, beingDeleted)

	_, err := r.rebuildResourcesConfigMap(context.Background(), instance, "user_type", "leaving-uid")
	suite.Require().NoError(err)

	cm := &corev1.ConfigMap{}
	suite.Require().NoError(r.Get(context.Background(), types.NamespacedName{Name: "inst-usertypes", Namespace: "default"}, cm))
	suite.NotContains(cm.Data["usertypes.yaml"], "leaving-role")
}

func (suite *ResourceControllerTestSuite) TestResourcesForDeploymentFiltersByPhaseAndHealth() {
	list := []appsv1alpha1.ThunderIDResource{
		{ObjectMeta: metav1.ObjectMeta{Name: "errored", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}}, Status: appsv1alpha1.ThunderIDResourceStatus{Phase: "Error"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "ready-healthy", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}}, Status: appsv1alpha1.ThunderIDResourceStatus{Phase: "Ready"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "other-instance", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "not-inst"}}, Status: appsv1alpha1.ThunderIDResourceStatus{Phase: "Error"}},
	}
	objs := make([]runtime.Object, len(list))
	for i := range list {
		obj := list[i]
		objs[i] = &obj
	}
	r := newFakeThunderIDResourceReconciler(objs...)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Status:     appsv1.DeploymentStatus{UnavailableReplicas: 0},
	}
	reqs := r.resourcesForDeployment(context.Background(), dep)

	names := make([]string, 0, len(reqs))
	for _, req := range reqs {
		names = append(names, req.Name)
	}
	// Error-phase always re-triggers regardless of health; Ready-phase only re-triggers when
	// unavailable (0 here, so it's excluded); a different instance's ThunderIDResource is never included.
	suite.Contains(names, "errored")
	suite.NotContains(names, "ready-healthy")
	suite.NotContains(names, "other-instance")
}

func (suite *ResourceControllerTestSuite) TestResourcesForDeploymentIncludesReadyWhenUnavailable() {
	obj := &appsv1alpha1.ThunderIDResource{
		ObjectMeta: metav1.ObjectMeta{Name: "ready-unhealthy", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}},
		Status:     appsv1alpha1.ThunderIDResourceStatus{Phase: "Ready"},
	}
	r := newFakeThunderIDResourceReconciler(obj)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"},
		Status:     appsv1.DeploymentStatus{UnavailableReplicas: 1},
	}
	reqs := r.resourcesForDeployment(context.Background(), dep)
	suite.Require().Len(reqs, 1)
	suite.Equal("ready-unhealthy", reqs[0].Name)
}

func (suite *ResourceControllerTestSuite) TestResourcesForInstance() {
	mine := &appsv1alpha1.ThunderIDResource{ObjectMeta: metav1.ObjectMeta{Name: "mine", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "inst"}}}
	notMine := &appsv1alpha1.ThunderIDResource{ObjectMeta: metav1.ObjectMeta{Name: "not-mine", Namespace: "default", Labels: map[string]string{appsv1alpha1.InstanceRefLabel: "other"}}}
	r := newFakeThunderIDResourceReconciler(mine, notMine)

	instance := &appsv1alpha1.ThunderIDInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "default"}}
	reqs := r.resourcesForInstance(context.Background(), instance)

	suite.Require().Len(reqs, 1)
	suite.Equal("mine", reqs[0].Name)
}
