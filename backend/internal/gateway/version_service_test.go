// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/export"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// fakeVersionStore holds versions and applied state in memory.
type fakeVersionStore struct {
	versions []Version
	applied  map[string]*AppliedVersion
	pruned   int
	err      error
	// setErr fails recording what a gateway holds.
	setErr error
	// excluded is what each gateway is set to leave alone; excludeErr fails keeping it.
	excluded   map[string]map[string]excludedResource
	excludeErr error
}

func (f *fakeVersionStore) Add(_ context.Context, version Version) (*Version, error) {
	if f.err != nil {
		return nil, f.err
	}
	for _, kept := range f.versions {
		if kept.Hash == version.Hash {
			return nil, errors.New("duplicate hash")
		}
	}
	v := version
	v.Seq = len(f.versions) + 1
	v.CreatedAt = testCreatedAt
	f.versions = append(f.versions, v)
	answer := v
	answer.variables = ""
	return &answer, nil
}

func (f *fakeVersionStore) Find(_ context.Context, prefix string) ([]int, error) {
	if f.err != nil {
		return nil, f.err
	}
	found := []int{}
	for i := len(f.versions) - 1; i >= 0 && len(found) < 2; i-- {
		if strings.HasPrefix(f.versions[i].Hash, prefix) {
			found = append(found, f.versions[i].Seq)
		}
	}
	return found, nil
}

// hashAt returns the hash of the version at a place in order, or "" for none.
func (f *fakeVersionStore) hashAt(seq int) string {
	for _, v := range f.versions {
		if v.Seq == seq {
			return v.Hash
		}
	}
	return ""
}

func (f *fakeVersionStore) List(context.Context) ([]Version, error) {
	if f.err != nil {
		return nil, f.err
	}
	listed := make([]Version, 0, len(f.versions))
	for i := len(f.versions) - 1; i >= 0; i-- {
		listed = append(listed, Version{Seq: f.versions[i].Seq, Hash: f.versions[i].Hash, Name: f.versions[i].Name,
			Note: f.versions[i].Note})
	}
	return listed, nil
}

func (f *fakeVersionStore) Get(_ context.Context, seq int) (*Version, error) {
	if f.err != nil {
		return nil, f.err
	}
	for i := range f.versions {
		if f.versions[i].Seq == seq {
			v := f.versions[i]
			return &v, nil
		}
	}
	return nil, nil
}

func (f *fakeVersionStore) Latest(context.Context) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	return len(f.versions), nil
}

func (f *fakeVersionStore) Prune(_ context.Context, through int) error {
	f.pruned = through
	return nil
}

func (f *fakeVersionStore) GetApplied(_ context.Context, gatewayID string) (*AppliedVersion, error) {
	if f.err != nil {
		return nil, f.err
	}
	if held, ok := f.applied[gatewayID]; ok {
		copied := *held
		copied.Applied, copied.Previous = f.hashAt(held.AppliedVersion), f.hashAt(held.PreviousVersion)
		return &copied, nil
	}
	return nil, nil
}

func (f *fakeVersionStore) SetApplied(_ context.Context, gatewayID string, applied, previous, _ int) error {
	if f.setErr != nil {
		return f.setErr
	}
	if f.applied == nil {
		f.applied = map[string]*AppliedVersion{}
	}
	f.applied[gatewayID] = &AppliedVersion{GatewayID: gatewayID, AppliedVersion: applied,
		PreviousVersion: previous}
	return nil
}

func (f *fakeVersionStore) DeleteApplied(_ context.Context, gatewayID string) error {
	delete(f.applied, gatewayID)
	return nil
}

func (f *fakeVersionStore) GetExcluded(_ context.Context, gatewayID string) ([]excludedResource, error) {
	excluded := []excludedResource{}
	for _, resource := range f.excluded[gatewayID] {
		excluded = append(excluded, resource)
	}
	sort.Slice(excluded, func(i, j int) bool { return excluded[i].Key < excluded[j].Key })
	return excluded, nil
}

func (f *fakeVersionStore) SetExcluded(_ context.Context, gatewayID string, exclude []excludedResource,
	include []string) error {
	if f.excludeErr != nil {
		return f.excludeErr
	}
	if f.excluded == nil {
		f.excluded = map[string]map[string]excludedResource{}
	}
	if f.excluded[gatewayID] == nil {
		f.excluded[gatewayID] = map[string]excludedResource{}
	}
	for _, resource := range exclude {
		f.excluded[gatewayID][resource.Key] = resource
	}
	for _, key := range include {
		delete(f.excluded[gatewayID], key)
	}
	return nil
}

func (f *fakeVersionStore) DeleteExcluded(_ context.Context, gatewayID string) error {
	delete(f.excluded, gatewayID)
	return nil
}

// fakeExporter answers a capture with canned files, and a value capture with canned values.
type fakeExporter struct {
	response *export.ExportResponse
	err      *tidcommon.ServiceError
	asked    *export.ExportRequest

	variables map[string]string
	secrets   map[string]string
	valuesErr error
	// valuesOf records the resource type and resource each value capture asked about.
	valuesOf []valuesRequest
	// valuesCtxDone records whether a value capture asked with a context that was already done.
	valuesCtxDone bool
}

type valuesRequest struct {
	resourceType string
	resource     interface{}
}

func (f *fakeExporter) ExportResources(_ context.Context,
	request *export.ExportRequest) (*export.ExportResponse, *tidcommon.ServiceError) {
	f.asked = request
	return f.response, f.err
}

func (f *fakeExporter) PlaceholderValues(ctx context.Context, resourceType string,
	resource interface{}) (map[string]string, map[string]string, error) {
	f.valuesOf = append(f.valuesOf, valuesRequest{resourceType: resourceType, resource: resource})
	f.valuesCtxDone = f.valuesCtxDone || ctx.Err() != nil
	return f.variables, f.secrets, f.valuesErr
}

// fakeGatewayClient records what an apply sends a gateway.
type fakeGatewayClient struct {
	sent []gatewayImportRequest
	keys []string
	err  error
	// held are the values the gateway's store holds, by name; heldErr fails reading them.
	held    map[string]bool
	heldErr error
	// imported is the gateway's account of an import; unset, it refused nothing.
	imported json.RawMessage
}

func (f *fakeGatewayClient) UnsetValues(_ context.Context, _ *Gateway, _ string,
	variables, secrets []string) (*MissingValues, error) {
	if f.heldErr != nil {
		return nil, f.heldErr
	}
	return &MissingValues{Variables: unheld(variables, f.held), Secrets: unheld(secrets, f.held)}, nil
}

func (f *fakeGatewayClient) Import(_ context.Context, _ *Gateway, key string,
	req gatewayImportRequest) (json.RawMessage, error) {
	f.sent = append(f.sent, req)
	f.keys = append(f.keys, key)
	if f.err != nil {
		return nil, f.err
	}
	if f.imported != nil {
		return f.imported, nil
	}
	return json.RawMessage(`{"summary":{"failed":0},"results":[]}`), nil
}

func (f *fakeGatewayClient) last() gatewayImportRequest { return f.sent[len(f.sent)-1] }

func unit(id string) string {
	return "resource_type: organization_unit\nid: " + id + "\nname: " + id
}

func exportOf(env string, ids ...string) *export.ExportResponse {
	response := &export.ExportResponse{}
	for _, id := range ids {
		response.Files = append(response.Files, export.ExportFile{Content: unit(id)})
	}
	if env != "" {
		response.EnvFile = &export.EnvironmentFile{Content: env}
	}
	return response
}

type versionFixture struct {
	svc      VersionServiceInterface
	versions *fakeVersionStore
	exporter *fakeExporter
	client   *fakeGatewayClient
}

// newVersionFixture builds a service administering one registered gateway, gw-1, whose key is sealed.
func newVersionFixture(t *testing.T) *versionFixture {
	t.Helper()
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })

	property, err := cmodels.NewProperty("key", "the-key", true)
	require.NoError(t, err)
	sealedKey, err := cmodels.SerializePropertiesToJSONArray([]cmodels.Property{*property})
	require.NoError(t, err)

	f := &versionFixture{
		versions: &fakeVersionStore{},
		exporter: &fakeExporter{response: exportOf("", "ou-a")},
		client:   &fakeGatewayClient{},
	}
	gateways := &fakeStore{gateways: []Gateway{{ID: "gw-1", Name: "dev", BaseURL: "https://dp.test", Key: sealedKey}}}
	f.svc = newVersionService(gateways, f.versions, f.exporter, f.client)
	return f
}

// hash returns the hash of the version at a place in order, which is how an apply names it.
func (f *versionFixture) hash(seq int) string { return f.versions.hashAt(seq) }

func (f *versionFixture) capture(t *testing.T, ids ...string) *Version {
	t.Helper()
	f.exporter.response = exportOf("", ids...)
	v, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	return v
}

// A capture records every resource through the export, users and groups included.
func TestACaptureRecordsTheExportOfEveryResource(t *testing.T) {
	f := newVersionFixture(t)

	v, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Note: "  first  "})

	require.Nil(t, svcErr)
	assert.Equal(t, 1, v.Seq)
	assert.Equal(t, "first", v.Note)
	assert.Contains(t, v.Resources, "ou-a")
	assert.NotEmpty(t, f.exporter.asked.Users)
	assert.NotEmpty(t, f.exporter.asked.Groups)
	assert.NotEmpty(t, f.exporter.asked.Applications)
}

// The values an export carries beside its documents are kept, sealed, and never returned.
func TestACaptureSealsTheValuesItCarries(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = exportOf("HOST=https://app.test\nSECRET=s3cret\n# comment\n", "ou-a")

	v, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})

	require.Nil(t, svcErr)
	properties, err := cmodels.DeserializePropertiesFromJSON(f.versions.versions[0].variables)
	require.NoError(t, err)
	require.Len(t, properties, 1)
	assert.True(t, properties[0].IsSecret(), "a version's values were not sealed as a secret")
	opened, err := properties[0].GetValue()
	require.NoError(t, err)
	assert.JSONEq(t, `{"HOST":"https://app.test","SECRET":"s3cret"}`, opened)
	assert.Empty(t, v.variables, "a capture answer carried the values")
}

// A capture keeps only the configured number of versions.
func TestACaptureRemovesVersionsBeyondTheBound(t *testing.T) {
	f := newVersionFixture(t)
	for _, id := range []string{"ou-a", "ou-b", "ou-c"} {
		f.capture(t, id)
	}
	assert.Equal(t, 3-maxVersions(), f.versions.pruned)
}

// A version is named by the SHA-256 of what it captured, and capturing the same configuration again
// answers with that version instead of keeping another.
func TestTheSameConfigurationIsTheSameVersion(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = exportOf("", "ou-a")

	first, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Name: " Spring ", Note: "first"})
	require.Nil(t, svcErr)
	again, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Name: "Other", Note: "again"})
	require.Nil(t, svcErr)

	sum := sha256.Sum256([]byte(first.Resources))
	assert.Equal(t, hex.EncodeToString(sum[:]), first.Hash)
	assert.Equal(t, "Spring", first.Name)
	assert.False(t, first.Unchanged)
	assert.True(t, again.Unchanged)
	assert.Equal(t, first.Hash, again.Hash)
	assert.Equal(t, "Spring", again.Name, "an unchanged capture renamed the version it matched")
	assert.Len(t, f.versions.versions, 1)
}

// A version is named by its hash or a prefix of at least seven of its characters; anything shorter or
// not hex is refused, and a prefix two versions share names neither.
func TestAVersionIsNamedByAPrefixOfItsHash(t *testing.T) {
	f := newVersionFixture(t)
	f.versions.versions = []Version{
		{Seq: 1, Hash: "abcdef0111", Resources: unit("ou-a")},
		{Seq: 2, Hash: "abcdef0222", Resources: unit("ou-b")},
	}

	v, svcErr := f.svc.GetVersion(context.Background(), "ABCDEF01")
	require.Nil(t, svcErr)
	assert.Equal(t, 1, v.Seq)

	for ref, code := range map[string]string{
		"abcdef0": ErrorAmbiguousVersion.Code,
		"abcdef":  ErrorInvalidVersion.Code,
		"2":       ErrorInvalidVersion.Code,
		"zzzzzzz": ErrorInvalidVersion.Code,
		"1234567": ErrorVersionNotFound.Code,
	} {
		_, svcErr := f.svc.GetVersion(context.Background(), ref)
		require.NotNil(t, svcErr, ref)
		assert.Equal(t, code, svcErr.Code, ref)
	}
}

// An export that could not write a resource would make a version that silently drops it, so nothing
// is captured.
// A resource the export could not write is left out, as the export leaves it out, and the capture's
// answer names it.
func TestACaptureSkipsWhatTheExportCouldNotWrite(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = exportOf("", "ou-a")
	failed := export.ExportError{ResourceType: "user", ResourceID: "user-1", Error: "Missing username"}
	f.exporter.response.Summary = &export.ExportSummary{Errors: []export.ExportError{failed}}

	v, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})

	require.Nil(t, svcErr)
	assert.Contains(t, v.Resources, "ou-a")
	assert.Equal(t, []export.ExportError{failed}, v.Skipped)
	require.Len(t, f.versions.versions, 1)
}

func TestACaptureReportsExportAndStoreFailures(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.err = &tidcommon.InternalServerError
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	assert.NotNil(t, svcErr)

	f.exporter.err = nil
	f.versions.err = errors.New("database is down")
	_, svcErr = f.svc.Capture(context.Background(), CaptureRequest{})
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

// A version is named by its number or by "latest".
func TestAVersionIsResolvedByNumberOrLatest(t *testing.T) {
	f := newVersionFixture(t)
	_, svcErr := f.svc.GetVersion(context.Background(), latestVersion)
	assert.Equal(t, ErrorNoVersions.Code, svcErr.Code, "nothing captured yet")

	f.capture(t, "ou-a")
	f.capture(t, "ou-b")

	latest, svcErr := f.svc.GetVersion(context.Background(), "")
	require.Nil(t, svcErr)
	assert.Equal(t, 2, latest.Seq)
	first, svcErr := f.svc.GetVersion(context.Background(), f.hash(1)[:7])
	require.Nil(t, svcErr)
	assert.Equal(t, 1, first.Seq)

	_, svcErr = f.svc.GetVersion(context.Background(), "first")
	assert.Equal(t, ErrorInvalidVersion.Code, svcErr.Code)
	_, svcErr = f.svc.GetVersion(context.Background(), "9999999")
	assert.Equal(t, ErrorVersionNotFound.Code, svcErr.Code)

	listed, svcErr := f.svc.ListVersions(context.Background())
	require.Nil(t, svcErr)
	assert.Equal(t, []int{2, 1}, []int{listed[0].Seq, listed[1].Seq})
}

// Applying sends the version whole with the gateway's own key, and records it as what the gateway
// holds.
func TestApplySendsTheVersionAndRecordsIt(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr)
	assert.True(t, result.Recorded)
	assert.Equal(t, "the-key", f.client.keys[0], "the stored key was not opened before presenting it")
	assert.Contains(t, f.client.last().Content, "ou-a")
	assert.Equal(t, "runtime", f.client.last().Options.Target)
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Equal(t, 1, held.AppliedVersion)
	assert.Zero(t, held.PreviousVersion)
}

// A later version that no longer has a resource removes it from the gateway, and the version it held
// becomes the one a revert returns to.
func TestApplyRemovesWhatTheNewVersionDropped(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	f.capture(t, "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	require.Nil(t, svcErr)

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})

	require.Nil(t, svcErr)
	assert.Equal(t, []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-b"}}, f.client.last().Deletions)
	assert.Equal(t, DiffSummary{Unchanged: 1, Deleted: 1}, result.Diff.Summary)
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Equal(t, [2]int{2, 1}, [2]int{held.AppliedVersion, held.PreviousVersion})
}

// Applying the version a gateway already holds leaves its revert target alone.
func TestReapplyingTheHeldVersionKeepsTheRevertTarget(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.capture(t, "ou-b")
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})

	require.Nil(t, svcErr)
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Equal(t, 1, held.PreviousVersion)
}

// A revert applies the version held before, and keeps the one reverted from so it can be undone.
func TestRevertReturnsToThePreviousVersion(t *testing.T) {
	f := newVersionFixture(t)
	_, svcErr := f.svc.Revert(context.Background(), "gw-1", RevertRequest{})
	assert.Equal(t, ErrorNothingToRevert.Code, svcErr.Code, "nothing was applied yet")

	f.capture(t, "ou-a")
	f.capture(t, "ou-b")
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})

	result, svcErr := f.svc.Revert(context.Background(), "gw-1", RevertRequest{})

	require.Nil(t, svcErr)
	assert.True(t, result.Recorded)
	assert.Contains(t, f.client.last().Content, "ou-a")
	assert.Equal(t, []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-b"}}, f.client.last().Deletions)
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Equal(t, [2]int{1, 2}, [2]int{held.AppliedVersion, held.PreviousVersion})
}

// A removal the gateway refused would never be sent again once the version moved on, so the apply is
// not recorded, and the next apply sends it again.
func TestAnApplyWhoseRemovalFailedIsNotRecordedAndIsRetried(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	f.capture(t, "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	require.Nil(t, svcErr)
	dropB := []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-b"}}
	refused := `{"summary":{"failed":1},"results":[{"operation":"update","status":"success"},` +
		`{"operation":"delete","status":"failed"}]}`

	f.client.imported = json.RawMessage(refused)
	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})

	require.Nil(t, svcErr)
	assert.False(t, result.Recorded)
	assert.JSONEq(t, refused, string(result.Import))
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Equal(t, 1, held.AppliedVersion, "an apply whose removal failed was recorded")

	f.client.imported = nil
	result, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})

	require.Nil(t, svcErr)
	assert.Equal(t, dropB, f.client.last().Deletions, "the failed removal was not sent again")
	assert.True(t, result.Recorded)
	held, _ = f.svc.GetApplied(context.Background(), "gw-1")
	assert.Equal(t, [2]int{2, 1}, [2]int{held.AppliedVersion, held.PreviousVersion})
}

// A refused write is sent again by every later apply, so it does not stop the version being recorded.
func TestAnApplyWithARefusedWriteIsRecorded(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.client.imported = json.RawMessage(`{"summary":{"failed":1},"results":[{"operation":"create","status":"failed"}]}`)

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr)
	assert.True(t, result.Recorded)
}

// An answer that does not say how its removals went is not taken as a clean one.
func TestAnApplyWhoseRemovalsAreUnconfirmedIsNotRecorded(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b")
	f.capture(t, "ou-a")
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	f.client.imported = json.RawMessage(`{"summary":{"failed":0}}`)

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})

	require.Nil(t, svcErr)
	assert.False(t, result.Recorded)
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Equal(t, 1, held.AppliedVersion)
}

// A dry run reaches the gateway as one and records nothing.
func TestADryRunRecordsNothing(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{DryRun: true})

	require.Nil(t, svcErr)
	assert.True(t, f.client.last().DryRun)
	assert.False(t, result.Recorded)
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Zero(t, held.AppliedVersion)
}

// A gateway that cannot be reached is reported, and nothing is recorded for it.
func TestAnUnreachableGatewayIsReportedAndNotRecorded(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.client.err = errors.New("connection refused")

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	assert.Equal(t, ErrorGatewayUnreachable.Code, svcErr.Code)
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Zero(t, held.AppliedVersion)
}

func TestApplyNamesWhatIsMissing(t *testing.T) {
	f := newVersionFixture(t)
	_, svcErr := f.svc.Apply(context.Background(), "", ApplyRequest{})
	assert.Equal(t, ErrorInvalidGatewayID.Code, svcErr.Code)
	_, svcErr = f.svc.Apply(context.Background(), "gw-none", ApplyRequest{})
	assert.Equal(t, ErrorGatewayNotFound.Code, svcErr.Code)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	assert.Equal(t, ErrorNoVersions.Code, svcErr.Code)
	_, svcErr = f.svc.Diff(context.Background(), "gw-none", "")
	assert.Equal(t, ErrorGatewayNotFound.Code, svcErr.Code)
	_, svcErr = f.svc.GetApplied(context.Background(), "gw-none")
	assert.Equal(t, ErrorGatewayNotFound.Code, svcErr.Code)
}

// A version's values reach the gateway, with a list given as a list rather than as JSON text.
func TestApplySendsTheVersionsValues(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = exportOf(`HOST=https://app.test`+"\n"+`URIS=["https://a.test","https://b.test"]`, "ou-a")
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr)
	assert.Equal(t, "https://app.test", f.client.last().Variables["HOST"])
	assert.Equal(t, []interface{}{"https://a.test", "https://b.test"}, f.client.last().Variables["URIS"])
}

// A gateway declared in a file holds its key as the file wrote it, so it is presented as it is.
func TestADeclaredGatewaysKeyIsPresentedAsWritten(t *testing.T) {
	f := newVersionFixture(t)
	declared := &fakeStore{gateways: []Gateway{{ID: "gw-d", BaseURL: "https://dp.test", Key: "plain-key"}}}
	svc := newVersionService(declared, f.versions, f.exporter, f.client)
	f.capture(t, "ou-a")

	_, svcErr := svc.Apply(context.Background(), "gw-d", ApplyRequest{})

	require.Nil(t, svcErr)
	assert.Equal(t, "plain-key", f.client.keys[0])
}

// The diff compares with what the gateway holds and changes nothing.
func TestTheDiffChangesNothing(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")

	diff, svcErr := f.svc.Diff(context.Background(), "gw-1", "latest")

	require.Nil(t, svcErr)
	assert.Equal(t, DiffSummary{Added: 1}, diff.Summary)
	assert.Empty(t, f.client.sent)
}

// Removing a gateway forgets what it held.
func TestForgetDropsWhatTheGatewayHeld(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	f.svc.Forget(context.Background(), "gw-1")

	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Zero(t, held.AppliedVersion)
}

func TestAStoreFailureIsAnInternalErrorForEveryRead(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.versions.err = errors.New("database is down")

	for name, call := range map[string]func() *tidcommon.ServiceError{
		"list": func() *tidcommon.ServiceError { _, e := f.svc.ListVersions(context.Background()); return e },
		"get": func() *tidcommon.ServiceError {
			_, e := f.svc.GetVersion(context.Background(), "aaaaaaa")
			return e
		},
		"latest":  func() *tidcommon.ServiceError { _, e := f.svc.GetVersion(context.Background(), ""); return e },
		"applied": func() *tidcommon.ServiceError { _, e := f.svc.GetApplied(context.Background(), "gw-1"); return e },
		"revert": func() *tidcommon.ServiceError {
			_, e := f.svc.Revert(context.Background(), "gw-1", RevertRequest{})
			return e
		},
	} {
		svcErr := call()
		require.NotNil(t, svcErr, name)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code, name)
	}
}

// captureReferring captures one application that names a variable and a secret held by the gateway.
func (f *versionFixture) captureReferring(t *testing.T) {
	t.Helper()
	f.exporter.response = &export.ExportResponse{Files: []export.ExportFile{{Content: "resource_type: application\n" +
		"id: app-1\nname: app\nclient_id: var:APP_CLIENT_ID\nclient_secret: \"sec:APP_SECRET\"\n"}}}
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
}

// An apply that would leave a resource naming a value the gateway lacks is refused before anything
// is sent, while a dry run reports what is missing.
func TestApplyIsRefusedWhileTheGatewayLacksAValue(t *testing.T) {
	f := newVersionFixture(t)
	f.captureReferring(t)
	f.client.held = map[string]bool{"APP_CLIENT_ID": true}

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorMissingValues.Code, svcErr.Code)
	assert.Empty(t, f.client.sent, "an apply missing a value reached the gateway")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{DryRun: true})
	require.Nil(t, svcErr)
	assert.Equal(t, &MissingValues{Secrets: []string{"APP_SECRET"}}, result.Missing)
	assert.True(t, f.client.last().DryRun)
}

// Once the gateway holds every value, the apply goes ahead and reports nothing missing.
func TestApplyProceedsWhenTheGatewayHoldsEveryValue(t *testing.T) {
	f := newVersionFixture(t)
	f.captureReferring(t)
	f.client.held = map[string]bool{"APP_CLIENT_ID": true, "APP_SECRET": true}

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr)
	assert.True(t, result.Recorded)
	assert.Nil(t, result.Missing)
}

// A gateway whose store cannot be read is reported as unreachable, and nothing is sent to it.
func TestApplyReportsAStoreThatCannotBeRead(t *testing.T) {
	f := newVersionFixture(t)
	f.captureReferring(t)
	f.client.heldErr = errors.New("connection refused")

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{DryRun: true})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorGatewayUnreachable.Code, svcErr.Code)
	assert.Empty(t, f.client.sent)
}

// A version a capture removed before the apply could record it is reported, not recorded.
func TestApplyReportsAVersionRemovedWhileApplied(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.versions.setErr = errVersionRemoved

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorVersionRemoved.Code, svcErr.Code)
}

// A recorded version that is no longer kept stops the apply rather than sending it without the
// deletions it could not work out.
func TestApplyStopsWhenTheHeldVersionIsNotKept(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.versions.applied = map[string]*AppliedVersion{"gw-1": {GatewayID: "gw-1", AppliedVersion: 99}}

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	assert.Empty(t, f.client.sent, "an apply without its deletions reached the gateway")
}

// An apply that another apply to the same gateway overtook is reported, so its recorded state is not
// one its own import did not leave.
func TestApplyReportsAnotherApplyFinishingFirst(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.versions.setErr = errAppliedChanged

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorAppliedChanged.Code, svcErr.Code)
}
