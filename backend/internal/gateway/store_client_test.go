// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

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

// A removal of many names reaches the collection with them in names=, and an age, when given, as whole
// seconds rounded up in unchangedFor=; it answers with the names the gateway removed.
func TestTheStoreClientRemovesManyNamesInOneCall(t *testing.T) {
	ctx, client := context.Background(), newGatewayClient()
	gw, calls := storeGateway(t, http.StatusOK, `{"deleted":["A"]}`)

	deleted, err := client.DeleteVariables(ctx, gw, "the-key", []string{"A", "B"}, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"A"}, deleted)
	deleted, err = client.DeleteSecrets(ctx, gw, "the-key", []string{"S"}, time.Hour+time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, []string{"A"}, deleted)

	assert.Equal(t, []storeCall{
		{method: http.MethodDelete, path: "/variables", query: "names=A%2CB", key: "the-key"},
		{method: http.MethodDelete, path: "/secrets", query: "names=S&unchangedFor=3601",
			key: "the-key"},
	}, *calls)
}

// A gateway too old to remove many names at once refuses with its status, which the caller reads as no
// way to remove values by age.
func TestTheStoreClientReportsARefusedRemovalOfManyNames(t *testing.T) {
	gw, _ := storeGateway(t, http.StatusMethodNotAllowed, ``)

	_, err := newGatewayClient().DeleteVariables(context.Background(), gw, "k", []string{"A"}, time.Second)

	var refusal *StoreRefusal
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, http.StatusMethodNotAllowed, refusal.Status)
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

// HeldValues asks for the named values with the key, in batches the gateway answers for, and
// returns those it holds with when each was set, so the caller can tell what is missing and what
// changed.
func TestHeldValuesReportsWhatTheGatewayHoldsAndSince(t *testing.T) {
	gw, calls := storeGateway(t, http.StatusOK,
		`{"variables":[{"name":"A","value":"1","updatedAt":"2026-10-14T10:00:00Z"}],`+
			`"secrets":[{"name":"S0","exists":true}]}`)
	secrets := make([]string, maxNamesPerLookup+1)
	for i := range secrets {
		secrets[i] = fmt.Sprintf("S%d", i)
	}

	held, err := newGatewayClient().HeldValues(context.Background(), gw, "the-key", []string{"A", "B"}, secrets)

	require.NoError(t, err)
	missing := held.missing([]string{"A", "B"}, secrets)
	assert.Equal(t, []string{"B"}, missing.Variables)
	assert.Equal(t, secrets[1:], missing.Secrets)
	snapshot := held.snapshot([]string{"A", "B"}, secrets)
	assert.JSONEq(t, `{"variables":{"A":"2026-10-14T10:00:00Z"}}`, snapshot,
		"only values the store gave a time for were recorded")
	assert.True(t, held.changedFrom([]string{"A"}, secrets, snapshot).empty(), "nothing was set again")
	earlier := `{"variables":{"A":"2026-10-14T09:00:00Z"}}`
	assert.Equal(t, []string{"A"}, held.changedFrom([]string{"A", "B"}, secrets, earlier).Variables)
	later := `{"variables":{"A":"2026-10-14T11:00:00Z"}}`
	assert.Equal(t, []string{"A"}, held.changedFrom([]string{"A"}, nil, later).Variables,
		"a time that differs either way is a change, whatever order the clocks put them in")
	assert.True(t, held.changedFrom([]string{"A"}, nil, `{}`).empty(), "a value not recorded is not reported")
	assert.True(t, held.changedFrom([]string{"A"}, nil, "").empty(), "nothing recorded reports nothing")
	assert.True(t, held.changedFrom([]string{"A"}, nil, "not json").empty())
	require.Len(t, *calls, 3, "the secrets were not asked for in batches")
	assert.Equal(t, "/variables", (*calls)[0].path)
	assert.Equal(t, "limit=100&names=A%2CB", (*calls)[0].query)
	assert.Equal(t, "the-key", (*calls)[0].key)
	assert.Equal(t, "/secrets", (*calls)[2].path)

	refusing, _ := storeGateway(t, http.StatusUnauthorized, `{"code":"AUTH-4010"}`)
	_, err = newGatewayClient().HeldValues(context.Background(), refusing, "k", nil, []string{"S"})
	assert.True(t, isRefusal(err))
	_, err = newGatewayClient().HeldValues(context.Background(), refusing, "k", []string{"A"}, nil)
	assert.True(t, isRefusal(err))
}

func isRefusal(err error) bool {
	var refusal *StoreRefusal
	return errors.As(err, &refusal)
}

// A batch holds at most maxNamesPerLookup names, and no more than fit in maxNamesQueryBytes as sent,
// so long names make smaller batches; every name is in exactly one batch, in order.
func TestNameBatchesBoundTheCountAndTheSize(t *testing.T) {
	short := make([]string, 250)
	for i := range short {
		short[i] = fmt.Sprintf("N%03d", i)
	}
	long := make([]string, 100)
	for i := range long {
		long[i] = fmt.Sprintf("%03d", i) + strings.Repeat("L", 252)
	}
	cases := map[string]struct {
		names []string
		sizes []int
	}{
		"none":  {nil, nil},
		"short": {short, []int{100, 100, 50}},
		// 255 characters and a %2C each: 23 fit in 6 KB.
		"long": {long, []int{23, 23, 23, 23, 8}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			batches := nameBatches(tc.names)
			var sizes []int
			var all []string
			for _, batch := range batches {
				sizes = append(sizes, len(batch))
				all = append(all, batch...)
				sent := url.Values{"names": {strings.Join(batch, ",")}}.Encode()
				assert.LessOrEqual(t, len(sent)-len("names="), maxNamesQueryBytes)
			}
			assert.Equal(t, tc.sizes, sizes)
			assert.Equal(t, tc.names, all)
		})
	}
}
