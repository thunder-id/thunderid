// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/export"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const (
	testSecret   = "shhh"
	testResource = "resource_type: application"
)

type fakeExporter struct {
	response *export.ExportResponse
	svcErr   *tidcommon.ServiceError
	asked    *export.ExportRequest
}

func (f *fakeExporter) ExportResources(
	_ context.Context, req *export.ExportRequest,
) (*export.ExportResponse, *tidcommon.ServiceError) {
	f.asked = req
	return f.response, f.svcErr
}

type fakeDataPlane struct {
	gotSecret    string
	gotContent   string
	gotVariables map[string]string
	err          error
}

func (f *fakeDataPlane) Apply(
	_ context.Context, _ *Gateway, secret, content string, variables map[string]string,
) (*ImportResult, error) {
	f.gotSecret, f.gotContent, f.gotVariables = secret, content, variables
	if f.err != nil {
		return nil, f.err
	}
	return &ImportResult{Summary: &ImportSummary{Imported: 2}}, nil
}

// registered builds a stored gateway whose credential is encrypted the way registration leaves it.
func registered(t *testing.T) *Gateway {
	t.Helper()
	// The credential is built the way registration builds it, so the provider has to be in place
	// before it is encrypted rather than only before it is read back.
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	secret, err := cmodels.NewProperty("clientSecret", testSecret, true)
	if err != nil {
		t.Fatalf("failed to build the credential: %v", err)
	}
	stored, err := cmodels.SerializePropertiesToJSONArray([]cmodels.Property{*secret})
	if err != nil {
		t.Fatalf("failed to serialize the credential: %v", err)
	}
	return &Gateway{ID: "gw-1", Name: "dev", BaseURL: "https://dp.example.test",
		ClientID: "system-app", ClientSecret: stored}
}

func exportOf(env string) *export.ExportResponse {
	resp := &export.ExportResponse{Files: []export.ExportFile{{FileName: "a.yaml", Content: testResource}}}
	if env != "" {
		resp.EnvFile = &export.EnvironmentFile{FileName: ".env", Content: env}
	}
	return resp
}

func applyService(t *testing.T, gw *Gateway, exp *fakeExporter, dp *fakeDataPlane) ServiceInterface {
	t.Helper()
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })
	return newService(&fakeStore{gateways: []Gateway{*gw}}, exp, dp)
}

func TestApplySendsTheCurrentConfiguration(t *testing.T) {
	gw := registered(t)
	exp := &fakeExporter{response: exportOf("APP_URL=https://example.test\n")}
	dp := &fakeDataPlane{}

	result, svcErr := applyService(t, gw, exp, dp).Apply(context.Background(), "gw-1")
	if svcErr != nil {
		t.Fatalf("apply: %v", svcErr)
	}
	if result.Import.Summary.Imported != 2 {
		t.Fatalf("expected the gateway's answer through, got %+v", result.Import)
	}
	if dp.gotContent == "" {
		t.Error("expected the exported configuration to be sent")
	}
	if dp.gotVariables["APP_URL"] != "https://example.test" {
		t.Errorf("expected the resolved variable to be sent, got %q", dp.gotVariables["APP_URL"])
	}
	// The credential must be decrypted before it is presented to the gateway.
	if dp.gotSecret != testSecret {
		t.Errorf("expected the decrypted credential, got %q", dp.gotSecret)
	}
}

// Every resource type is asked for, or an apply would silently leave some configuration behind.
func TestApplyExportsEveryResourceType(t *testing.T) {
	gw := registered(t)
	exp := &fakeExporter{response: exportOf("")}
	applyService(t, gw, exp, &fakeDataPlane{}).Apply(context.Background(), "gw-1")

	if exp.asked == nil {
		t.Fatal("the exporter was not asked")
	}
	for name, ids := range map[string][]string{
		"applications": exp.asked.Applications, "users": exp.asked.Users, "flows": exp.asked.Flows,
		"connections": exp.asked.Connections, "roles": exp.asked.Roles, "themes": exp.asked.Themes,
	} {
		if len(ids) != 1 || ids[0] != "*" {
			t.Errorf("expected every %s to be exported, got %v", name, ids)
		}
	}
}

// A placeholder the control plane has no value for reaches the gateway empty. Saying so is how an
// operator finds out, rather than discovering it when a login fails.
func TestApplyReportsTheVariablesItCouldNotResolve(t *testing.T) {
	gw := registered(t)
	env := "APP_SECRET=\nAPP_URL=https://example.test\nIDP_SECRET=\n"
	exp := &fakeExporter{response: exportOf(env)}

	result, svcErr := applyService(t, gw, exp, &fakeDataPlane{}).Apply(context.Background(), "gw-1")
	if svcErr != nil {
		t.Fatalf("apply: %v", svcErr)
	}
	if len(result.UnresolvedVariables) != 2 ||
		result.UnresolvedVariables[0] != "APP_SECRET" || result.UnresolvedVariables[1] != "IDP_SECRET" {
		t.Fatalf("expected both unresolved names in order, got %v", result.UnresolvedVariables)
	}
}

// An empty value would resolve the placeholder to nothing and write an empty credential. Leaving it
// out is what lets the gateway supply it from its own environment.
func TestApplyOmitsTheVariablesItCouldNotResolve(t *testing.T) {
	gw := registered(t)
	exp := &fakeExporter{response: exportOf("APP_SECRET=\nAPP_URL=https://example.test\n")}
	dp := &fakeDataPlane{}

	if _, svcErr := applyService(t, gw, exp, dp).Apply(context.Background(), "gw-1"); svcErr != nil {
		t.Fatalf("apply: %v", svcErr)
	}
	if _, sent := dp.gotVariables["APP_SECRET"]; sent {
		t.Error("an unresolved placeholder was sent, which would write an empty credential")
	}
	if dp.gotVariables["APP_URL"] != "https://example.test" {
		t.Error("expected the resolved variable to still be sent")
	}
}

func TestApplyRefusesAnUnknownGateway(t *testing.T) {
	gw := registered(t)
	_, svcErr := applyService(t, gw, &fakeExporter{}, &fakeDataPlane{}).Apply(context.Background(), "missing")
	if svcErr == nil || svcErr.Code != ErrorGatewayNotFound.Code {
		t.Fatalf("expected not found, got %v", svcErr)
	}
}

func TestApplyRefusesWhenThereIsNothingToSend(t *testing.T) {
	gw := registered(t)
	exp := &fakeExporter{response: &export.ExportResponse{}}
	_, svcErr := applyService(t, gw, exp, &fakeDataPlane{}).Apply(context.Background(), "gw-1")
	if svcErr == nil || svcErr.Code != ErrorNothingToApply.Code {
		t.Fatalf("expected a refusal, got %v", svcErr)
	}
}

// A failure from the gateway can quote the request back, so it is reported without that detail.
func TestApplyReportsAGatewayFailureWithoutItsBody(t *testing.T) {
	gw := registered(t)
	exp := &fakeExporter{response: exportOf("")}
	dp := &fakeDataPlane{err: errors.New("the gateway answered 500: client_secret=shhh")}

	_, svcErr := applyService(t, gw, exp, dp).Apply(context.Background(), "gw-1")
	if svcErr == nil || svcErr.Code != ErrorGatewayUnreachable.Code {
		t.Fatalf("expected an unreachable error, got %v", svcErr)
	}
	if svcErr.ErrorDescription.DefaultValue != "" {
		t.Errorf("expected no detail from the gateway in the response, got %q",
			svcErr.ErrorDescription.DefaultValue)
	}
}
