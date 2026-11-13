// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/variablestore"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type captureFixture struct {
	capture  *ValueCapture
	exporter *fakeExporter
	gateways *fakeStore
	// calls are those the default gateway's store received, and others those the other gateway's did.
	calls  []storeCall
	others []storeCall
	// refuse makes the default gateway refuse each call; hangUp makes it drop each connection unanswered.
	refuse bool
	hangUp bool
	// hangUps makes the default gateway drop that many calls unanswered, then answer the rest.
	hangUps int
	// held are the variables a listing of the default gateway's store answers with.
	held []string
	mu   sync.Mutex
}

// newCaptureFixture builds a bound capture over two gateways served over TLS, gw-2 being the
// default, each with its own sealed key.
func newCaptureFixture(t *testing.T) *captureFixture {
	t.Helper()
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })

	seal := func(key string) string {
		property, err := cmodels.NewProperty("key", key, true)
		require.NoError(t, err)
		sealed, err := cmodels.SerializePropertiesToJSONArray([]cmodels.Property{*property})
		require.NoError(t, err)
		return sealed
	}
	f := &captureFixture{exporter: &fakeExporter{}}
	record := func(into *[]storeCall) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			f.mu.Lock()
			*into = append(*into, storeCall{method: r.Method, path: r.URL.EscapedPath(),
				key: r.Header.Get(constants.APIKeyHeaderName), body: string(raw)})
			hangUp, refuse, held := f.hangUp, f.refuse, f.held
			if f.hangUps > 0 {
				f.hangUps--
				hangUp = true
			}
			f.mu.Unlock()
			switch {
			case hangUp:
				conn, _, err := w.(http.Hijacker).Hijack()
				require.NoError(t, err)
				_ = conn.Close()
			case refuse:
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"code":"VAR-1001"}`))
			case r.Method == http.MethodGet:
				listing := variablestore.VariableListResponse{Variables: []variablestore.Variable{}}
				for _, name := range held {
					listing.Variables = append(listing.Variables, variablestore.Variable{Name: name})
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(listing)
			default:
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"name":"set"}`))
			}
		}
	}
	other := gatewayFor(t, record(&f.others))
	other.ID, other.Name, other.Key = "gw-1", "other", seal("other-key")
	dev := gatewayFor(t, record(&f.calls))
	dev.ID, dev.Name, dev.Key, dev.IsDefault = "gw-2", testGatewayName, seal("the-key"), true
	f.gateways = &fakeStore{gateways: []Gateway{*other, *dev}}

	f.capture = NewValueCapture()
	// Written in the background, a value's write is tried once and soon, so a test sees the outcome
	// of one write; a test of the retries sets its own.
	f.capture.retryDelay = 5 * time.Millisecond
	f.capture.attempts = 1
	f.capture.bind(f.exporter, f.gateways, newStoreService(f.gateways, newGatewayClient()), newPipeline())
	return f
}

// settle waits for every value on its way to have arrived or been given up on.
func (f *captureFixture) settle(t *testing.T) {
	t.Helper()
	require.Eventually(t, f.capture.idle, 5*time.Second, 5*time.Millisecond, "a value is still on its way")
}

// calls and others are appended by the gateways' handlers, so they are read under the fixture's lock.
func (f *captureFixture) received() ([]storeCall, []storeCall) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]storeCall(nil), f.calls...), append([]storeCall(nil), f.others...)
}

// paths returns the paths the default gateway's store was called at, in order.
func (f *captureFixture) paths() []string {
	calls, _ := f.received()
	paths := make([]string, 0, len(calls))
	for _, call := range calls {
		paths = append(paths, call.path)
	}
	return paths
}

// Each value goes to its own collection of the default gateway's store, set whether or not the name
// is held, with that gateway's key.
func TestACaptureWritesVariablesAndSecretsToTheDefaultGateway(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.variables = map[string]string{"APPLICATION_MY_APP_CLIENT_ID": "the-id"}
	f.exporter.secrets = map[string]string{"APPLICATION_MY_APP_CLIENT_SECRET": "the-secret"}
	resource := struct{ Name string }{Name: "My App"}

	f.capture.CaptureValues(context.Background(), "application", resource)
	f.settle(t)

	require.Len(t, f.exporter.valuesOf, 1)
	assert.Equal(t, "application", f.exporter.valuesOf[0].resourceType)
	assert.Equal(t, resource, f.exporter.valuesOf[0].resource)

	calls, others := f.received()
	assert.ElementsMatch(t, []storeCall{
		{method: http.MethodPut, path: "/variables/APPLICATION_MY_APP_CLIENT_ID", key: "the-key",
			body: `{"value":"the-id"}`},
		{method: http.MethodPut, path: "/secrets/APPLICATION_MY_APP_CLIENT_SECRET", key: "the-key",
			body: `{"value":"the-secret"}`},
	}, calls)
	assert.Empty(t, others, "a value was written to a gateway that is not the default")
}

// With a configured pipeline, values go to its first gateway, the one auto apply deploys
// the version that refers to them to, and not to the default gateway.
func TestACaptureWritesToThePipelinesFirstGateway(t *testing.T) {
	f := newCaptureFixture(t)
	f.capture.pipeline = pipelineOf(config.GatewayPipelineStage{Gateway: "other"})
	f.exporter.secrets = map[string]string{"APPLICATION_MY_APP_CLIENT_SECRET": "the-secret"}

	f.capture.CaptureValues(context.Background(), "application", struct{}{})
	f.settle(t)

	calls, others := f.received()
	assert.Empty(t, calls, "a value went to the default gateway, which is not the first gateway")
	assert.Equal(t, []storeCall{{method: http.MethodPut, path: "/secrets/APPLICATION_MY_APP_CLIENT_SECRET",
		key: "other-key", body: `{"value":"the-secret"}`}}, others)
}

// A name is escaped into the path, so it addresses one value however it is spelled.
func TestACaptureEscapesTheName(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.variables = map[string]string{"A/B C": "value"}

	f.capture.CaptureValues(context.Background(), "application", struct{}{})
	f.settle(t)

	assert.Equal(t, []string{"/variables/A%2FB%20C"}, f.paths())
}

// Without a default gateway there is nowhere a value belongs, so none is written anywhere.
func TestACaptureWithoutADefaultGatewayWritesNothing(t *testing.T) {
	f := newCaptureFixture(t)
	f.gateways.gateways[1].IsDefault = false
	f.exporter.secrets = map[string]string{"APPLICATION_MY_APP_CLIENT_SECRET": "the-secret"}

	f.capture.CaptureValues(context.Background(), "application", struct{}{})
	f.settle(t)

	assert.Empty(t, f.calls)
}

// A gateway that cannot be reached costs the values it would have held, not the write that captured
// them: every value is still attempted and nothing is returned to the caller.
func TestACaptureToAnUnreachableGatewayCarriesOn(t *testing.T) {
	f := newCaptureFixture(t)
	f.hangUp = true
	f.exporter.variables = map[string]string{"A": "1", "B": "2"}
	f.exporter.secrets = map[string]string{"C": "3"}

	f.capture.CaptureValues(context.Background(), "application", struct{}{})
	f.settle(t)

	assert.ElementsMatch(t, []string{"/variables/A", "/variables/B", "/secrets/C"}, f.paths())
}

// A value the gateway refuses does not stop the rest.
func TestACaptureTheGatewayRefusesCarriesOn(t *testing.T) {
	f := newCaptureFixture(t)
	f.refuse = true
	f.exporter.variables = map[string]string{"A": "1", "B": "2"}

	f.capture.CaptureValues(context.Background(), "application", struct{}{})
	f.settle(t)

	assert.ElementsMatch(t, []string{"/variables/A", "/variables/B"}, f.paths())
}

// A resource whose export refers to no value asks nothing of the gateways.
func TestACaptureWithNoValuesLooksUpNoGateway(t *testing.T) {
	f := newCaptureFixture(t)
	f.gateways.err = errors.New("the database is down")

	f.capture.CaptureValues(context.Background(), "application", struct{}{})
	f.settle(t)

	assert.Empty(t, f.calls)
}

// Values that cannot be worked out are not guessed at.
func TestACaptureWhoseValuesFailWritesNothing(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.variables = map[string]string{"A": "1"}
	f.exporter.valuesErr = errors.New("no exporter")

	f.capture.CaptureValues(context.Background(), "application", struct{}{})
	f.settle(t)

	assert.Empty(t, f.calls)
}

// A gateway store that fails to list leaves the values uncaptured rather than failing the caller.
func TestACaptureWhoseGatewayLookupFailsWritesNothing(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.variables = map[string]string{"A": "1"}
	f.gateways.err = errors.New("the database is down")
	f.gateways.gateways = nil

	f.capture.CaptureValues(context.Background(), "application", struct{}{})
	f.settle(t)

	assert.Empty(t, f.calls)
}

// A capture not yet bound, or none at all, does nothing.
func TestAnUnboundCaptureDoesNothing(t *testing.T) {
	assert.NotPanics(t, func() {
		NewValueCapture().CaptureValues(context.Background(), "application", struct{}{})
		var none *ValueCapture
		none.CaptureValues(context.Background(), "application", struct{}{})
	})
}

// A client that hangs up once its write succeeded does not cut the capture short: the values are
// still worked out and written, on a context of the capture's own.
func TestACaptureOutlivesACanceledRequest(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.variables = map[string]string{"APPLICATION_MY_APP_CLIENT_ID": "the-id"}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	f.capture.CaptureValues(canceled, "application", struct{}{})
	f.settle(t)

	assert.False(t, f.exporter.valuesCtxDone, "the values were worked out on the canceled request context")
	assert.Len(t, f.paths(), 1)
}

// A write the gateway did not answer is tried again, with a growing delay, until it succeeds, and a
// newer value given meanwhile is what arrives.
func TestACaptureIsTriedAgainUntilTheGatewayAnswers(t *testing.T) {
	f := newCaptureFixture(t)
	f.capture.attempts = 4
	f.hangUp = true
	f.exporter.variables = map[string]string{"A": "1"}

	f.capture.CaptureValues(context.Background(), "application", struct{}{})
	require.Eventually(t, func() bool { return len(f.paths()) >= 1 }, time.Second, time.Millisecond)
	f.exporter.variables = map[string]string{"A": "2"}
	f.capture.CaptureValues(context.Background(), "application", struct{}{})
	f.mu.Lock()
	f.hangUp = false
	f.mu.Unlock()
	f.settle(t)

	calls, _ := f.received()
	require.NotEmpty(t, calls)
	assert.Equal(t, `{"value":"2"}`, calls[len(calls)-1].body, "the newer value did not arrive last")
	assert.Equal(t, 2*f.capture.retryDelay, f.capture.pushDelay(2))
	assert.Equal(t, pushRetryDelayMax, f.capture.pushDelay(40))
}

// A write the gateway did not answer as many times as it is tried is given up on, and the caller is
// never told: the apply that needs the value reports it missing.
func TestACaptureGivesUpAfterItsAttempts(t *testing.T) {
	f := newCaptureFixture(t)
	f.capture.attempts = 2
	f.hangUp = true
	f.exporter.variables = map[string]string{"A": "1"}

	f.capture.CaptureValues(context.Background(), "application", struct{}{})
	f.settle(t)

	assert.Equal(t, []string{"/variables/A", "/variables/A"}, f.paths())
}

// listedExporter lists resources of one type, each the ID it is read by, under rules that name the given
// variables.
type listedExporter struct {
	fakeResourceExporter
	ids       []string
	variables []string
	// readAsRuntime records whether every read was made as the system's own work.
	readAsRuntime bool
	// failing are the IDs whose read fails, each that many times before it succeeds; a negative count
	// fails every time.
	failing map[string]int
}

func (e *listedExporter) GetAllResourceIDs(context.Context) ([]string, *tidcommon.ServiceError) {
	return e.ids, nil
}

func (e *listedExporter) GetResourceByID(ctx context.Context, id string) (interface{}, string,
	*tidcommon.ServiceError) {
	e.readAsRuntime = security.IsRuntimeContext(ctx)
	if left, ok := e.failing[id]; ok && left != 0 {
		e.failing[id] = left - 1
		return nil, "", &tidcommon.InternalServerError
	}
	return id, id, nil
}

func (e *listedExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{Variables: e.variables}
}

// Only the variables the first gateway lacks are written, each with the value the resource holds, so
// a value the gateway holds is never replaced.
func TestFillingWritesOnlyTheVariablesTheFirstGatewayLacks(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.variables = map[string]string{"AGENT__A1__ID": "jane@example.com", "AGENT__A1__IS_LOCAL": "false"}
	f.held = []string{"AGENT__A1__ID"}
	agents := &listedExporter{fakeResourceExporter: fakeResourceExporter{resourceType: "agent"},
		ids: []string{"a1"}, variables: []string{"Owner.ID"}}

	f.capture.fillMissingVariables(context.Background(), []declarativeresource.ResourceExporter{agents})
	f.settle(t)

	assert.True(t, agents.readAsRuntime, "a resource was read as a caller rather than the system")
	require.Len(t, f.exporter.valuesOf, 1)
	assert.Equal(t, valuesRequest{resourceType: "agent", resource: "a1"}, f.exporter.valuesOf[0])
	calls, others := f.received()
	require.Len(t, calls, 2)
	assert.Equal(t, http.MethodGet, calls[0].method)
	assert.Equal(t, storeCall{method: http.MethodPut, path: "/variables/AGENT__A1__IS_LOCAL", key: "the-key",
		body: `{"value":"false"}`}, calls[1])
	assert.Empty(t, others)
}

// An exporter whose rules name no variable has nothing to fill, so its resources are not read and the
// gateway is not asked.
func TestFillingPassesOverExportersThatNameNoVariables(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.variables = map[string]string{"A": "1"}

	f.capture.fillMissingVariables(context.Background(), []declarativeresource.ResourceExporter{
		&fakeResourceExporter{resourceType: "group"},
		&listedExporter{fakeResourceExporter: fakeResourceExporter{resourceType: "user"}, ids: []string{"u1"}},
	})

	assert.Empty(t, f.exporter.valuesOf)
	assert.Empty(t, f.paths())
}

// A gateway that does not answer yet is asked again, and filled once it answers.
func TestFillingWaitsForTheGateway(t *testing.T) {
	f := newCaptureFixture(t)
	f.capture.attempts = 3
	f.hangUps = 1
	f.exporter.variables = map[string]string{"A": "1"}
	agents := &listedExporter{fakeResourceExporter: fakeResourceExporter{resourceType: "agent"},
		ids: []string{"a1"}, variables: []string{"Owner.ID"}}

	f.capture.fillMissingVariables(context.Background(), []declarativeresource.ResourceExporter{agents})
	f.settle(t)

	assert.Equal(t, []string{"/variables", "/variables", "/variables/A"}, f.paths())
}

// A gateway that never answers is given up on after as many tries as a captured value gets.
func TestFillingGivesUpOnAGatewayThatNeverAnswers(t *testing.T) {
	f := newCaptureFixture(t)
	f.capture.attempts = 2
	f.hangUp = true
	f.exporter.variables = map[string]string{"A": "1"}
	agents := &listedExporter{fakeResourceExporter: fakeResourceExporter{resourceType: "agent"},
		ids: []string{"a1"}, variables: []string{"Owner.ID"}}

	f.capture.fillMissingVariables(context.Background(), []declarativeresource.ResourceExporter{agents})
	f.settle(t)

	assert.Equal(t, []string{"/variables", "/variables"}, f.paths())
}

// A store that refuses the lookup is not asked again: a refusal does not change with time.
func TestFillingStopsWhenTheGatewayRefuses(t *testing.T) {
	f := newCaptureFixture(t)
	f.capture.attempts = 5
	f.refuse = true
	f.exporter.variables = map[string]string{"A": "1"}
	agents := &listedExporter{fakeResourceExporter: fakeResourceExporter{resourceType: "agent"},
		ids: []string{"a1"}, variables: []string{"Owner.ID"}}

	f.capture.fillMissingVariables(context.Background(), []declarativeresource.ResourceExporter{agents})

	assert.Equal(t, []string{"/variables"}, f.paths())
}

// Without a first gateway there is nothing to fill, so no resource is read.
func TestFillingWithoutAFirstGatewayReadsNothing(t *testing.T) {
	f := newCaptureFixture(t)
	f.gateways.gateways[1].IsDefault = false
	agents := &listedExporter{fakeResourceExporter: fakeResourceExporter{resourceType: "agent"},
		ids: []string{"a1"}, variables: []string{"Owner.ID"}}

	f.capture.fillMissingVariables(context.Background(), []declarativeresource.ResourceExporter{agents})

	assert.False(t, agents.readAsRuntime, "a resource was read")
	assert.Empty(t, f.exporter.valuesOf)
	assert.Empty(t, f.paths())
}

// A value a write has already put on its way is newer than the one the fill read, so the fill keeps it.
func TestFillingKeepsAValueAlreadyOnItsWay(t *testing.T) {
	f := newCaptureFixture(t)
	key := "gw-2/" + collectionVariables + "/A"
	f.capture.pending[key] = &pendingValue{gatewayID: "gw-2", collection: collectionVariables, name: "A",
		value: "new", running: true}

	f.capture.enqueue(context.Background(), "gw-2", collectionVariables, map[string]string{"A": "old"}, false)

	assert.Equal(t, "new", f.capture.pending[key].value)
	assert.False(t, f.capture.pending[key].again)
}

// A value a write captured while a fill ran is newer than the fill's, even once its write has finished.
func TestFillingKeepsAValueCapturedMeanwhile(t *testing.T) {
	f := newCaptureFixture(t)
	f.capture.fills = 1
	f.capture.capturedDuringFill = map[string]bool{}

	f.capture.enqueue(context.Background(), "gw-2", collectionVariables, map[string]string{"A": "new"}, true)
	f.settle(t)
	f.capture.enqueue(context.Background(), "gw-2", collectionVariables, map[string]string{"A": "old"}, false)
	f.settle(t)

	calls, _ := f.received()
	require.Len(t, calls, 1)
	assert.Equal(t, `{"value":"new"}`, calls[0].body)
}

// A resource that could not be read is read again on the next try, and filled then.
func TestFillingRetriesAResourceThatCouldNotBeRead(t *testing.T) {
	f := newCaptureFixture(t)
	f.capture.attempts = 3
	f.exporter.variables = map[string]string{"A": "1"}
	agents := &listedExporter{fakeResourceExporter: fakeResourceExporter{resourceType: "agent"},
		ids: []string{"a1"}, variables: []string{"Owner.ID"}, failing: map[string]int{"a1": 1}}

	f.capture.fillMissingVariables(context.Background(), []declarativeresource.ResourceExporter{agents})
	f.settle(t)

	assert.Equal(t, []string{"/variables", "/variables/A"}, f.paths())
}

// The resources that can be read are filled even when another never can be.
func TestFillingFillsWhatItCanRead(t *testing.T) {
	f := newCaptureFixture(t)
	f.capture.attempts = 2
	f.exporter.variables = map[string]string{"A": "1"}
	agents := &listedExporter{fakeResourceExporter: fakeResourceExporter{resourceType: "agent"},
		ids: []string{"a1", "broken"}, variables: []string{"Owner.ID"}, failing: map[string]int{"broken": -1}}

	f.capture.fillMissingVariables(context.Background(), []declarativeresource.ResourceExporter{agents})
	f.settle(t)

	assert.Contains(t, f.paths(), "/variables/A")
}
