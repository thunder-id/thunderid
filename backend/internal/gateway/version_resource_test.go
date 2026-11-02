// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// fakeResourceExporter exports a resource type and, when viewable, shows its documents.
type fakeResourceExporter struct {
	resourceType string
}

func (f *fakeResourceExporter) GetResourceType() string      { return f.resourceType }
func (f *fakeResourceExporter) GetParameterizerType() string { return f.resourceType }
func (f *fakeResourceExporter) GetAllResourceIDs(context.Context) ([]string, *tidcommon.ServiceError) {
	return nil, nil
}
func (f *fakeResourceExporter) GetResourceByID(context.Context, string) (
	interface{}, string, *tidcommon.ServiceError) {
	return nil, "", nil
}
func (f *fakeResourceExporter) ValidateResource(context.Context, interface{}, string, *log.Logger) (
	string, *declarativeresource.ExportError) {
	return "", nil
}
func (f *fakeResourceExporter) GetResourceRules() *declarativeresource.ResourceRules { return nil }

// fakeViewer shows a document as the fields it holds, and for a document whose name says "with
// members" a part "members" holding their count.
type fakeViewer struct {
	fakeResourceExporter
	err error
}

func (f *fakeViewer) ViewResource(_ context.Context, document *yaml.Node) (interface{}, error) {
	if f.err != nil {
		return nil, f.err
	}
	var fields map[string]interface{}
	if err := document.Decode(&fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func (f *fakeViewer) ViewResourceParts(_ context.Context, document *yaml.Node) (map[string]interface{}, error) {
	var fields map[string]interface{}
	if err := document.Decode(&fields); err != nil {
		return nil, err
	}
	if fields["name"] == "with members" {
		return map[string]interface{}{"members": map[string]int{"count": 2}}, nil
	}
	return nil, nil
}

type appliedConfigurationFixture struct {
	*versionFixture
	viewer *fakeViewer
	svc    AppliedConfigurationServiceInterface
}

// newAppliedConfigurationFixture captures one version holding the documents it is given and records
// it as what gw-1 applied. Organization units are shown by a viewer; groups have an exporter that
// cannot show them.
func newAppliedConfigurationFixture(t *testing.T, documents ...string) *appliedConfigurationFixture {
	t.Helper()
	f := newVersionFixture(t)
	f.exporter.response = exportOf("")
	for _, document := range documents {
		f.exporter.response.Files = append(f.exporter.response.Files, export.ExportFile{Content: document})
	}
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	f.versions.applied = map[string]*AppliedVersion{"gw-1": {GatewayID: "gw-1", AppliedVersion: 1}}

	viewer := &fakeViewer{fakeResourceExporter: fakeResourceExporter{resourceType: "organization_unit"}}
	versions, ok := f.svc.(*versionService)
	require.True(t, ok)
	return &appliedConfigurationFixture{
		versionFixture: f,
		viewer:         viewer,
		svc: newAppliedConfigurationService(versions, []declarativeresource.ResourceExporter{
			viewer, &fakeResourceExporter{resourceType: "group"},
		}),
	}
}

// Every resource of the version a gateway applied is shown by the exporter of its type, with the
// parts it names, and a document that cannot be shown is listed as skipped.
func TestTheAppliedConfigurationShowsEveryResource(t *testing.T) {
	f := newAppliedConfigurationFixture(t, unit("ou-a"),
		"resource_type: organization_unit\nid: ou-b\nname: with members",
		"resource_type: organization_unit\nname: no-id",
		"resource_type: group\nid: g-1\nname: Admins",
		"resource_type: theme\nid: t-1\nname: Dark")

	configuration, svcErr := f.svc.GetAppliedConfiguration(context.Background(), "gw-1")

	require.Nil(t, svcErr)
	assert.Equal(t, "gw-1", configuration.GatewayID)
	assert.Equal(t, f.hash(1), configuration.Version)
	require.Len(t, configuration.Resources, 3)
	assert.Equal(t, AppliedResource{ResourceType: "organization_unit", ID: "ou-a",
		Resource: map[string]interface{}{"resource_type": "organization_unit", "id": "ou-a", "name": "ou-a"}},
		configuration.Resources[0])
	assert.Equal(t, map[string]interface{}{"members": map[string]int{"count": 2}}, configuration.Resources[1].Parts)
	assert.Equal(t, "no-id", configuration.Resources[2].ID, "a resource exported without an id is named")
	assert.Equal(t, []SkippedResource{
		{ResourceType: "group", ID: "g-1", Code: ErrorResourceNotViewable.Code,
			Reason: ErrorResourceNotViewable.Error.DefaultValue},
		{ResourceType: "theme", ID: "t-1", Code: ErrorResourceNotViewable.Code,
			Reason: ErrorResourceNotViewable.Error.DefaultValue},
	}, configuration.Skipped)
}

// A gateway nothing has been applied to runs nothing, and a gateway that is not registered is not
// found.
func TestTheAppliedConfigurationOfAGatewayWithNothingApplied(t *testing.T) {
	f := newAppliedConfigurationFixture(t, unit("ou-a"))
	f.versions.applied = nil

	configuration, svcErr := f.svc.GetAppliedConfiguration(context.Background(), "gw-1")
	require.Nil(t, svcErr)
	assert.Equal(t, &AppliedConfiguration{GatewayID: "gw-1", Resources: []AppliedResource{}}, configuration)

	_, svcErr = f.svc.GetAppliedConfiguration(context.Background(), "gw-missing")
	assert.Equal(t, ErrorGatewayNotFound.Code, svcErr.Code)
}

// A document that is not YAML as it stands, as one captured with a template range is not, or that
// its exporter fails to read, is skipped while the rest is shown.
func TestTheAppliedConfigurationSkipsWhatCannotBeRead(t *testing.T) {
	f := newAppliedConfigurationFixture(t, unit("ou-a"), "resource_type: organization_unit\nid: ou-t\nname: t\n"+
		"redirectUris:\n{{- range .OU_T_URIS}}\n  - {{.}}\n{{- end}}")

	configuration, svcErr := f.svc.GetAppliedConfiguration(context.Background(), "gw-1")
	require.Nil(t, svcErr)
	require.Len(t, configuration.Resources, 1)
	assert.Equal(t, []SkippedResource{{ResourceType: "organization_unit", ID: "ou-t",
		Code: ErrorResourceNotReadable.Code, Reason: ErrorResourceNotReadable.Error.DefaultValue}},
		configuration.Skipped)

	f.viewer.err = errors.New("not a unit")
	configuration, svcErr = f.svc.GetAppliedConfiguration(context.Background(), "gw-1")
	require.Nil(t, svcErr)
	assert.Empty(t, configuration.Resources)
	assert.Equal(t, ErrorResourceNotReadable.Code, configuration.Skipped[0].Code)
}

// A placeholder standing as a whole value is shown as it is written.
func TestAnAppliedPlaceholderIsShownAsItIsWritten(t *testing.T) {
	f := newAppliedConfigurationFixture(t, "resource_type: organization_unit\nid: ou-a\n"+
		"name: {{.OU_A_NAME}}\nhandles:\n  - {{.OU_A_HANDLE}}\n  - plain")

	configuration, svcErr := f.svc.GetAppliedConfiguration(context.Background(), "gw-1")

	require.Nil(t, svcErr)
	fields := configuration.Resources[0].Resource.(map[string]interface{})
	assert.Equal(t, "{{.OU_A_NAME}}", fields["name"])
	assert.Equal(t, []interface{}{"{{.OU_A_HANDLE}}", "plain"}, fields["handles"])
}

// The route serves a gateway's applied configuration, and a gateway that is not registered is 404.
func TestTheAppliedConfigurationRouteServesIt(t *testing.T) {
	f := newAppliedConfigurationFixture(t, unit("ou-a"))
	mux := http.NewServeMux()
	registerAppliedConfigurationRoutes(mux, newAppliedConfigurationHandler(f.svc))
	serve := func(path string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		return recorder
	}

	recorder := serve("/gateways/gw-1/applied-configuration")
	require.Equal(t, http.StatusOK, recorder.Code)
	var body AppliedConfiguration
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "ou-a", body.Resources[0].ID)

	assert.Equal(t, http.StatusNotFound, serve("/gateways/gw-missing/applied-configuration").Code)
}
