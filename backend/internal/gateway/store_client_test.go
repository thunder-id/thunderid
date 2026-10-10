// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/variablestore"
)

// storeCall is what a stand-in gateway's store was asked.
type storeCall struct {
	method, path, query, key, contentType, body string
}

// storeGateway serves a gateway's store, recording each call and answering with status and body.
func storeGateway(t *testing.T, status int, body string) (*Gateway, *[]storeCall) {
	t.Helper()
	calls := &[]storeCall{}
	gw := gatewayFor(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		*calls = append(*calls, storeCall{method: r.Method, path: r.URL.EscapedPath(), query: r.URL.RawQuery,
			key: r.Header.Get(constants.APIKeyHeaderName), contentType: r.Header.Get("Content-Type"),
			body: string(raw)})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
	return gw, calls
}

// Each variable call reaches its own path of the gateway's store with the key, and reads the answer.
func TestTheStoreClientCallsTheGatewaysVariables(t *testing.T) {
	ctx, client := context.Background(), newGatewayClient()
	gw, calls := storeGateway(t, http.StatusOK, `{"name":"A","value":"1"}`)

	variable, err := client.GetVariable(ctx, gw, "the-key", "A")
	require.NoError(t, err)
	assert.Equal(t, &variablestore.Variable{Name: "A", Value: "1"}, variable)

	_, err = client.CreateVariable(ctx, gw, "the-key", variablestore.VariableRequest{Name: "A", Value: "1"})
	require.NoError(t, err)
	_, created, err := client.SetVariable(ctx, gw, "the-key", "A", variablestore.VariableUpdateRequest{Value: "2"})
	require.NoError(t, err)
	assert.False(t, created, "a value replaced was reported created")
	require.NoError(t, client.DeleteVariable(ctx, gw, "the-key", "A"))

	assert.Equal(t, []storeCall{
		{method: http.MethodGet, path: "/variables/A", key: "the-key"},
		{method: http.MethodPost, path: "/variables", key: "the-key", contentType: "application/json",
			body: `{"name":"A","value":"1"}`},
		{method: http.MethodPut, path: "/variables/A", key: "the-key", contentType: "application/json",
			body: `{"value":"2"}`},
		{method: http.MethodDelete, path: "/variables/A", key: "the-key"},
	}, *calls)
}

// Each secret call reaches its own path of the gateway's store, and a write that created the value
// says so.
func TestTheStoreClientCallsTheGatewaysSecrets(t *testing.T) {
	ctx, client := context.Background(), newGatewayClient()
	gw, calls := storeGateway(t, http.StatusCreated, `{"name":"S","exists":true}`)

	secret, created, err := client.SetSecret(ctx, gw, "k", "S", variablestore.SecretUpdateRequest{Value: "v"})
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, &variablestore.Secret{Name: "S", Exists: true}, secret)

	_, err = client.CreateSecret(ctx, gw, "k", variablestore.SecretRequest{Name: "S", Value: "v"})
	require.NoError(t, err)
	_, err = client.GetSecret(ctx, gw, "k", "S")
	require.NoError(t, err)
	require.NoError(t, client.DeleteSecret(ctx, gw, "k", "S"))

	assert.Equal(t, []string{"PUT /secrets/S", "POST /secrets", "GET /secrets/S", "DELETE /secrets/S"},
		[]string{(*calls)[0].method + " " + (*calls)[0].path, (*calls)[1].method + " " + (*calls)[1].path,
			(*calls)[2].method + " " + (*calls)[2].path, (*calls)[3].method + " " + (*calls)[3].path})
}

// A listing passes only the narrowing it was given, and reads the gateway's page.
func TestTheStoreClientListsWithTheQueryGiven(t *testing.T) {
	ctx, client := context.Background(), newGatewayClient()
	gw, calls := storeGateway(t, http.StatusOK, `{"totalResults":1,"count":1,"secrets":[{"name":"S","exists":true}]}`)

	listing, err := client.ListSecrets(ctx, gw, "k", StoreListQuery{Limit: "5", Filter: `name sw "S"`})
	require.NoError(t, err)
	assert.Equal(t, 1, listing.TotalResults)
	assert.Equal(t, "S", listing.Secrets[0].Name)
	_, err = client.ListVariables(ctx, gw, "k", StoreListQuery{})
	require.NoError(t, err)

	assert.Equal(t, "/secrets", (*calls)[0].path)
	assert.Equal(t, "filter=name+sw+%22S%22&limit=5", (*calls)[0].query)
	assert.Equal(t, "/variables", (*calls)[1].path)
	assert.Empty(t, (*calls)[1].query)
}

// A name is escaped on its way to the gateway, so it cannot reach past the store.
func TestTheStoreClientEscapesTheName(t *testing.T) {
	gw, calls := storeGateway(t, http.StatusNotFound, `{"code":"VAR-1004"}`)

	_, _ = newGatewayClient().GetVariable(context.Background(), gw, "k", "../import")

	assert.Equal(t, "/variables/..%2Fimport", (*calls)[0].path)
}

// A call the gateway refuses fails with the gateway's status and error as it gave them, and one
// whose answer cannot be read, or that cannot reach the gateway, fails otherwise.
func TestTheStoreClientReportsWhatWentWrong(t *testing.T) {
	ctx, client := context.Background(), newGatewayClient()
	refusing, _ := storeGateway(t, http.StatusConflict, `{"code":"VAR-1005"}`)
	_, err := client.CreateVariable(ctx, refusing, "k", variablestore.VariableRequest{Name: "A"})
	var refusal *StoreRefusal
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, http.StatusConflict, refusal.Status)
	assert.JSONEq(t, `{"code":"VAR-1005"}`, string(refusal.Body))
	assert.Contains(t, refusal.Error(), "409")

	garbled, _ := storeGateway(t, http.StatusOK, `not json`)
	_, err = client.GetSecret(ctx, garbled, "k", "S")
	assert.Error(t, err)
	assert.False(t, isRefusal(err))

	broken := *garbled
	broken.CACertificate = "not a certificate"
	assert.Error(t, client.DeleteSecret(ctx, &broken, "k", "S"))
}

// UnsetValues asks for the named values with the key, in batches the gateway answers for, and
// returns those it holds no value for.
func TestUnsetValuesReturnsWhatTheGatewayLacks(t *testing.T) {
	gw, calls := storeGateway(t, http.StatusOK,
		`{"variables":[{"name":"A","value":"1"}],"secrets":[{"name":"S0","exists":true}]}`)
	secrets := make([]string, maxNamesPerLookup+1)
	for i := range secrets {
		secrets[i] = fmt.Sprintf("S%d", i)
	}

	missing, err := newGatewayClient().UnsetValues(context.Background(), gw, "the-key", []string{"A", "B"}, secrets)

	require.NoError(t, err)
	assert.Equal(t, []string{"B"}, missing.Variables)
	assert.Equal(t, secrets[1:], missing.Secrets)
	require.Len(t, *calls, 3, "the secrets were not asked for in batches")
	assert.Equal(t, "/variables", (*calls)[0].path)
	assert.Equal(t, "limit=100&names=A%2CB", (*calls)[0].query)
	assert.Equal(t, "the-key", (*calls)[0].key)
	assert.Equal(t, "/secrets", (*calls)[2].path)

	refusing, _ := storeGateway(t, http.StatusUnauthorized, `{"code":"AUTH-4010"}`)
	_, err = newGatewayClient().UnsetValues(context.Background(), refusing, "k", nil, []string{"S"})
	assert.True(t, isRefusal(err))
	_, err = newGatewayClient().UnsetValues(context.Background(), refusing, "k", []string{"A"}, nil)
	assert.True(t, isRefusal(err))
}

func isRefusal(err error) bool {
	var refusal *StoreRefusal
	return errors.As(err, &refusal)
}
