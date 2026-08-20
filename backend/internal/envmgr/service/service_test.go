/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/envmgr/model"
	"github.com/thunder-id/thunderid/internal/envmgr/store"
	"github.com/thunder-id/thunderid/internal/envmgr/thunder"
)

const testAppB = "app-b"

// fakeClient records import calls and serves canned export/reveal data.
type fakeClient struct {
	exportResources string
	exportEnv       string
	secrets         map[string]string
	envVars         map[string]string
	secretNames     []string
	// secretNamesCalls counts how often the data plane was asked, so a test can show that pods do
	// not each ask for themselves.
	secretNamesCalls int
	kvWrites         map[string]map[string]interface{}
	kvExisting       map[string][2]string
	imports          []thunder.ImportRequest
}

func (f *fakeClient) Export(context.Context) (thunder.ExportResult, error) {
	return thunder.ExportResult{Resources: f.exportResources, EnvFile: f.exportEnv}, nil
}

func (f *fakeClient) SecretKeys(context.Context) ([]string, error) {
	keys := make([]string, 0, len(f.secrets))
	for k := range f.secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func (f *fakeClient) EnvironmentVariables(context.Context, string) (map[string]string, error) {
	return f.envVars, nil
}

func (f *fakeClient) SecretNames(context.Context) ([]string, error) {
	f.secretNamesCalls++
	return f.secretNames, nil
}

func (f *fakeClient) PutSecret(_ context.Context, name string, body map[string]interface{}) error {
	if f.kvWrites == nil {
		f.kvWrites = map[string]map[string]interface{}{}
	}
	f.kvWrites[name] = body
	f.secretNames = append(f.secretNames, name)
	return nil
}

func (f *fakeClient) GetSecret(_ context.Context, name string) (string, string, bool, error) {
	if body, ok := f.kvWrites[name]; ok {
		kind, _ := body["kind"].(string)
		value, _ := body["value"].(string)
		return kind, value, true, nil
	}
	if v, ok := f.kvExisting[name]; ok {
		return v[0], v[1], true, nil
	}
	return "", "", false, nil
}

func (f *fakeClient) Import(_ context.Context, req thunder.ImportRequest) (*thunder.ImportResponse, error) {
	f.imports = append(f.imports, req)
	return &thunder.ImportResponse{Summary: &thunder.ImportSummary{}}, nil
}

func (f *fakeClient) lastImport() thunder.ImportRequest {
	return f.imports[len(f.imports)-1]
}

// fakeDataPlanes hands every environment the same fake, standing in for the connections data planes
// hold open to the control plane.
type fakeDataPlanes struct {
	plane     DataPlane
	connected bool
}

func (f *fakeDataPlanes) For(dataPlaneID string) (DataPlane, error) {
	if !f.connected {
		return nil, fmt.Errorf("data plane %s is not connected", dataPlaneID)
	}
	return f.plane, nil
}

func (f *fakeDataPlanes) Status(string) model.DataPlaneStatus {
	return model.DataPlaneStatus{Connected: f.connected}
}

func newTestService(t *testing.T, fake *fakeClient) *Service {
	t.Helper()
	svc := New(newMemStore(), func(string, thunder.Credentials, bool) ThunderClient { return fake })
	svc.SetWorkspaceURL("https://cp")
	svc.SetOrganization("org1")
	svc.SetDataPlanes(&fakeDataPlanes{plane: fake, connected: true})
	svc.SetSecretSealer(fakeSealer{})
	return svc
}

// app returns a minimal application document.
func app(id string) string {
	return fmt.Sprintf("resource_type: application\nid: %s\nname: %s", id, id)
}

func bundleOf(ids ...string) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, app(id))
	}
	return strings.Join(parts, "\n---\n")
}

func TestApplyTracksDeletions(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)

	env, err := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})
	if err != nil {
		t.Fatalf("create env: %v", err)
	}

	if _, err := svc.UploadVersion(context.Background(), env.ID, bundleOf("app-a", testAppB), nil, "v1"); err != nil {
		t.Fatalf("upload v1: %v", err)
	}
	res, err := svc.Apply(context.Background(), env.ID, "latest", false)
	if err != nil {
		t.Fatalf("apply v1: %v", err)
	}
	if len(fake.lastImport().Deletions) != 0 {
		t.Fatalf("first apply should have no deletions: %+v", fake.lastImport().Deletions)
	}
	if res.TargetSeq != 1 {
		t.Fatalf("expected target seq 1, got %d", res.TargetSeq)
	}

	// v2 removes app-b -> apply must request its deletion.
	if _, err := svc.UploadVersion(context.Background(), env.ID, bundleOf("app-a"), nil, "v2"); err != nil {
		t.Fatalf("upload v2: %v", err)
	}
	if _, err := svc.Apply(context.Background(), env.ID, "latest", false); err != nil {
		t.Fatalf("apply v2: %v", err)
	}
	dels := fake.lastImport().Deletions
	if len(dels) != 1 || dels[0].ResourceType != "application" || dels[0].ID != testAppB {
		t.Fatalf("expected deletion of application app-b, got %+v", dels)
	}

	// Applied version must be recorded on the environment.
	got, _ := svc.GetEnvironment(context.Background(), env.ID)
	if got.AppliedSeq != 2 {
		t.Fatalf("expected appliedSeq 2, got %d", got.AppliedSeq)
	}
}

func TestApplyDryRunDoesNotRecord(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})
	_, _ = svc.UploadVersion(context.Background(), env.ID, bundleOf("app-a"), nil, "v1")

	if _, err := svc.Apply(context.Background(), env.ID, "latest", true); err != nil {
		t.Fatalf("dry run apply: %v", err)
	}
	if !fake.lastImport().DryRun {
		t.Fatalf("expected dryRun propagated to import")
	}
	got, _ := svc.GetEnvironment(context.Background(), env.ID)
	if got.AppliedSeq != 0 {
		t.Fatalf("dry run must not record appliedSeq, got %d", got.AppliedSeq)
	}
}

func TestCaptureRecordsSecretKeysWithoutValues(t *testing.T) {
	fake := &fakeClient{
		exportResources: bundleOf("app-a"),
		exportEnv:       "APP_A_CLIENT_ID=abc\nAPP_A_CLIENT_SECRET=from-export",
		secrets:         map[string]string{"APP_A_CLIENT_SECRET": "never-read"},
	}
	svc := newTestService(t, fake)
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name:   "dev",
		Target: model.Target{DataPlaneID: "dp"},
	})

	v, err := svc.CaptureVersion(context.Background(), env.ID, "captured")
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	full, _ := svc.GetVersion(context.Background(), env.ID, v.Seq)

	if full.Variables["APP_A_CLIENT_ID"] != "abc" {
		t.Fatalf("expected the non-secret variable to be carried over")
	}
	// A secret's value must not be stored, even when the export happened to include one.
	if _, present := full.Variables["APP_A_CLIENT_SECRET"]; present {
		t.Fatalf("secret value was stored: %#v", full.Variables)
	}
	if len(full.SecretKeys) != 1 || full.SecretKeys[0] != "APP_A_CLIENT_SECRET" {
		t.Fatalf("expected the secret key to be recorded, got %v", full.SecretKeys)
	}
}

func TestCaptureLetsControlPlaneVariablesOverrideTheExport(t *testing.T) {
	fake := &fakeClient{
		exportResources: bundleOf("app-a"),
		exportEnv:       "APP_A_REDIRECT_URIS=[\"https://stale\"]",
		envVars:         map[string]string{"APP_A_REDIRECT_URIS": `["https://configured"]`},
	}
	svc := newTestService(t, fake)
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name:   "dev",
		Target: model.Target{DataPlaneID: "dp"},
	})

	v, _ := svc.CaptureVersion(context.Background(), env.ID, "")
	full, _ := svc.GetVersion(context.Background(), env.ID, v.Seq)
	if full.Variables["APP_A_REDIRECT_URIS"] != `["https://configured"]` {
		t.Fatalf("control plane value should win, got %q", full.Variables["APP_A_REDIRECT_URIS"])
	}
}

func TestApplyOmitsSecretsSoTheDataPlaneFillsThem(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})

	resources := "resource_type: application\nid: app-a\nname: app-a\nclientSecret: {{.APP_A_CLIENT_SECRET}}"
	stored, err := svc.store.AddVersion(context.Background(), model.Version{
		EnvID:      env.ID,
		Origin:     model.OriginUploaded,
		CreatedAt:  svc.now().UTC(),
		Resources:  resources,
		Variables:  map[string]string{},
		SecretKeys: []string{"APP_A_CLIENT_SECRET"},
	})
	if err != nil {
		t.Fatalf("add version: %v", err)
	}
	if _, err := svc.Apply(context.Background(), env.ID, strconv.Itoa(stored.Seq), false); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if _, present := fake.lastImport().Variables["APP_A_CLIENT_SECRET"]; present {
		t.Fatalf("a secret must be omitted so the data plane resolves it, got %#v",
			fake.lastImport().Variables["APP_A_CLIENT_SECRET"])
	}
	// The placeholder itself must survive in the content for the data plane to fill.
	if !strings.Contains(fake.lastImport().Content, "{{.APP_A_CLIENT_SECRET}}") {
		t.Fatalf("the placeholder should remain in the content: %s", fake.lastImport().Content)
	}
}

// A capture reads the organization's workspace, so a service that does not know where its control
// plane answers has nothing to read and says so rather than storing an empty version.
func TestCaptureRequiresAWorkspace(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	svc.SetWorkspaceURL("")
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})
	if _, err := svc.CaptureVersion(context.Background(), env.ID, ""); !errors.Is(err, ErrNoWorkspace) {
		t.Fatalf("expected ErrNoWorkspace, got %v", err)
	}
}

func TestApplyPicksUpVariablesAddedAfterCapture(t *testing.T) {
	// The control plane has nothing configured at capture time.
	fake := &fakeClient{
		exportResources: "resource_type: application\nid: app-a\nname: app-a\n" +
			"redirectUris:\n  {{- range .APP_A_REDIRECT_URIS}}\n  - {{.}}\n  {{- end}}",
	}
	svc := newTestService(t, fake)
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name:   "dev",
		Target: model.Target{DataPlaneID: "dp"},
	})
	if _, err := svc.CaptureVersion(context.Background(), env.ID, ""); err != nil {
		t.Fatalf("capture: %v", err)
	}

	status, err := svc.CheckVariables(context.Background(), env.ID, "latest")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(status.Missing) != 1 || status.Missing[0] != "APP_A_REDIRECT_URIS" {
		t.Fatalf("expected the redirect URIs to be reported missing, got %v", status.Missing)
	}

	// The operator now sets it in the control plane, without re-capturing.
	fake.envVars = map[string]string{"APP_A_REDIRECT_URIS": `["https://dp/callback"]`}

	status, err = svc.CheckVariables(context.Background(), env.ID, "latest")
	if err != nil {
		t.Fatalf("re-check: %v", err)
	}
	if len(status.Missing) != 0 {
		t.Fatalf("the new value should clear the warning, got %v", status.Missing)
	}

	if _, err := svc.Apply(context.Background(), env.ID, "latest", false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	arr, ok := fake.lastImport().Variables["APP_A_REDIRECT_URIS"].([]interface{})
	if !ok || len(arr) != 1 || arr[0] != "https://dp/callback" {
		t.Fatalf("apply should use the live value, got %#v", fake.lastImport().Variables["APP_A_REDIRECT_URIS"])
	}
}

func TestPromoteSelective(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	dev, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{Name: "dev", Rank: intp(1),
		Target: model.Target{DataPlaneID: "dev"}})
	prod, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{Name: "prod", Rank: intp(2),
		Target: model.Target{DataPlaneID: "prod"}})

	_, _ = svc.UploadVersion(context.Background(), dev.ID, bundleOf("app-a", testAppB), nil, "dev-v1")

	// Promote only app-a to prod.
	result, err := svc.Promote(context.Background(), PromoteInput{
		FromEnvID: dev.ID, ToEnvID: prod.ID, Selection: []string{"application/id:app-a"},
		SelectionProvided: true,
	})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if result.Preview.Summary.Added != 2 {
		t.Fatalf("preview should show 2 additions, got %+v", result.Preview.Summary)
	}
	full, _ := svc.GetVersion(context.Background(), prod.ID, result.NewVersion.Seq)
	if strings.Contains(full.Resources, testAppB) {
		t.Fatalf("app-b should not have been promoted:\n%s", full.Resources)
	}
	if !strings.Contains(full.Resources, "app-a") {
		t.Fatalf("app-a should have been promoted:\n%s", full.Resources)
	}
	if full.Origin != model.OriginPromoted || full.SourceEnvID != dev.ID {
		t.Fatalf("promotion metadata wrong: %+v", full)
	}
}

func TestPromoteAllAndApply(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	dev, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{Name: "dev", Rank: intp(1),
		Target: model.Target{DataPlaneID: "dev"}})
	prod, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{Name: "prod", Rank: intp(2),
		Target: model.Target{DataPlaneID: "prod"}})
	_, _ = svc.UploadVersion(context.Background(), dev.ID, bundleOf("app-a", testAppB), nil, "dev-v1")

	result, err := svc.Promote(context.Background(), PromoteInput{FromEnvID: dev.ID, ToEnvID: prod.ID, Apply: true})
	if err != nil {
		t.Fatalf("promote+apply: %v", err)
	}
	if result.Applied == nil {
		t.Fatalf("expected apply result")
	}
	got, _ := svc.GetEnvironment(context.Background(), prod.ID)
	if got.AppliedSeq != result.NewVersion.Seq {
		t.Fatalf("prod applied seq %d != new version %d", got.AppliedSeq, result.NewVersion.Seq)
	}
}

func TestRevertRestoresAndAdvancesHead(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})
	_, _ = svc.UploadVersion(context.Background(), env.ID, bundleOf("app-a"), nil, "v1")
	_, _ = svc.UploadVersion(context.Background(), env.ID, bundleOf("app-a", testAppB), nil, "v2")

	result, err := svc.Revert(context.Background(), RevertInput{EnvID: env.ID, ToRef: "1"})
	if err != nil {
		t.Fatalf("revert: %v", err)
	}
	if result.NewVersion.Seq != 3 || result.NewVersion.Origin != model.OriginReverted {
		t.Fatalf("revert should add a new head v3: %+v", result.NewVersion)
	}
	full, _ := svc.GetVersion(context.Background(), env.ID, 3)
	if strings.Contains(full.Resources, testAppB) {
		t.Fatalf("reverted head should match v1 (no app-b):\n%s", full.Resources)
	}
	// Preview reflects removing app-b (current v2 -> target v1).
	if result.Preview.Summary.Deleted != 1 {
		t.Fatalf("expected one deletion in revert preview, got %+v", result.Preview.Summary)
	}
}

func TestRevertToPreviousResolvesSecondNewest(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})
	_, _ = svc.UploadVersion(context.Background(), env.ID, bundleOf("app-a"), nil, "v1")
	_, _ = svc.UploadVersion(context.Background(), env.ID, bundleOf("app-a", testAppB), nil, "v2")
	_, _ = svc.UploadVersion(context.Background(), env.ID, bundleOf("app-a", testAppB, "app-c"), nil, "v3")

	result, err := svc.Revert(context.Background(), RevertInput{EnvID: env.ID, ToRef: "previous"})
	if err != nil {
		t.Fatalf("revert to previous: %v", err)
	}
	// v3 is the head, so "previous" is v2: the new head must match v2's content.
	full, _ := svc.GetVersion(context.Background(), env.ID, result.NewVersion.Seq)
	if !strings.Contains(full.Resources, testAppB) || strings.Contains(full.Resources, "app-c") {
		t.Fatalf("expected v2 content restored, got:\n%s", full.Resources)
	}
	if full.SourceSeq != 2 {
		t.Fatalf("expected sourceSeq 2, got %d", full.SourceSeq)
	}
}

func TestRevertToPreviousRequiresTwoVersions(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})
	_, _ = svc.UploadVersion(context.Background(), env.ID, bundleOf("app-a"), nil, "v1")

	if _, err := svc.Revert(context.Background(),
		RevertInput{EnvID: env.ID, ToRef: "previous"}); !errors.Is(err, ErrNoPreviousVersion) {
		t.Fatalf("expected ErrNoPreviousVersion, got %v", err)
	}
}

func TestApplyAllPushesEveryEnvironment(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	dev, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Rank: intp(1), Target: model.Target{DataPlaneID: "dev"},
	})
	prod, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "prod", Rank: intp(2), Target: model.Target{DataPlaneID: "prod"},
	})
	_, _ = svc.UploadVersion(context.Background(), dev.ID, bundleOf("app-a"), nil, "v1")
	_, _ = svc.UploadVersion(context.Background(), prod.ID, bundleOf("app-a"), nil, "v1")

	results := svc.ApplyAll(context.Background(), false)

	if len(results) != 2 {
		t.Fatalf("expected a result per environment, got %d", len(results))
	}
	for _, r := range results {
		if r.Error != "" || r.Applied == nil {
			t.Fatalf("%s should have applied, got error %q", r.EnvName, r.Error)
		}
	}
	if len(fake.imports) != 2 {
		t.Fatalf("expected two imports, got %d", len(fake.imports))
	}
}

func TestApplyAllSkipsEnvironmentsWithoutAVersionAndKeepsGoing(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	empty, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "empty", Rank: intp(1), Target: model.Target{DataPlaneID: "empty"},
	})
	ready, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "ready", Rank: intp(2), Target: model.Target{DataPlaneID: "ready"},
	})
	_, _ = svc.UploadVersion(context.Background(), ready.ID, bundleOf("app-a"), nil, "v1")

	results := svc.ApplyAll(context.Background(), false)

	byID := map[string]ApplyAllResult{}
	for _, r := range results {
		byID[r.EnvID] = r
	}
	// The environment with nothing to apply is reported, not fatal, and the other still goes through.
	if byID[empty.ID].Error == "" {
		t.Fatalf("expected the empty environment to report why it was skipped")
	}
	if byID[ready.ID].Applied == nil {
		t.Fatalf("a skipped environment must not stop the others")
	}
}

func TestCheckVariablesReportsSecretsTheDataPlaneLacks(t *testing.T) {
	// The data plane holds one of the two secrets the configuration needs.
	fake := &fakeClient{
		exportResources: "resource_type: application\nid: app-a\nname: app-a\n" +
			"clientSecret: {{.APP_A_CLIENT_SECRET}}\nother: {{.APP_B_CLIENT_SECRET}}",
		secrets:     map[string]string{"APP_A_CLIENT_SECRET": "x", "APP_B_CLIENT_SECRET": "y"},
		secretNames: []string{"APP_A_CLIENT_SECRET"},
	}
	svc := newTestService(t, fake)
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev",
		Target: model.Target{
			DataPlaneID: "dp",
		},
	})
	if _, err := svc.CaptureVersion(context.Background(), env.ID, ""); err != nil {
		t.Fatalf("capture: %v", err)
	}

	status, err := svc.CheckVariables(context.Background(), env.ID, "latest")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !status.SecretsChecked {
		t.Fatal("the secret service was configured, so it should have been consulted")
	}
	if len(status.MissingSecrets) != 1 || status.MissingSecrets[0] != "APP_B_CLIENT_SECRET" {
		t.Fatalf("expected only the absent secret to be reported, got %v", status.MissingSecrets)
	}
}

func TestCheckVariablesUsesTheDataPlanesOwnSecretStore(t *testing.T) {
	fake := &fakeClient{
		exportResources: "resource_type: application\nid: app-a\nname: app-a\n" +
			"clientSecret: {{.APP_A_CLIENT_SECRET}}",
		secrets:     map[string]string{"APP_A_CLIENT_SECRET": "x"},
		secretNames: []string{"APP_A_CLIENT_SECRET"},
	}
	svc := newTestService(t, fake)
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name:   "dev",
		Target: model.Target{DataPlaneID: "dp"},
	})
	_, _ = svc.CaptureVersion(context.Background(), env.ID, "")

	status, _ := svc.CheckVariables(context.Background(), env.ID, "latest")
	// With no separate service named, the data plane's own store answers, so the check is real.
	if !status.SecretsChecked {
		t.Fatal("the data plane's own secret store should have been consulted")
	}
	if len(status.MissingSecrets) != 0 {
		t.Fatalf("the store holds the secret, got %v", status.MissingSecrets)
	}
}

func TestPromoteLeavesTheDestinationsCredentialsAlone(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	dev, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Rank: intp(1), Target: model.Target{DataPlaneID: "dev"},
	})
	stage, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "stage", Rank: intp(2),
		Target: model.Target{
			DataPlaneID: "stage",
		},
	})

	// A version whose configuration needs one secret.
	if _, err := svc.store.AddVersion(context.Background(), model.Version{
		EnvID: dev.ID, Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(),
		Resources:  "resource_type: application\nid: app-a\nname: app-a",
		Variables:  map[string]string{},
		SecretKeys: []string{"APP_A_CLIENT_SECRET"},
	}); err != nil {
		t.Fatalf("add version: %v", err)
	}

	result, err := svc.Promote(context.Background(), PromoteInput{FromEnvID: dev.ID, ToEnvID: stage.ID})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}

	// Nothing invents a credential for the destination: it is named as one the destination has to hold,
	// and set against its own data plane.
	if len(result.Secrets.Generated) != 0 || len(result.Secrets.Skipped) != 1 {
		t.Fatalf("a promote must not issue credentials, got %+v", result.Secrets)
	}
	// A promotion is started from the source, so it must not reach into the destination's secret store:
	// those credentials arrive by the destination's own control plane capturing them.
	if len(fake.kvWrites) != 0 {
		t.Fatalf("a promote must not write to the destination's secret store, got %v", fake.kvWrites)
	}

	// A promotion moves a version onto the destination and writes nothing anywhere else: the
	// organization has one workspace, and the promoted configuration reaches a deployment only when it
	// is applied.
	if len(fake.imports) != 0 {
		t.Fatalf("a promote must import nothing on its own, got %v", fake.imports)
	}
}

func TestPromoteNeverReadsTheDestinationsSecretStore(t *testing.T) {
	// The destination holds a credential of its own. A promote must neither consult it nor replace it:
	// the destination's secret store belongs to the destination, and the source cannot manage it.
	fake := &fakeClient{kvExisting: map[string][2]string{"APP_A_CLIENT_SECRET": {"hash", "the-hash"}}}
	svc := newTestService(t, fake)
	dev, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Rank: intp(1), Target: model.Target{DataPlaneID: "dev"},
	})
	stage, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "stage", Rank: intp(2),
		Target: model.Target{
			DataPlaneID: "stage",
		},
	})
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		EnvID: dev.ID, Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(),
		Resources: "resource_type: application\nid: app-a\nname: app-a",
		Variables: map[string]string{}, SecretKeys: []string{"APP_A_CLIENT_SECRET"},
	})

	if _, err := svc.Promote(context.Background(), PromoteInput{FromEnvID: dev.ID, ToEnvID: stage.ID}); err != nil {
		t.Fatalf("promote: %v", err)
	}

	if len(fake.kvWrites) != 0 {
		t.Fatalf("the destination's credential must be left alone, got %v", fake.kvWrites)
	}
}

// A credential is created once, in the organization's workspace, and belongs to the one environment
// the control plane administers directly. Sending it everywhere would set the credential running in
// production from a change made while developing.
func TestCaptureSecretReachesOnlyTheControlPlaneManagedEnvironment(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	dev, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Rank: intp(1), Target: model.Target{DataPlaneID: "dev-dp"},
	})
	_, _ = svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "prod", Rank: intp(2), Target: model.Target{DataPlaneID: "prod-dp"},
	})

	// The first environment takes the mark, so a credential has somewhere to go from the outset.
	stored, _ := svc.GetEnvironment(context.Background(), dev.ID)
	if !stored.ManagedByControlPlane {
		t.Fatal("the organization's first environment must be the one the control plane manages")
	}

	delivered, err := svc.CaptureSecretForTenant(context.Background(), "acme", "MY_SECRET",
		map[string]interface{}{"kind": "hash", "value": "h", "algorithm": "PBKDF2"})
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if delivered != 1 {
		t.Fatalf("expected the secret sent to one environment, got %d", delivered)
	}
	if fake.kvWrites["MY_SECRET"]["kind"] != "hash" {
		t.Fatalf("expected the credential stored, got %#v", fake.kvWrites)
	}
}

// The mark moves, and it moves rather than toggling: an organization is never left without one.
func TestSetManagedEnvironmentMovesTheMark(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	dev, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Rank: intp(1), Target: model.Target{DataPlaneID: "dev-dp"},
	})
	prod, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "prod", Rank: intp(2), Target: model.Target{DataPlaneID: "prod-dp"},
	})

	if _, err := svc.SetManagedEnvironment(context.Background(), prod.ID); err != nil {
		t.Fatalf("set managed: %v", err)
	}

	movedTo, _ := svc.GetEnvironment(context.Background(), prod.ID)
	movedFrom, _ := svc.GetEnvironment(context.Background(), dev.ID)
	if !movedTo.ManagedByControlPlane {
		t.Fatal("the named environment should hold the mark")
	}
	if movedFrom.ManagedByControlPlane {
		t.Fatal("the environment that held it should have given it up")
	}
}

// Removing the marked environment hands the mark to what is left, so a credential created afterwards
// still has somewhere to go.
func TestDeletingTheManagedEnvironmentPassesTheMarkOn(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	dev, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Rank: intp(1), Target: model.Target{DataPlaneID: "dev-dp"},
	})
	stage, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "stage", Rank: intp(2), Target: model.Target{DataPlaneID: "stage-dp"},
	})

	if err := svc.DeleteEnvironment(context.Background(), dev.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	successor, _ := svc.GetEnvironment(context.Background(), stage.ID)
	if !successor.ManagedByControlPlane {
		t.Fatal("the mark should have passed to the environment left lowest in the chain")
	}
}

// An organization with no environment yet is not an error: there is simply nowhere to send it, and
// the credential is recreated when one is registered.
func TestCaptureSecretWithNoEnvironmentsDeliversNothing(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	if n, err := svc.CaptureSecretForTenant(context.Background(), "acme", "X",
		map[string]interface{}{"kind": "value", "value": "v"}); err != nil || n != 0 {
		t.Fatalf("expected zero deliveries and no error, got %d %v", n, err)
	}
}

func TestVersionHistoryPruned(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})
	for i := 0; i < 7; i++ {
		if _, err := svc.UploadVersion(context.Background(), env.ID, bundleOf("app-"+strconv.Itoa(i)), nil,
			"v"); err != nil {
			t.Fatalf("upload: %v", err)
		}
	}
	versions, _ := svc.ListVersions(context.Background(), env.ID)
	if len(versions) != store.KeepPrevious+1 {
		t.Fatalf("expected %d versions retained, got %d", store.KeepPrevious+1, len(versions))
	}
	// Newest first: seq 7 down to 4.
	if versions[0].Seq != 7 || versions[len(versions)-1].Seq != 4 {
		t.Fatalf("unexpected retained range: %d..%d", versions[0].Seq, versions[len(versions)-1].Seq)
	}
}

func intp(i int) *int { return &i }

func TestCheckVariablesClassifiesACredentialInAnOlderVersion(t *testing.T) {
	svc := newTestService(t, &fakeClient{secretNames: []string{"ADMIN2_WSO2_COM_PASSWORD"}})

	env, err := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev",
		Target: model.Target{
			DataPlaneID: "dp",
		},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	// A version captured before credentials were classified: SecretKeys is empty even though the
	// bundle plainly holds a password.
	if _, err := svc.UploadVersion(context.Background(), env.ID,
		"resource_type: user\ncredentials:\n  password: \"{{.ADMIN2_WSO2_COM_PASSWORD}}\"\n",
		map[string]string{}, "captured earlier"); err != nil {
		t.Fatalf("upload version: %v", err)
	}

	status, err := svc.CheckVariables(context.Background(), env.ID, "latest")
	if err != nil {
		t.Fatalf("check variables: %v", err)
	}
	if len(status.Missing) != 0 {
		t.Fatalf("a password is not a missing variable, got %v", status.Missing)
	}
	if len(status.SecretBacked) != 1 || status.SecretBacked[0] != "ADMIN2_WSO2_COM_PASSWORD" {
		t.Fatalf("the password should be reported as secret backed, got %v", status.SecretBacked)
	}
	if len(status.MissingSecrets) != 0 {
		t.Fatalf("the secret service holds it, so nothing is missing, got %v", status.MissingSecrets)
	}
}

// setupPromotionPair builds a dev -> prod pair with two applications in dev.
func setupPromotionPair(t *testing.T, fake *fakeClient) (*Service, model.Environment, model.Environment) {
	t.Helper()
	svc := newTestService(t, fake)
	dev, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Rank: intp(1), Target: model.Target{DataPlaneID: "dev"}})
	prod, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "prod", Rank: intp(2), Target: model.Target{DataPlaneID: "prod"}})
	if _, err := svc.UploadVersion(context.Background(), dev.ID, bundleOf("app-a", testAppB), nil,
		"dev-v1"); err != nil {
		t.Fatalf("upload: %v", err)
	}
	return svc, dev.Environment, prod.Environment
}

func TestPromoteRemembersWhatWasHeldBack(t *testing.T) {
	svc, dev, prod := setupPromotionPair(t, &fakeClient{})

	// The user promotes app-a and deliberately leaves app-b behind.
	if _, err := svc.Promote(context.Background(), PromoteInput{
		FromEnvID: dev.ID, ToEnvID: prod.ID,
		Selection: []string{"application/id:app-a"}, SelectionProvided: true,
	}); err != nil {
		t.Fatalf("promote: %v", err)
	}

	env, err := svc.GetEnvironment(context.Background(), prod.ID)
	if err != nil {
		t.Fatalf("get environment: %v", err)
	}
	if len(env.Excluded) != 1 || env.Excluded[0] != "application/id:app-b" {
		t.Fatalf("the held back resource should have been recorded, got %v", env.Excluded)
	}
}

func TestPromoteKeepsHoldingBackOnALaterRun(t *testing.T) {
	svc, dev, prod := setupPromotionPair(t, &fakeClient{})

	if _, err := svc.Promote(context.Background(), PromoteInput{
		FromEnvID: dev.ID, ToEnvID: prod.ID,
		Selection: []string{"application/id:app-a"}, SelectionProvided: true,
	}); err != nil {
		t.Fatalf("first promote: %v", err)
	}

	// dev changes again and the user promotes without expressing a preference. The earlier decision
	// stands: asking again every time is how a held back resource eventually slips through.
	_, err := svc.UploadVersion(context.Background(), dev.ID, bundleOf("app-a", testAppB, "app-c"), nil, "dev-v2")
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	result, err := svc.Promote(context.Background(), PromoteInput{FromEnvID: dev.ID, ToEnvID: prod.ID})
	if err != nil {
		t.Fatalf("second promote: %v", err)
	}

	full, _ := svc.GetVersion(context.Background(), prod.ID, result.NewVersion.Seq)
	if strings.Contains(full.Resources, testAppB) {
		t.Fatalf("app-b was held back earlier and must stay held back:\n%s", full.Resources)
	}
	if !strings.Contains(full.Resources, "app-c") {
		t.Fatalf("a resource nobody held back should promote:\n%s", full.Resources)
	}
}

func TestPromoteReleasesAResourceWhenItIsSelectedAgain(t *testing.T) {
	svc, dev, prod := setupPromotionPair(t, &fakeClient{})

	if _, err := svc.Promote(context.Background(), PromoteInput{
		FromEnvID: dev.ID, ToEnvID: prod.ID,
		Selection: []string{"application/id:app-a"}, SelectionProvided: true,
	}); err != nil {
		t.Fatalf("first promote: %v", err)
	}

	// The user changes their mind and selects app-b this time.
	result, err := svc.Promote(context.Background(), PromoteInput{
		FromEnvID: dev.ID, ToEnvID: prod.ID,
		Selection: []string{"application/id:app-b"}, SelectionProvided: true,
	})
	if err != nil {
		t.Fatalf("second promote: %v", err)
	}

	full, _ := svc.GetVersion(context.Background(), prod.ID, result.NewVersion.Seq)
	if !strings.Contains(full.Resources, testAppB) {
		t.Fatalf("app-b was selected and should have promoted:\n%s", full.Resources)
	}
	env, _ := svc.GetEnvironment(context.Background(), prod.ID)
	for _, key := range env.Excluded {
		if key == "application/id:app-b" {
			t.Fatal("selecting a resource again must clear the record, or it would be held back forever")
		}
	}
}

func TestApplyLeavesAHeldBackResourceAlone(t *testing.T) {
	fake := &fakeClient{}
	svc, dev, prod := setupPromotionPair(t, fake)

	if _, err := svc.Promote(context.Background(), PromoteInput{
		FromEnvID: dev.ID, ToEnvID: prod.ID,
		Selection: []string{"application/id:app-a"}, SelectionProvided: true,
	}); err != nil {
		t.Fatalf("promote: %v", err)
	}
	if _, err := svc.Apply(context.Background(), prod.ID, "latest", false); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Held back means left alone on the data plane: neither pushed nor deleted.
	if strings.Contains(fake.lastImport().Content, testAppB) {
		t.Fatalf("a held back resource must not be applied:\n%s", fake.lastImport().Content)
	}
	for _, d := range fake.lastImport().Deletions {
		if d.ID == testAppB {
			t.Fatal("holding a resource back means leaving it alone, not deleting it from the data plane")
		}
	}
}

// The promotion view shows whether each environment's data plane is connected, because nothing can be
// applied or promoted to one that is not, and an operator should see that before starting.
func TestEnvironmentSummariesReportDataPlaneConnectivity(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	planes := &fakeDataPlanes{plane: fake, connected: true}
	svc.SetDataPlanes(planes)
	if _, err := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	summaries, err := svc.ListEnvironmentSummaries(context.Background())
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if !summaries[0].DataPlane.Connected {
		t.Fatal("a connected data plane should be reported as connected")
	}

	planes.connected = false
	summaries, err = svc.ListEnvironmentSummaries(context.Background())
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if summaries[0].DataPlane.Connected {
		t.Fatal("a data plane that dropped should be reported as disconnected")
	}
}

// An environment that names no data plane cannot be applied to, and says so rather than failing at the
// transport with something that reads like an outage.
// A data plane this pod cannot reach is no longer a failure: the work is queued for whichever pod
// holds that connection, and the caller is given the id to collect the answer with.
func TestApplyQueuesWhenThisPodCannotReachTheDataPlane(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	svc.SetDataPlanes(&fakeDataPlanes{plane: fake, connected: false})
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		EnvID: env.ID, Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(), Resources: app("a"),
	})

	result, err := svc.Apply(context.Background(), env.ID, "latest", false)

	if err != nil {
		t.Fatalf("expected the apply to be queued, got %v", err)
	}
	if result.JobID == "" {
		t.Fatal("expected a job id to collect the answer with")
	}
	if result.Status != store.JobPending {
		t.Fatalf("expected the job to be left pending, got %q", result.Status)
	}
	if result.Import != nil {
		t.Fatal("expected no import result, because nothing was delivered")
	}

	// Nothing was applied, so the environment must not claim to hold the version.
	stored, _ := svc.store.GetEnvironment(context.Background(), env.ID)
	if stored.AppliedSeq != 0 {
		t.Fatalf("expected the environment to record nothing applied, got %d", stored.AppliedSeq)
	}
}

// The pod that holds the connection finishes the work in the same request, so the caller gets its
// answer without polling.
func TestApplyDeliversInlineWhenThisPodHoldsTheConnection(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		EnvID: env.ID, Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(), Resources: app("a"),
	})

	result, err := svc.Apply(context.Background(), env.ID, "latest", false)

	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if result.Status != store.JobDone {
		t.Fatalf("expected the job to be finished, got %q", result.Status)
	}
	if result.Import == nil {
		t.Fatal("expected the data plane's answer in the same response")
	}

	stored, _ := svc.store.GetEnvironment(context.Background(), env.ID)
	if stored.AppliedSeq != result.TargetSeq {
		t.Fatalf("expected the environment to record version %d applied, got %d",
			result.TargetSeq, stored.AppliedSeq)
	}
}

// fakeTokenIssuer records what it was asked to issue and hands back a predictable token.
type fakeTokenIssuer struct {
	issued []string
	n      int
}

func (f *fakeTokenIssuer) Issue(_ context.Context, dataPlaneID, _ string) (string, error) {
	f.issued = append(f.issued, dataPlaneID)
	f.n++
	return fmt.Sprintf("token-%d", f.n), nil
}

// Registering an environment mints the credential its data plane connects with, and returns it once.
// Asking an operator to invent one and configure it on both sides is the step this removes.
func TestCreateEnvironmentIssuesTheDataPlaneToken(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	issuer := &fakeTokenIssuer{}
	svc.SetDataPlaneTokenIssuer(issuer)

	env, err := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name:   "dev",
		Target: model.Target{DataPlaneID: "org1:dev"},
	})

	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if env.DataPlaneToken != "token-1" {
		t.Fatalf("expected the issued token to be returned once, got %q", env.DataPlaneToken)
	}
	if len(issuer.issued) != 1 || issuer.issued[0] != "org1:dev" {
		t.Fatalf("expected a token for the environment's data plane, got %v", issuer.issued)
	}
}

// A deployment using a single shared token issues none, and registering an environment still works.
func TestCreateEnvironmentWithoutAnIssuerReturnsNoToken(t *testing.T) {
	svc := newTestService(t, &fakeClient{})

	env, err := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Target: model.Target{DataPlaneID: "org1:dev"},
	})

	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if env.DataPlaneToken != "" {
		t.Fatalf("expected no token, got %q", env.DataPlaneToken)
	}
}

// Rotation issues a new token for the same data plane.

// Rotation issues a new token for the same data plane.
func TestRegenerateDataPlaneTokenIssuesAnotherForTheSameDataPlane(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	issuer := &fakeTokenIssuer{}
	svc.SetDataPlaneTokenIssuer(issuer)
	env, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Target: model.Target{DataPlaneID: "org1:dev"},
	})

	token, err := svc.RegenerateDataPlaneToken(context.Background(), env.ID)

	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if token != "token-2" {
		t.Fatalf("expected a freshly issued token, got %q", token)
	}
	if len(issuer.issued) != 2 || issuer.issued[1] != "org1:dev" {
		t.Fatalf("expected the same data plane to be reissued, got %v", issuer.issued)
	}
}

// A promotion compares against what the destination is running, not its newest capture. Captures pile
// up as drafts; until one is applied the environment has not moved, and comparing against the newest
// would describe a destination state that nothing is running.
func TestPromoteComparesAgainstWhatTheDestinationIsRunning(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	dev, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Rank: intp(1), Target: model.Target{DataPlaneID: "dev-dp"},
	})
	stage, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "stage", Rank: intp(2), Target: model.Target{DataPlaneID: "stage-dp"},
	})

	// Stage is running v1. A later capture adds app-b, but nothing has adopted it.
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		EnvID: stage.ID, Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(), Resources: app("app-a"),
	})
	if _, err := svc.Apply(context.Background(), stage.ID, "1", false); err != nil {
		t.Fatalf("apply v1 to stage: %v", err)
	}
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		EnvID: stage.ID, Origin: model.OriginCaptured, CreatedAt: svc.now().UTC(),
		Resources: bundleOf("app-a", testAppB),
	})

	// Dev is running both applications.
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		EnvID: dev.ID, Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(),
		Resources: bundleOf("app-a", testAppB),
	})
	if _, err := svc.Apply(context.Background(), dev.ID, "1", false); err != nil {
		t.Fatalf("apply to dev: %v", err)
	}

	result, err := svc.Promote(context.Background(), PromoteInput{FromEnvID: dev.ID, ToEnvID: stage.ID})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}

	// Against what stage runs, app-b is an addition. Against its newest capture it would read as
	// unchanged, and the operator would be told a promotion changes nothing when it adds a resource.
	if result.Preview.Summary.Added != 1 {
		t.Fatalf("expected app-b to read as an addition, got %+v", result.Preview.Summary)
	}
}

// A promotion moves what the source is running. A capture taken on top of that is a draft, and
// promoting it would push work the source itself never adopted.

// A promotion moves what the source is running. A capture taken on top of that is a draft, and
// promoting it would push work the source itself never adopted.
func TestPromoteSendsWhatTheSourceIsRunning(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	dev, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Rank: intp(1), Target: model.Target{DataPlaneID: "dev-dp"},
	})
	stage, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "stage", Rank: intp(2), Target: model.Target{DataPlaneID: "stage-dp"},
	})

	// Dev is running v1; v2 is a capture that added app-b and was never applied.
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		EnvID: dev.ID, Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(), Resources: app("app-a"),
	})
	if _, err := svc.Apply(context.Background(), dev.ID, "1", false); err != nil {
		t.Fatalf("apply v1 to dev: %v", err)
	}
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		EnvID: dev.ID, Origin: model.OriginCaptured, CreatedAt: svc.now().UTC(),
		Resources: bundleOf("app-a", testAppB),
	})

	result, err := svc.Promote(context.Background(), PromoteInput{FromEnvID: dev.ID, ToEnvID: stage.ID})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}

	promoted, err := svc.store.GetVersion(context.Background(), stage.ID, result.NewVersion.Seq)
	if err != nil {
		t.Fatalf("get promoted version: %v", err)
	}
	if strings.Contains(promoted.Resources, testAppB) {
		t.Fatalf("app-b was captured but never applied, so it must not promote:\n%s", promoted.Resources)
	}
	if !strings.Contains(promoted.Resources, "app-a") {
		t.Fatalf("expected app-a to be promoted:\n%s", promoted.Resources)
	}
}
