// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/export"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// fakeVersionStore holds versions and applied state in memory.
type fakeVersionStore struct {
	versions []Version
	applied  map[string]*AppliedVersion
	pruned   int
	err      error
	// setErr fails recording what a gateway holds; replaceErr fails replacing the current version.
	setErr     error
	replaceErr error
	settleErr  error
	// onReplace runs as a replacement fails, and onSettle as a version is settled, standing for what
	// another capture does at the same moment.
	onReplace func(version Version)
	onSettle  func()
	// settled are the versions an apply sent, which stay current but are not replaced in place.
	settled map[int]bool
	// excluded is what each gateway is set to leave alone; excludeErr fails keeping it.
	excluded   map[string]map[string]excludedResource
	excludeErr error
	// leases are the gateways' apply leases; leaseErr fails every lease call.
	leases   map[string]*applyLease
	leaseErr error
	// onSetApplied runs once a version is recorded as applied, with its number.
	onSetApplied func(applied int)
}

func (f *fakeVersionStore) GetLease(_ context.Context, gatewayID string) (*applyLease, error) {
	if f.leaseErr != nil {
		return nil, f.leaseErr
	}
	if lease, ok := f.leases[gatewayID]; ok {
		copied := *lease
		return &copied, nil
	}
	return nil, nil
}

func (f *fakeVersionStore) ClaimLease(_ context.Context, gatewayID, token string) (bool, error) {
	if f.leaseErr != nil {
		return false, f.leaseErr
	}
	if f.leases == nil {
		f.leases = map[string]*applyLease{}
	}
	lease, ok := f.leases[gatewayID]
	if !ok {
		lease = &applyLease{}
		f.leases[gatewayID] = lease
	}
	if lease.Token != "" && time.Since(lease.Since) <= leaseTimeout {
		return false, nil
	}
	lease.Token, lease.Since = token, time.Now()
	return true, nil
}

func (f *fakeVersionStore) HoldsLease(_ context.Context, gatewayID, token string) (bool, error) {
	lease, ok := f.leases[gatewayID]
	return ok && lease.Token == token, nil
}

func (f *fakeVersionStore) ReleaseLease(_ context.Context, gatewayID, token string) error {
	if lease, ok := f.leases[gatewayID]; ok && lease.Token == token {
		lease.Token, lease.Since = "", time.Time{}
	}
	return nil
}

func (f *fakeVersionStore) MarkPendingLatest(_ context.Context, gatewayID string, by *Actor) (bool, error) {
	lease, ok := f.leases[gatewayID]
	if !ok || lease.Token == "" {
		return false, nil
	}
	lease.PendingLatest, lease.PendingBy = true, by
	return true, nil
}

func (f *fakeVersionStore) TakePendingLatest(_ context.Context, gatewayID string) (*Actor, bool, error) {
	lease, ok := f.leases[gatewayID]
	if !ok || !lease.PendingLatest {
		return nil, false, nil
	}
	by := lease.PendingBy
	lease.PendingLatest, lease.PendingBy = false, nil
	return by, true, nil
}

func (f *fakeVersionStore) DeleteLease(_ context.Context, gatewayID string) error {
	delete(f.leases, gatewayID)
	return nil
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

// ReplaceCurrent replaces the newest version when it was captured as current and no gateway holds
// it or could be reverted to it, as the store's statement does.
func (f *fakeVersionStore) ReplaceCurrent(_ context.Context, version Version) (*Version, error) {
	if f.replaceErr != nil {
		if f.onReplace != nil {
			f.onReplace(version)
		}
		return nil, f.replaceErr
	}
	if len(f.versions) == 0 {
		return nil, nil
	}
	newest := &f.versions[len(f.versions)-1]
	if !newest.Current || f.settled[newest.Seq] {
		return nil, nil
	}
	for _, held := range f.applied {
		if held.AppliedVersion == newest.Seq || held.PreviousVersion == newest.Seq {
			return nil, nil
		}
	}
	newest.Hash, newest.Name, newest.Note = version.Hash, version.Name, version.Note
	newest.Resources, newest.variables = version.Resources, version.variables
	newest.CreatedBy = version.CreatedBy
	answer := *newest
	answer.variables = ""
	return &answer, nil
}

// Settle keeps the version with the number and hash, when there is one, as it is from now on.
func (f *fakeVersionStore) Settle(_ context.Context, seq int, hash string) (bool, error) {
	if f.onSettle != nil {
		f.onSettle()
	}
	if f.settleErr != nil {
		return false, f.settleErr
	}
	for i := range f.versions {
		if f.versions[i].Seq == seq && f.versions[i].Hash == hash {
			if f.settled == nil {
				f.settled = make(map[int]bool)
			}
			f.settled[seq] = true
			return true, nil
		}
	}
	return false, nil
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
			Note: f.versions[i].Note, Current: f.versions[i].Current, CreatedBy: f.versions[i].CreatedBy})
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

func (f *fakeVersionStore) Unsettle(_ context.Context, seq int, hash string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if len(f.versions) == 0 || !f.settled[seq] {
		return false, nil
	}
	newest := f.versions[len(f.versions)-1]
	if newest.Seq != seq || newest.Hash != hash {
		return false, nil
	}
	for _, held := range f.applied {
		if held.AppliedVersion == seq || held.PreviousVersion == seq {
			return false, nil
		}
	}
	delete(f.settled, seq)
	return true, nil
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

func (f *fakeVersionStore) SetApplied(_ context.Context, gatewayID string, applied int, hash string,
	previous, _ int, by *Actor, values, token string) error {
	if f.setErr != nil {
		return f.setErr
	}
	if f.hashAt(applied) != hash {
		return errVersionRemoved
	}
	if lease, ok := f.leases[gatewayID]; !ok || lease.Token != token {
		return errAppliedChanged
	}
	if f.applied == nil {
		f.applied = map[string]*AppliedVersion{}
	}
	now := time.Now()
	f.applied[gatewayID] = &AppliedVersion{GatewayID: gatewayID, AppliedVersion: applied,
		PreviousVersion: previous, AppliedAt: &now, AppliedBy: by, heldValues: values}
	if f.onSetApplied != nil {
		f.onSetApplied(applied)
	}
	return nil
}

func (f *fakeVersionStore) DeleteApplied(_ context.Context, gatewayID string) error {
	delete(f.applied, gatewayID)
	return nil
}

func (f *fakeVersionStore) GetExcluded(_ context.Context, gatewayID string) ([]excludedResource, error) {
	excluded := make([]excludedResource, 0, len(f.excluded[gatewayID]))
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
	// As the store does: the include keys come off first, then each exclude is written, and one
	// already held keeps its row with ONCE = held ONCE AND new ONCE, as the upsert's conflict clause.
	for _, key := range include {
		delete(f.excluded[gatewayID], key)
	}
	for _, resource := range exclude {
		if held, ok := f.excluded[gatewayID][resource.Key]; ok {
			held.Once = held.Once && resource.Once
			f.excluded[gatewayID][resource.Key] = held
			continue
		}
		f.excluded[gatewayID][resource.Key] = resource
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
	// held are the values the gateway's store holds, by name; heldSince is when each was set, for
	// those that say; heldErr fails reading them.
	held      map[string]bool
	heldSince map[string]time.Time
	heldErr   error
	// imported is the gateway's account of an import; unset, it refused nothing.
	imported json.RawMessage
	// onImport runs while an import is in flight, for what may change at that moment.
	onImport func()
}

func (f *fakeGatewayClient) HeldValues(_ context.Context, _ *Gateway, _ string,
	variables, secrets []string) (*HeldValues, error) {
	if f.heldErr != nil {
		return nil, f.heldErr
	}
	held := &HeldValues{Variables: map[string]time.Time{}, Secrets: map[string]time.Time{}}
	for _, name := range variables {
		if f.held[name] {
			held.Variables[name] = f.heldSince[name]
		}
	}
	for _, name := range secrets {
		if f.held[name] {
			held.Secrets[name] = f.heldSince[name]
		}
	}
	return held, nil
}

func (f *fakeGatewayClient) Import(_ context.Context, _ *Gateway, key string,
	req gatewayImportRequest) (json.RawMessage, error) {
	f.sent = append(f.sent, req)
	f.keys = append(f.keys, key)
	if f.onImport != nil && !req.DryRun {
		f.onImport()
	}
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

// testSecretFields are the fields a secret may stand in, by resource type, as the exporters name them.
var testSecretFields = map[string][]string{
	"application": {"inboundAuthConfig[].config.clientSecret", "client_secret"},
}

// application returns an application document whose client secret and description are given.
func application(id, clientSecret, description string) string {
	return "resource_type: application\nid: " + id + "\nname: " + id + "\ndescription: " + description +
		"\ninboundAuthConfig:\n  - type: oauth2\n    config:\n      clientSecret: " + clientSecret
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
	names    *fakeActorNames
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
		names:    &fakeActorNames{names: map[string]string{"u-1": "alice", "u-2": "bob"}},
	}
	gateways := &fakeStore{gateways: []Gateway{{ID: "gw-1", Name: "dev", BaseURL: "https://dp.test", Key: sealedKey}}}
	f.svc = newVersionService(gateways, f.versions, f.exporter, f.client, testSecretFields, f.names)
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

// signedIn is a request's context for a user signed in with the given claims.
// fakeActorNames names the users a lookup at the issuer would, from a fixed map.
type fakeActorNames struct {
	names map[string]string
	fills int
}

func (f *fakeActorNames) fill(_ context.Context, actors ...*Actor) {
	f.fills++
	for _, actor := range actors {
		if actor != nil && actor.Name == "" {
			actor.Name = f.names[actor.ID]
		}
	}
}

func signedIn(id string, claims map[string]interface{}) context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(id, "", "", nil, claims))
}

// A capture is kept for the user who asked for it, and a current version replaced in place for the
// user whose change replaced it.
func TestACaptureIsKeptForWhoAskedForIt(t *testing.T) {
	f := newVersionFixture(t)

	v, svcErr := f.svc.Capture(signedIn("u-1", map[string]interface{}{"username": "alice"}),
		CaptureRequest{Current: true})
	require.Nil(t, svcErr)
	assert.Equal(t, &Actor{ID: "u-1", Name: "alice"}, v.CreatedBy)

	f.exporter.response = exportOf("", "ou-b")
	replaced, svcErr := f.svc.Capture(withActor(context.Background(), &Actor{ID: "u-2", Name: "bob"}),
		CaptureRequest{Current: true})
	require.Nil(t, svcErr)
	assert.Equal(t, 1, replaced.Seq)
	assert.Equal(t, &Actor{ID: "u-2", Name: "bob"}, replaced.CreatedBy)
	listed, _ := f.svc.ListVersions(context.Background())
	assert.Equal(t, &Actor{ID: "u-2", Name: "bob"}, listed[0].CreatedBy)
}

// A capture keeps only the configured number of versions.
func TestACaptureRemovesVersionsBeyondTheBound(t *testing.T) {
	f := newVersionFixture(t)
	for _, id := range []string{"ou-a", "ou-b", "ou-c"} {
		f.capture(t, id)
	}
	assert.Equal(t, 3-maxVersions(), f.versions.pruned)
}

// A capture asked to keep the configuration current replaces the current version until a gateway
// holds it, then adds a new current version; only the newest is listed as current.
func TestACurrentCaptureKeepsOneVersionUntilAGatewayHoldsIt(t *testing.T) {
	f := newVersionFixture(t)
	current := func(ids ...string) *Version {
		t.Helper()
		f.exporter.response = exportOf("", ids...)
		v, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})
		require.Nil(t, svcErr)
		return v
	}

	first := current("ou-a")
	second := current("ou-b")
	assert.Equal(t, 1, first.Seq)
	assert.Equal(t, 1, second.Seq, "a change made a new version rather than updating the current one")
	assert.True(t, second.Current)
	assert.Len(t, f.versions.versions, 1)
	assert.Contains(t, f.versions.versions[0].Resources, "ou-b")

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: second.Hash})
	require.Nil(t, svcErr)
	third := current("ou-c")
	assert.Equal(t, 2, third.Seq, "the version a gateway holds was replaced")
	assert.Contains(t, f.versions.versions[0].Resources, "ou-b")

	listed, svcErr := f.svc.ListVersions(context.Background())
	require.Nil(t, svcErr)
	require.Len(t, listed, 2)
	assert.True(t, listed[0].Current)
	assert.False(t, listed[1].Current, "a version a gateway held was still listed as current")
}

// A capture that is not asked to keep the configuration current adds a version even after a current
// one, and a later current capture does not replace it.
func TestAPlainCaptureIsAlwaysKept(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = exportOf("", "ou-a")
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})
	require.Nil(t, svcErr)

	kept := f.capture(t, "ou-b")
	f.exporter.response = exportOf("", "ou-c")
	next, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})
	require.Nil(t, svcErr)

	assert.Equal(t, 2, kept.Seq)
	assert.Equal(t, 3, next.Seq)
	assert.Len(t, f.versions.versions, 3)
}

// A current capture that cannot replace the current version fails rather than adding one.
func TestACurrentCaptureReportsAFailureToReplace(t *testing.T) {
	f := newVersionFixture(t)
	f.versions.replaceErr = errors.New("database is down")

	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})

	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	assert.Empty(t, f.versions.versions)
}

// A current capture that collides with the same configuration captured by another at the same moment
// answers with the version the other kept.
func TestACurrentCaptureCollidingWithAnotherAnswersWithItsVersion(t *testing.T) {
	f := newVersionFixture(t)
	f.versions.replaceErr = errors.New("duplicate key value violates unique constraint")
	f.versions.onReplace = func(version Version) {
		f.versions.versions = append(f.versions.versions, Version{Seq: 1, Hash: version.Hash, Current: true})
	}

	v, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})

	require.Nil(t, svcErr)
	assert.Equal(t, 1, v.Seq)
	assert.Len(t, f.versions.versions, 1)
}

// A change captured while a gateway imports the current version adds a version rather than rewriting
// the one the gateway is receiving, so the apply is recorded as what the gateway was sent.
func TestAChangeCapturedDuringAnApplyAddsAVersion(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = exportOf("", "ou-a")
	v, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})
	require.Nil(t, svcErr)
	f.client.onImport = func() {
		f.exporter.response = exportOf("", "ou-b")
		_, captureErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})
		require.Nil(t, captureErr)
	}

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: v.Hash})

	require.Nil(t, svcErr)
	assert.True(t, result.Recorded)
	require.Len(t, f.versions.versions, 2)
	assert.Equal(t, v.Hash, f.versions.versions[0].Hash, "the version being applied was rewritten")
	assert.Equal(t, 1, f.versions.applied["gw-1"].AppliedVersion)
	listed, svcErr := f.svc.ListVersions(context.Background())
	require.Nil(t, svcErr)
	require.Len(t, listed, 2)
	assert.True(t, listed[0].Current)
	assert.False(t, listed[1].Current)
}

// The version an apply sent stays the current one until the next change, and that change adds a
// version rather than replacing it.
func TestAnAppliedCurrentVersionStaysCurrentUntilTheNextChange(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = exportOf("", "ou-a")
	v, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: v.Hash})
	require.Nil(t, svcErr)

	listed, svcErr := f.svc.ListVersions(context.Background())
	require.Nil(t, svcErr)
	require.Len(t, listed, 1)
	assert.True(t, listed[0].Current, "the version a gateway runs stopped being current before any change")
}

// An apply of a version replaced or removed after it was read is refused before anything is sent,
// and one whose version cannot be settled fails the same way.
func TestAnApplyOfAVersionThatCannotBeSettledSendsNothing(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = exportOf("", "ou-a")
	v, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})
	require.Nil(t, svcErr)
	f.versions.onSettle = func() { f.versions.versions[0].Hash = "replaced" }

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: v.Hash})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorVersionRemoved.Code, svcErr.Code)
	assert.Empty(t, f.client.sent)

	f.versions.onSettle = nil
	f.versions.versions[0].Hash = v.Hash
	f.versions.settleErr = errors.New("database is down")
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: v.Hash})

	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	assert.Empty(t, f.client.sent)
}

// An apply is not recorded when its version was replaced as the current one while it was written.
func TestAnApplyOfAReplacedVersionIsNotRecorded(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = exportOf("", "ou-a")
	v, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})
	require.Nil(t, svcErr)
	f.client.onImport = func() { f.versions.versions[0].Hash = "replaced" }

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: v.Hash})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorVersionRemoved.Code, svcErr.Code)
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
	failed := declarativeresource.ExportError{ResourceType: "user", ResourceID: "user-1", Error: "Missing username"}
	f.exporter.response.Summary = &export.ExportSummary{Errors: []declarativeresource.ExportError{failed}}

	v, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})

	require.Nil(t, svcErr)
	assert.Contains(t, v.Resources, "ou-a")
	assert.Equal(t, []declarativeresource.ExportError{failed}, v.Skipped)
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

// An apply and a revert record who made them.
func TestAnApplyRecordsWhoMadeIt(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.capture(t, "ou-b")

	alice := signedIn("u-1", map[string]interface{}{"username": "alice", "email": "alice@example.com"})
	_, svcErr := f.svc.Apply(alice, "gw-1", ApplyRequest{Version: f.hash(1)})
	require.Nil(t, svcErr)
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Equal(t, &Actor{ID: "u-1", Name: "alice"}, held.AppliedBy)

	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})
	_, svcErr = f.svc.Revert(signedIn("u-3", nil), "gw-1", RevertRequest{})
	require.Nil(t, svcErr)
	held, _ = f.svc.GetApplied(context.Background(), "gw-1")
	assert.Nil(t, held.AppliedBy, "a user the issuer does not name was given out by id")
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

// A revert applies the version held before and goes back one step only: a second revert is refused,
// since it would return to the version just reverted from, until a later apply.
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
	assert.Equal(t, [2]int{1, 0}, [2]int{held.AppliedVersion, held.PreviousVersion})

	_, svcErr = f.svc.Revert(context.Background(), "gw-1", RevertRequest{})
	assert.Equal(t, ErrorNothingToRevert.Code, svcErr.Code, "a revert was reverted")
	_, svcErr = f.svc.Revert(context.Background(), "gw-1", RevertRequest{DryRun: true})
	assert.Equal(t, ErrorNothingToRevert.Code, svcErr.Code, "a second revert was offered")

	// A later apply gives the gateway a version to revert to again.
	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})
	held, _ = f.svc.GetApplied(context.Background(), "gw-1")
	assert.Equal(t, [2]int{2, 1}, [2]int{held.AppliedVersion, held.PreviousVersion})
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

// A version read on its own says where it refers to each value; a listing does not.
func TestAVersionReadSaysWhereItRefersToValues(t *testing.T) {
	f := newVersionFixture(t)
	f.captureReferring(t)

	v, svcErr := f.svc.GetVersion(context.Background(), "latest")

	require.Nil(t, svcErr)
	assert.Equal(t, []ValueReference{
		{Name: "APP_CLIENT_ID", Kind: ReferenceVariable, ResourceType: "application", ResourceID: "app-1",
			ResourceName: "app", Field: "client_id"},
		{Name: "APP_SECRET", Kind: ReferenceSecret, ResourceType: "application", ResourceID: "app-1",
			ResourceName: "app", Field: "client_secret"},
	}, v.References)
	listed, svcErr := f.svc.ListVersions(context.Background())
	require.Nil(t, svcErr)
	assert.Empty(t, listed[0].References)
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

// A gateway whose host resolves to a private address this deployment does not reach is reported as
// such, naming the setting that allows it, rather than as any unreachable gateway.
func TestAGatewayOnAPrivateAddressIsReportedWithTheSetting(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.client.err = fmt.Errorf("failed to reach the gateway: %w",
		fmt.Errorf("host %q resolves to a private address: %w", "dp.internal", syshttp.ErrPrivateAddress))

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorGatewayPrivateAddress.Code, svcErr.Code)
	assert.Contains(t, svcErr.ErrorDescription.DefaultValue, "gateway.allow_private_addresses")
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
	svc := newVersionService(declared, f.versions, f.exporter, f.client, testSecretFields, &fakeActorNames{})
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
	assert.Equal(t, &MissingValues{
		Secrets: []string{"APP_SECRET"},
		References: []ValueReference{{Name: "APP_SECRET", Kind: ReferenceSecret, ResourceType: "application",
			ResourceID: "app-1", ResourceName: "app", Field: "client_secret"}},
	}, result.Missing, "a missing value does not say what it is for")
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

// The configuration as it stands says where it refers to each value before anything is captured, and
// reading it keeps no version.
func TestTheCurrentConfigurationSaysWhereItRefersToValuesWithoutCapturing(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = &export.ExportResponse{Files: []export.ExportFile{{Content: "resource_type: application\n" +
		"id: app-1\nname: Orders\nclientId: var:APPLICATION_ORDERS_CLIENT_ID\nredirectUris:\n" +
		"  - var:APPLICATION_ORDERS_REDIRECT_URIS"}}}

	v, svcErr := f.svc.GetVersion(context.Background(), " current ")

	require.Nil(t, svcErr)
	assert.Zero(t, v.Seq)
	assert.Equal(t, versionHash(v.Resources), v.Hash, "the hash is not the one a capture would give it")
	assert.Equal(t, []ValueReference{
		{Name: "APPLICATION_ORDERS_CLIENT_ID", Kind: ReferenceVariable, ResourceType: "application",
			ResourceID: "app-1", ResourceName: "Orders", Field: "clientId"},
		{Name: "APPLICATION_ORDERS_REDIRECT_URIS", Kind: ReferenceVariable, ResourceType: "application",
			ResourceID: "app-1", ResourceName: "Orders", Field: "redirectUris", List: true},
	}, v.References)
	listed, svcErr := f.svc.ListVersions(context.Background())
	require.Nil(t, svcErr)
	assert.Empty(t, listed, "reading the current configuration captured a version")
}

// A failed export fails the read as it fails a capture.
func TestTheCurrentConfigurationReportsAFailedExport(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.err = &tidcommon.InternalServerError

	_, svcErr := f.svc.GetVersion(context.Background(), "current")

	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

// An apply settles the current version before anything is sent; when the gateway received nothing,
// the version is current again, so the next capture after a change replaces it rather than adding one.
func TestAnApplyThatWroteNothingLeavesTheVersionCurrent(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = exportOf("", "ou-a")
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})
	require.Nil(t, svcErr)
	f.client.err = errImportNotWritten

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.NotNil(t, svcErr)

	f.exporter.response = exportOf("", "ou-a", "ou-b")
	replaced, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})
	require.Nil(t, svcErr)
	assert.Equal(t, 1, replaced.Seq, "a version was added beside one no gateway received")
	assert.Len(t, f.versions.versions, 1)
}

// A version a gateway did receive stays settled, whatever a later apply of it does.
func TestAVersionAGatewayRunsStaysSettled(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = exportOf("", "ou-a")
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.client.err = errImportNotWritten

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorGatewayUnreachable.Code, svcErr.Code, "the apply did not reach the import")

	f.exporter.response = exportOf("", "ou-a", "ou-b")
	added, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})
	require.Nil(t, svcErr)
	assert.Equal(t, 2, added.Seq)
}

// An apply refused for a value the gateway lacks sent nothing either, so the version is current again.
func TestAnApplyRefusedForMissingValuesLeavesTheVersionCurrent(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = &export.ExportResponse{Files: []export.ExportFile{
		{Content: "resource_type: application\nid: app-1\nname: app\nclient_id: var:APP_CLIENT_ID\n"},
	}}
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})
	require.Nil(t, svcErr)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorMissingValues.Code, svcErr.Code)

	f.exporter.response = &export.ExportResponse{Files: []export.ExportFile{
		{Content: "resource_type: application\nid: app-1\nname: renamed\nclient_id: var:APP_CLIENT_ID\n"},
	}}
	replaced, svcErr := f.svc.Capture(context.Background(), CaptureRequest{Current: true})
	require.Nil(t, svcErr)
	assert.Equal(t, 1, replaced.Seq, "a version was added beside one no gateway received")
}

// A configuration that refers to a secret from a field that holds no credential is refused by every
// gateway, so a capture refuses it first, naming where the secret stands, and keeps no version.
func TestACaptureRefusesASecretOutsideACredentialField(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = &export.ExportResponse{Files: []export.ExportFile{
		{Content: application("app-1", "sec:APP_SECRET", "sec:APP_SECRET")},
		{Content: unit("ou-a")},
	}}

	v, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})

	require.NotNil(t, svcErr)
	assert.Nil(t, v)
	assert.Equal(t, ErrorMisplacedSecret.Code, svcErr.Code)
	assert.Contains(t, svcErr.ErrorDescription.DefaultValue, "application/app-1 description")
	assert.NotContains(t, svcErr.ErrorDescription.DefaultValue, "clientSecret")
	assert.Empty(t, f.versions.versions, "a refused configuration was kept as a version")
}

// A version captured before captures refused a misplaced secret still holds it; a diff says where,
// so the resource can be put right before a gateway refuses it.
func TestADiffReportsASecretStandingOutsideACredentialField(t *testing.T) {
	f := newVersionFixture(t)
	content := application("app-1", "sec:APP_SECRET", "sec:APP_SECRET")
	_, err := f.versions.Add(context.Background(), Version{Hash: versionHash(content), Resources: content})
	require.NoError(t, err)

	diff, svcErr := f.svc.Diff(context.Background(), "gw-1", "latest")

	require.Nil(t, svcErr)
	assert.Equal(t, []ValueReference{{Name: "APP_SECRET", Kind: ReferenceSecret, ResourceType: "application",
		ResourceID: "app-1", ResourceName: "app-1", Field: "description"}}, diff.MisplacedSecrets)

	f.capture(t, "ou-a")
	clean, svcErr := f.svc.Diff(context.Background(), "gw-1", "latest")
	require.Nil(t, svcErr)
	assert.Empty(t, clean.MisplacedSecrets)
}

// shortLeaseWait makes an apply waiting for a gateway's lease give up quickly, for a test.
func shortLeaseWait(t *testing.T) {
	t.Helper()
	poll, wait := leasePoll, leaseWait
	leasePoll, leaseWait = time.Millisecond, 30*time.Millisecond
	t.Cleanup(func() { leasePoll, leaseWait = poll, wait })
}

// An apply holds the gateway's lease from before it settles the version until it has recorded what
// the gateway holds, and gives it back after, whatever the outcome.
func TestAnApplyHoldsTheGatewaysLeaseWhileItRuns(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.client.onImport = func() {
		lease, _ := f.versions.GetLease(context.Background(), "gw-1")
		require.NotNil(t, lease)
		assert.NotEmpty(t, lease.Token, "the import ran without the lease")
	}

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	lease, _ := f.versions.GetLease(context.Background(), "gw-1")
	assert.Empty(t, lease.Token, "the lease was not given back")

	f.client.err = errors.New("down")
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.NotNil(t, svcErr)
	lease, _ = f.versions.GetLease(context.Background(), "gw-1")
	assert.Empty(t, lease.Token, "the lease was not given back after a failed apply")

	f.svc.Forget(context.Background(), "gw-1")
	lease, _ = f.versions.GetLease(context.Background(), "gw-1")
	assert.Nil(t, lease, "a removed gateway's lease was kept")
}

// While an apply runs, an apply of the latest version is queued behind it rather than run or refused,
// and once the running one finishes the latest version is applied once more, for whoever asked.
func TestAnApplyOfTheLatestVersionIsQueuedBehindARunningOne(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	var queued *ApplyResult
	var queuedErr *tidcommon.ServiceError
	f.client.onImport = func() {
		if queued == nil && queuedErr == nil {
			// A change captured while the first apply runs, then an apply of what is now latest.
			f.capture(t, "ou-a", "ou-b")
			queued, queuedErr = f.svc.Apply(signedIn("u-2", map[string]interface{}{"username": "bob"}),
				"gw-1", ApplyRequest{})
		}
	}

	// The queued apply runs on its own goroutine; its record is the signal to read the fakes after.
	recorded := make(chan struct{}, 2)
	f.versions.onSetApplied = func(applied int) {
		if applied == 2 {
			recorded <- struct{}{}
		}
	}

	first, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr)
	assert.Equal(t, f.hash(1), first.Diff.ToVersion)
	require.Nil(t, queuedErr)
	require.NotNil(t, queued)
	assert.True(t, queued.Queued)
	assert.Equal(t, f.hash(2), queued.QueuedVersion)
	select {
	case <-recorded:
	case <-time.After(2 * time.Second):
		t.Fatal("the latest version was not applied after the running apply")
	}
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	require.NotNil(t, held.AppliedBy)
	assert.Equal(t, "u-2", held.AppliedBy.ID, "the queued apply was not kept for who asked")
	assert.Len(t, f.client.sent, 2, "the queued apply was sent more than once")
}

// An apply of an older version, or a revert, is a choice of its own: it waits for the running apply
// as long as an import may take, and is refused if the gateway is still held then.
func TestAnApplyOfAnOlderVersionWaitsForTheLeaseAndIsRefusedWhenHeld(t *testing.T) {
	shortLeaseWait(t)
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.capture(t, "ou-a", "ou-b")
	f.versions.leases = map[string]*applyLease{"gw-1": {Token: "other", Since: time.Now()}}

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorApplyInProgress.Code, svcErr.Code)
	assert.Empty(t, f.client.sent, "the import was sent while another apply held the gateway")

	// A lease older than any apply can run was never given back, and is taken over.
	f.versions.leases["gw-1"].Since = time.Now().Add(-2 * leaseTimeout)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	require.Nil(t, svcErr)
	assert.Len(t, f.client.sent, 1)
}

// An apply that would remove more resources than the threshold is refused until it is confirmed; a
// dry run shows the removals without being refused.
func TestAnApplyRemovingManyResourcesNeedsConfirmation(t *testing.T) {
	f := newVersionFixture(t)
	units := []string{"ou-a", "ou-b", "ou-c", "ou-d", "ou-e", "ou-f", "ou-g"}
	f.capture(t, units...)
	f.capture(t, "ou-a")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	require.Nil(t, svcErr)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorRemovalsNeedConfirmation.Code, svcErr.Code)
	assert.Len(t, f.client.sent, 1, "the import was sent without confirmation")

	preview, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2), DryRun: true})
	require.Nil(t, svcErr)
	assert.Equal(t, 6, preview.Diff.Summary.Deleted)

	result, svcErr := f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Version: f.hash(2), ConfirmRemovals: true})
	require.Nil(t, svcErr)
	assert.True(t, result.Recorded)
	assert.Len(t, f.client.last().Deletions, 6)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	require.Nil(t, svcErr, "an apply adding resources needs no confirmation")
	_, svcErr = f.svc.Revert(context.Background(), "gw-1", RevertRequest{})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorRemovalsNeedConfirmation.Code, svcErr.Code, "a revert removing many was not refused")
}

// An apply reports the values the version refers to that were set on the gateway after the version
// it holds was applied, by name, so a value changed outside any version is seen as it takes effect.
func TestAnApplyReportsValuesSetSinceTheLastApply(t *testing.T) {
	f := newVersionFixture(t)
	f.captureReferring(t)
	f.client.held = map[string]bool{"APP_CLIENT_ID": true, "APP_SECRET": true}
	set := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	f.client.heldSince = map[string]time.Time{"APP_CLIENT_ID": set, "APP_SECRET": set}
	before, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	assert.Nil(t, before.Changed, "a first apply has nothing to compare with")
	again, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{DryRun: true})
	require.Nil(t, svcErr)
	assert.Nil(t, again.Changed, "a value not set again was reported")

	// Set again at a time earlier than this plane's clock says the apply was: only the store's own
	// times are compared, so it is still seen.
	f.client.heldSince = map[string]time.Time{"APP_CLIENT_ID": set, "APP_SECRET": set.Add(-time.Hour)}
	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{DryRun: true})

	require.Nil(t, svcErr)
	require.NotNil(t, result.Changed)
	assert.Empty(t, result.Changed.Variables)
	assert.Equal(t, []string{"APP_SECRET"}, result.Changed.Secrets)
	require.Len(t, result.Changed.References, 1)
	assert.Equal(t, "app-1", result.Changed.References[0].ResourceID)
}

// An apply that outlived its lease sends nothing more and records nothing: a lease taken over, on a
// node that stalled, fences the import and the record alike.
func TestAnApplyWhoseLeaseWasTakenOverWritesNothing(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.versions.onSettle = func() { f.versions.leases["gw-1"].Token = "another-node" }

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorAppliedChanged.Code, svcErr.Code)
	assert.Empty(t, f.client.sent, "the import was sent after the lease was lost")

	f.versions.onSettle = nil
	f.versions.leases["gw-1"].Token = ""
	f.client.onImport = func() { f.versions.leases["gw-1"].Token = "another-node" }
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorAppliedChanged.Code, svcErr.Code)
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Equal(t, 0, held.AppliedVersion, "an apply that lost its lease during the import was recorded")
}

// Only an apply that asks for "latest" with nothing chosen and nothing confirmed is queued behind a
// running one; a version named by its hash, or a confirmation, is a choice the queued run could not
// keep, so it waits instead. What the queued run would refuse for its removals is refused at once.
func TestOnlyAPlainApplyOfLatestIsQueued(t *testing.T) {
	shortLeaseWait(t)
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	f.versions.leases = map[string]*applyLease{"gw-1": {Token: "other", Since: time.Now()}}

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(1)})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorApplyInProgress.Code, svcErr.Code, "a version named by its hash was queued")
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{ConfirmRemovals: true})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorApplyInProgress.Code, svcErr.Code, "a confirmed apply was queued")
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: "latest", Skip: []string{"x"}})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorApplyInProgress.Code, svcErr.Code, "an apply skipping changes was queued")
	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: "latest"})
	require.Nil(t, svcErr)
	assert.True(t, result.Queued)
	// What was queued is answered by TestAnApplyOfTheLatestVersionIsQueuedBehindARunningOne; here the
	// ask is taken away, so the next release runs nothing in the background while the fakes change.
	f.versions.leases["gw-1"].PendingLatest = false

	f.versions.leases["gw-1"].Token = ""
	units := []string{"ou-a", "ou-b", "ou-c", "ou-d", "ou-e", "ou-f", "ou-g"}
	f.capture(t, units...)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Version: f.hash(2)})
	require.Nil(t, svcErr)
	f.capture(t, "ou-z")
	f.versions.leases["gw-1"].Token = "other"
	f.versions.leases["gw-1"].Since = time.Now()
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorRemovalsNeedConfirmation.Code, svcErr.Code, "an apply bound to be refused was queued")
}

// An apply that took longer to prepare than its lease allows for an import sends nothing and records
// nothing, so its import cannot overlap one from an apply that took the lease over.
func TestAnApplyThatTookTooLongToPrepareSendsNothing(t *testing.T) {
	budget := prepareBudget
	prepareBudget = -time.Second
	t.Cleanup(func() { prepareBudget = budget })
	f := newVersionFixture(t)
	f.capture(t, "ou-a")

	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorApplyTookTooLong.Code, svcErr.Code)
	assert.Empty(t, f.client.sent, "the import was sent after the budget ran out")
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Zero(t, held.AppliedVersion, "an apply that sent nothing was recorded")
	lease, _ := f.versions.GetLease(context.Background(), "gw-1")
	assert.Empty(t, lease.Token, "the lease was not given back")

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{DryRun: true})
	assert.Nil(t, svcErr, "a dry run sends nothing and is not bounded")
}

// An apply of the latest version that would be queued and would remove too many resources is refused
// before it is queued, while the caller still waits, rather than failing later where no one sees it.
func TestAnApplyRemovingManyIsRefusedBeforeItIsQueued(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a", "ou-b", "ou-c", "ou-d", "ou-e", "ou-f", "ou-g")
	_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.capture(t, "ou-a")
	f.versions.leases["gw-1"].Token, f.versions.leases["gw-1"].Since = "a-node-elsewhere", time.Now()

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorRemovalsNeedConfirmation.Code, svcErr.Code)
	assert.False(t, f.versions.leases["gw-1"].PendingLatest, "an apply that would be refused was queued")
}

// Without configuration the removal threshold is the default rather than off: a safety check fails
// closed.
func TestTheRemovalThresholdDefaultsWithoutConfiguration(t *testing.T) {
	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)

	assert.Equal(t, 5, removalConfirmationLimit())
}
