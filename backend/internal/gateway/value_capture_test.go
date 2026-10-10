// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/constants"
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
			*into = append(*into, storeCall{method: r.Method, path: r.URL.EscapedPath(),
				key: r.Header.Get(constants.APIKeyHeaderName), body: string(raw)})
			switch {
			case f.hangUp:
				conn, _, err := w.(http.Hijacker).Hijack()
				require.NoError(t, err)
				_ = conn.Close()
			case f.refuse:
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
	f.capture.bind(f.exporter, f.gateways, newStoreService(f.gateways, newGatewayClient()))
	return f
}

// paths returns the paths the default gateway's store was called at, in order.
func (f *captureFixture) paths() []string {
	paths := make([]string, 0, len(f.calls))
	for _, call := range f.calls {
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

	require.Len(t, f.exporter.valuesOf, 1)
	assert.Equal(t, "application", f.exporter.valuesOf[0].resourceType)
	assert.Equal(t, resource, f.exporter.valuesOf[0].resource)

	assert.Equal(t, []storeCall{
		{method: http.MethodPut, path: "/variables/APPLICATION_MY_APP_CLIENT_ID", key: "the-key",
			body: `{"value":"the-id"}`},
		{method: http.MethodPut, path: "/secrets/APPLICATION_MY_APP_CLIENT_SECRET", key: "the-key",
			body: `{"value":"the-secret"}`},
	}, f.calls)
	assert.Empty(t, f.others, "a value was written to a gateway that is not the default")
}

// A name is escaped into the path, so it addresses one value however it is spelled.
func TestACaptureEscapesTheName(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.variables = map[string]string{"A/B C": "value"}

	f.capture.CaptureValues(context.Background(), "application", struct{}{})

	assert.Equal(t, []string{"/variables/A%2FB%20C"}, f.paths())
}

// Without a default gateway there is nowhere a value belongs, so none is written anywhere.
func TestACaptureWithoutADefaultGatewayWritesNothing(t *testing.T) {
	f := newCaptureFixture(t)
	f.gateways.gateways[1].IsDefault = false
	f.exporter.secrets = map[string]string{"APPLICATION_MY_APP_CLIENT_SECRET": "the-secret"}

	f.capture.CaptureValues(context.Background(), "application", struct{}{})

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

	assert.Equal(t, []string{"/variables/A", "/variables/B", "/secrets/C"}, f.paths())
}

// A value the gateway refuses does not stop the rest.
func TestACaptureTheGatewayRefusesCarriesOn(t *testing.T) {
	f := newCaptureFixture(t)
	f.refuse = true
	f.exporter.variables = map[string]string{"A": "1", "B": "2"}

	f.capture.CaptureValues(context.Background(), "application", struct{}{})

	assert.Equal(t, []string{"/variables/A", "/variables/B"}, f.paths())
}

// A resource whose export refers to no value asks nothing of the gateways.
func TestACaptureWithNoValuesLooksUpNoGateway(t *testing.T) {
	f := newCaptureFixture(t)
	f.gateways.err = errors.New("the database is down")

	f.capture.CaptureValues(context.Background(), "application", struct{}{})

	assert.Empty(t, f.calls)
}

// Values that cannot be worked out are not guessed at.
func TestACaptureWhoseValuesFailWritesNothing(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.variables = map[string]string{"A": "1"}
	f.exporter.valuesErr = errors.New("no exporter")

	f.capture.CaptureValues(context.Background(), "application", struct{}{})

	assert.Empty(t, f.calls)
}

// A gateway store that fails to list leaves the values uncaptured rather than failing the caller.
func TestACaptureWhoseGatewayLookupFailsWritesNothing(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.variables = map[string]string{"A": "1"}
	f.gateways.err = errors.New("the database is down")
	f.gateways.gateways = nil

	f.capture.CaptureValues(context.Background(), "application", struct{}{})

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

	assert.False(t, f.exporter.valuesCtxDone, "the values were worked out on the canceled request context")
	assert.Len(t, f.calls, 1)
}
