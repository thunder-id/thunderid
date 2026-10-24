// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
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
	mu     sync.Mutex
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
			hangUp, refuse := f.hangUp, f.refuse
			f.mu.Unlock()
			switch {
			case hangUp:
				conn, _, err := w.(http.Hijacker).Hijack()
				require.NoError(t, err)
				_ = conn.Close()
			case refuse:
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"code":"VAR-1001"}`))
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
	f.capture.bind(f.exporter, f.gateways, newStoreService(f.gateways, newGatewayClient()))
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

var _ declarativeresource.ValueReleaser = (*ValueCapture)(nil)

// releaseValuesOf makes each resource, named by a string, refer to the given secrets; a resource
// named "unreadable" cannot be worked out.
func releaseValuesOf(secrets map[string]map[string]string) func(interface{}) (map[string]string,
	map[string]string, error) {
	return func(resource interface{}) (map[string]string, map[string]string, error) {
		if resource == "unreadable" {
			return nil, nil, errors.New("unreadable")
		}
		return map[string]string{}, secrets[resource.(string)], nil
	}
}

// A deleted resource's values are each removed from the default gateway's store, and nowhere else.
func TestAReleaseOfADeletedResourceRemovesEveryValue(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.valuesFor = func(interface{}) (map[string]string, map[string]string, error) {
		return map[string]string{"CONNECTION_A_URL": "u"}, map[string]string{"CONNECTION_A_PASSWORD": "p"}, nil
	}

	f.capture.ReleaseValues(context.Background(), "connection", "before", nil)
	f.settle(t)

	calls, others := f.received()
	assert.ElementsMatch(t, []storeCall{
		{method: http.MethodDelete, path: "/variables/CONNECTION_A_URL", key: "the-key"},
		{method: http.MethodDelete, path: "/secrets/CONNECTION_A_PASSWORD", key: "the-key"},
	}, calls)
	assert.Empty(t, others, "a value was removed from a gateway that is not the default")
}

// A value the resource still refers to is left for its capture to replace; only the others go.
func TestAReleaseKeepsWhatTheResourceStillRefersTo(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.valuesFor = releaseValuesOf(map[string]map[string]string{
		"before": {"CONNECTION_A_PASSWORD": "old", "CONNECTION_A_X_KEY": "k"},
		"after":  {"CONNECTION_A_PASSWORD": "new"},
	})

	f.capture.ReleaseValues(context.Background(), "connection", "before", "after")
	f.settle(t)

	assert.Equal(t, []string{"/secrets/CONNECTION_A_X_KEY"}, f.paths())
}

// When what the resource now refers to cannot be worked out, none of it was captured either, so
// everything it referred to is removed rather than left holding what it no longer does.
func TestAReleaseWhoseCurrentValuesFailRemovesEveryValue(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.valuesFor = releaseValuesOf(map[string]map[string]string{
		"before": {"CONNECTION_A_PASSWORD": "old"},
	})

	f.capture.ReleaseValues(context.Background(), "connection", "before", "unreadable")
	f.settle(t)

	assert.Equal(t, []string{"/secrets/CONNECTION_A_PASSWORD"}, f.paths())
}

// Values that cannot be worked out name nothing to remove.
func TestAReleaseWhosePreviousValuesFailRemovesNothing(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.valuesFor = releaseValuesOf(nil)

	f.capture.ReleaseValues(context.Background(), "connection", "unreadable", nil)
	f.settle(t)

	assert.Empty(t, f.paths())
}

// A value given for a name after its removal is what the gateway is left with.
func TestAValueCapturedAfterItsReleaseIsWhatRemains(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.valuesFor = releaseValuesOf(map[string]map[string]string{
		"before": {"CONNECTION_A_PASSWORD": "old"},
		"again":  {"CONNECTION_A_PASSWORD": "new"},
	})

	f.capture.ReleaseValues(context.Background(), "connection", "before", nil)
	f.capture.CaptureValues(context.Background(), "connection", "again")
	f.settle(t)

	calls, _ := f.received()
	require.NotEmpty(t, calls)
	last := calls[len(calls)-1]
	assert.Equal(t, http.MethodPut, last.method, "the removal arrived after the newer value")
	assert.Equal(t, `{"value":"new"}`, last.body)
}

// Without a default gateway, or before it is bound, a release removes nothing.
func TestAReleaseWithoutADefaultGatewayRemovesNothing(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.secrets = map[string]string{"CONNECTION_A_PASSWORD": "p"}
	for i := range f.gateways.gateways {
		f.gateways.gateways[i].IsDefault = false
	}

	f.capture.ReleaseValues(context.Background(), "connection", "before", nil)
	f.settle(t)
	NewValueCapture().ReleaseValues(context.Background(), "connection", "before", nil)

	calls, others := f.received()
	assert.Empty(t, calls)
	assert.Empty(t, others)
}

// A name the resource now refers to by a reference it holds is the value an apply resolves that
// reference from, so it is kept.
func TestAReleaseKeepsWhatTheResourceRefersToByReference(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.valuesFor = releaseValuesOf(map[string]map[string]string{
		"before": {"CONNECTION_A_PASSWORD": "old", "CONNECTION_A_X_KEY": "k"},
	})
	f.exporter.referencedFor = func(resource interface{}) []string {
		if resource == "after" {
			return []string{"CONNECTION_A_PASSWORD"}
		}
		return nil
	}

	f.capture.ReleaseValues(context.Background(), "connection", "before", "after")
	f.settle(t)

	assert.Equal(t, []string{"/secrets/CONNECTION_A_X_KEY"}, f.paths())
}

// A deleted resource that held a reference to its own name releases that name too.
func TestAReleaseOfADeletedResourceRemovesWhatItOwnedByReference(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.valuesFor = releaseValuesOf(map[string]map[string]string{"before": {}})
	f.exporter.ownedFor = func(interface{}) []string { return []string{"CONNECTION_A_PASSWORD"} }

	f.capture.ReleaseValues(context.Background(), "connection", "before", nil)
	f.settle(t)

	assert.Equal(t, []string{"/secrets/CONNECTION_A_PASSWORD"}, f.paths())
}
