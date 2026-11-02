// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/variablestore"
)

// newStoreFixture serves a gateway's store over TLS, answering every call with status and body, and
// returns a service administering it as gw-1, its key sealed.
func newStoreFixture(t *testing.T, status int, body string) (StoreServiceInterface, *[]storeCall) {
	t.Helper()
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })
	property, err := cmodels.NewProperty("key", "the-key", true)
	require.NoError(t, err)
	sealedKey, err := cmodels.SerializePropertiesToJSONArray([]cmodels.Property{*property})
	require.NoError(t, err)

	gw, calls := storeGateway(t, status, body)
	gw.Key = sealedKey
	return newStoreService(&fakeStore{gateways: []Gateway{*gw}}, newGatewayClient()), calls
}

// Each call reaches the gateway's store with its key opened, and comes back with what it returned.
func TestTheStoreServiceCallsTheGatewayWithItsKey(t *testing.T) {
	ctx := context.Background()
	svc, calls := newStoreFixture(t, http.StatusOK, `{"name":"A","value":"1","exists":true}`)

	variable, svcErr := svc.GetVariable(ctx, "gw-1", "A")
	require.Nil(t, svcErr)
	assert.Equal(t, &variablestore.Variable{Name: "A", Value: "1"}, variable.Value)
	set, svcErr := svc.SetVariable(ctx, "gw-1", "A", variablestore.VariableUpdateRequest{Value: "2"})
	require.Nil(t, svcErr)
	assert.False(t, set.Created)
	created, svcErr := svc.CreateVariable(ctx, "gw-1", variablestore.VariableRequest{Name: "A", Value: "1"})
	require.Nil(t, svcErr)
	assert.True(t, created.Created)
	removed, svcErr := svc.DeleteVariable(ctx, "gw-1", "A")
	require.Nil(t, svcErr)
	assert.Nil(t, removed.Value)
	_, svcErr = svc.ListVariables(ctx, "gw-1", StoreListQuery{Names: "A"})
	require.Nil(t, svcErr)

	secret, svcErr := svc.GetSecret(ctx, "gw-1", "S")
	require.Nil(t, svcErr)
	assert.True(t, secret.Value.Exists)
	_, svcErr = svc.SetSecret(ctx, "gw-1", "S", variablestore.SecretUpdateRequest{Value: "v"})
	require.Nil(t, svcErr)
	_, svcErr = svc.CreateSecret(ctx, "gw-1", variablestore.SecretRequest{Name: "S", Value: "v"})
	require.Nil(t, svcErr)
	_, svcErr = svc.DeleteSecret(ctx, "gw-1", "S")
	require.Nil(t, svcErr)
	_, svcErr = svc.ListSecrets(ctx, "gw-1", StoreListQuery{})
	require.Nil(t, svcErr)

	require.Len(t, *calls, 10)
	for _, call := range *calls {
		assert.Equal(t, "the-key", call.key, "%s %s", call.method, call.path)
	}
}

// A call the gateway refuses with an error of its own is answered with that refusal.
func TestTheStoreServicePassesTheGatewaysRefusal(t *testing.T) {
	svc, _ := newStoreFixture(t, http.StatusConflict, `{"code":"VAR-1005"}`)

	answer, svcErr := svc.CreateSecret(context.Background(), "gw-1", variablestore.SecretRequest{Name: "S"})

	require.Nil(t, svcErr)
	require.NotNil(t, answer.Refusal)
	assert.Equal(t, http.StatusConflict, answer.Refusal.Status)
	assert.JSONEq(t, `{"code":"VAR-1005"}`, string(answer.Refusal.Body))
}

// An unknown gateway, and one that answers with something other than an error of its own, are
// reported as such.
func TestTheStoreServiceReportsWhatWentWrong(t *testing.T) {
	svc, _ := newStoreFixture(t, http.StatusBadGateway, `<html>bad gateway</html>`)

	_, svcErr := svc.GetVariable(context.Background(), "absent", "A")
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorGatewayNotFound.Code, svcErr.Code)

	_, svcErr = svc.GetVariable(context.Background(), "gw-1", "A")
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorGatewayUnreachable.Code, svcErr.Code)
}
