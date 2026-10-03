// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

type captureFixture struct {
	capture  *ValueCapture
	exporter *fakeExporter
	gateways *fakeStore
	client   *fakeGatewayClient
}

// newCaptureFixture builds a bound capture over two gateways, gw-2 being the default, each with
// its own sealed key.
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

	f := &captureFixture{
		exporter: &fakeExporter{},
		gateways: &fakeStore{gateways: []Gateway{
			{ID: "gw-1", Name: "other", BaseURL: "https://other.test", Key: seal("other-key")},
			{ID: "gw-2", Name: "dev", BaseURL: "https://dp.test", Key: seal("the-key"),
				IsDefault: true},
		}},
		client: &fakeGatewayClient{answer: &StoreAnswer{Status: http.StatusCreated}},
	}
	f.capture = NewValueCapture()
	f.capture.bind(f.exporter, f.gateways,
		newVersionService(f.gateways, &fakeVersionStore{}, f.exporter, f.client))
	return f
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

	require.Len(t, f.client.forwarded, 2)
	assert.Equal(t, StoreCall{Method: http.MethodPut, Path: "/variables/APPLICATION_MY_APP_CLIENT_ID",
		Body: []byte(`{"value":"the-id"}`)}, f.client.forwarded[0])
	assert.Equal(t, StoreCall{Method: http.MethodPut, Path: "/secrets/APPLICATION_MY_APP_CLIENT_SECRET",
		Body: []byte(`{"value":"the-secret"}`)}, f.client.forwarded[1])
	assert.Equal(t, []string{"the-key", "the-key"}, f.client.keys)
}

// A name is escaped into the path, so it addresses one value however it is spelled.
func TestACaptureEscapesTheName(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.variables = map[string]string{"A/B C": "value"}

	f.capture.CaptureValues(context.Background(), "application", struct{}{})

	require.Len(t, f.client.forwarded, 1)
	assert.Equal(t, "/variables/A%2FB%20C", f.client.forwarded[0].Path)
}

// Without a default gateway there is nowhere a value belongs, so none is written anywhere.
func TestACaptureWithoutADefaultGatewayWritesNothing(t *testing.T) {
	f := newCaptureFixture(t)
	f.gateways.gateways[1].IsDefault = false
	f.exporter.secrets = map[string]string{"APPLICATION_MY_APP_CLIENT_SECRET": "the-secret"}

	f.capture.CaptureValues(context.Background(), "application", struct{}{})

	assert.Empty(t, f.client.forwarded)
}

// A gateway that cannot be reached costs the values it would have held, not the write that captured
// them: every value is still attempted and nothing is returned to the caller.
func TestACaptureToAnUnreachableGatewayCarriesOn(t *testing.T) {
	f := newCaptureFixture(t)
	f.client.err = errors.New("connection refused")
	f.exporter.variables = map[string]string{"A": "1", "B": "2"}
	f.exporter.secrets = map[string]string{"C": "3"}

	f.capture.CaptureValues(context.Background(), "application", struct{}{})

	require.Len(t, f.client.forwarded, 3)
	assert.Equal(t, "/variables/A", f.client.forwarded[0].Path)
	assert.Equal(t, "/variables/B", f.client.forwarded[1].Path)
	assert.Equal(t, "/secrets/C", f.client.forwarded[2].Path)
}

// A value the gateway refuses does not stop the rest.
func TestACaptureTheGatewayRefusesCarriesOn(t *testing.T) {
	f := newCaptureFixture(t)
	f.client.answer = &StoreAnswer{Status: http.StatusBadRequest}
	f.exporter.variables = map[string]string{"A": "1", "B": "2"}

	f.capture.CaptureValues(context.Background(), "application", struct{}{})

	assert.Len(t, f.client.forwarded, 2)
}

// A resource whose export refers to no value asks nothing of the gateways.
func TestACaptureWithNoValuesLooksUpNoGateway(t *testing.T) {
	f := newCaptureFixture(t)
	f.gateways.err = errors.New("the database is down")

	f.capture.CaptureValues(context.Background(), "application", struct{}{})

	assert.Empty(t, f.client.forwarded)
}

// Values that cannot be worked out are not guessed at.
func TestACaptureWhoseValuesFailWritesNothing(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.variables = map[string]string{"A": "1"}
	f.exporter.valuesErr = errors.New("no exporter")

	f.capture.CaptureValues(context.Background(), "application", struct{}{})

	assert.Empty(t, f.client.forwarded)
}

// A gateway store that fails to list leaves the values uncaptured rather than failing the caller.
func TestACaptureWhoseGatewayLookupFailsWritesNothing(t *testing.T) {
	f := newCaptureFixture(t)
	f.exporter.variables = map[string]string{"A": "1"}
	f.gateways.err = errors.New("the database is down")
	f.gateways.gateways = nil

	f.capture.CaptureValues(context.Background(), "application", struct{}{})

	assert.Empty(t, f.client.forwarded)
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
	require.Len(t, f.client.forwarded, 1)
}
