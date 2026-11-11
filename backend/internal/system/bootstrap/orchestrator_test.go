// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/importer"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// stubImportService records the request it receives and returns a configured result.
type stubImportService struct {
	called      bool
	lastRequest *importer.ImportRequest
	response    *importer.ImportResponse
	err         *tidcommon.ServiceError
}

func (s *stubImportService) ImportResources(_ context.Context, request *importer.ImportRequest) (
	*importer.ImportResponse, *tidcommon.ServiceError) {
	s.called = true
	s.lastRequest = request
	return s.response, s.err
}

func (s *stubImportService) DeleteResource(_ context.Context, _ *importer.DeleteResourceRequest) (
	*importer.DeleteResourceResponse, *tidcommon.ServiceError) {
	return nil, nil
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}

func okResponse(imported int) *importer.ImportResponse {
	return &importer.ImportResponse{Summary: &importer.ImportSummary{Imported: imported}}
}

func TestRun_AppliesBundleWithEnvSubstitutionAndOptions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", "# resource_type: organization_unit\nhandle: default\nname: Default\n")
	// A document with an env-var placeholder, resolved via SubstituteEnvironmentVariables.
	writeFile(t, dir, "b.yaml", "# resource_type: user\ntype: Person\nouHandle: default\n"+
		"attributes:\n  username: \"{{ .ADMIN_USERNAME }}\"\n")
	// JSON definitions (flows/themes kept as .json) are also loaded.
	writeFile(t, dir, "c.json", `{"displayName":"Dark","theme":{}}`)

	t.Setenv("ADMIN_USERNAME", "root")

	stub := &stubImportService{response: okResponse(3)}
	if err := Run(context.Background(), stub, Options{DefaultsDir: dir}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !stub.called {
		t.Fatal("expected ImportResources to be called")
	}

	// Documents are concatenated as a multi-document payload.
	content := stub.lastRequest.Content
	if !strings.Contains(content, "organization_unit") || !strings.Contains(content, `"displayName":"Dark"`) {
		t.Fatalf("bundle missing documents: %q", content)
	}
	if !strings.Contains(content, "\n---\n") {
		t.Fatalf("documents not joined as multi-document YAML: %q", content)
	}

	// The {{ .ADMIN_USERNAME }} placeholder is resolved from the environment.
	if !strings.Contains(content, `username: "root"`) || strings.Contains(content, "{{ .ADMIN_USERNAME }}") {
		t.Fatalf("env var not substituted in bundle: %q", content)
	}

	o := stub.lastRequest.Options
	if o == nil || !o.IsUpsertEnabled() || o.IsContinueOnErrorEnabled() || o.Target != "runtime" {
		t.Fatalf("unexpected import options: %#v", o)
	}
	// Resources a control plane deployed are left alone rather than failing the bootstrap on the refusal.
	if !o.SkipManaged {
		t.Fatal("expected the bootstrap to skip resources a control plane deployed")
	}
}

func TestRun_EmptyDirectoryIsNoOp(t *testing.T) {
	stub := &stubImportService{response: okResponse(0)}

	if err := Run(context.Background(), stub, Options{DefaultsDir: t.TempDir()}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stub.called {
		t.Fatal("ImportResources should not be called for an empty bundle")
	}
}

func TestRun_MissingDirectoryIsNoOp(t *testing.T) {
	stub := &stubImportService{response: okResponse(0)}
	opts := Options{DefaultsDir: filepath.Join(t.TempDir(), "does-not-exist")}

	if err := Run(context.Background(), stub, opts); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stub.called {
		t.Fatal("ImportResources should not be called when the directory is absent")
	}
}

func TestRun_PropagatesItemFailure(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", "# resource_type: organization_unit\nhandle: default\nname: Default\n")

	stub := &stubImportService{response: &importer.ImportResponse{
		Summary: &importer.ImportSummary{Imported: 0, Failed: 1},
		Results: []importer.ImportItemOutcome{{
			ResourceType: "organization_unit", ResourceName: "default", Status: "failed",
			Code: "OU-1", Message: "boom",
		}},
	}}

	err := Run(context.Background(), stub, Options{DefaultsDir: dir})
	if err == nil {
		t.Fatal("expected Run to fail when a document fails to import")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error should include the failure detail: %v", err)
	}
}

func TestRun_PropagatesServiceError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", "# resource_type: organization_unit\nhandle: default\nname: Default\n")

	stub := &stubImportService{err: &tidcommon.ServiceError{Code: "IMP-1"}}

	if err := Run(context.Background(), stub, Options{DefaultsDir: dir}); err == nil {
		t.Fatal("expected Run to fail when the import service returns an error")
	}
}
