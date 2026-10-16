// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package userlookup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// issuer is a trusted issuer that grants a client credentials token and answers for its users.
type issuer struct {
	server *httptest.Server
	users  map[string]string
	tokens atomic.Int32
	looked atomic.Int32
	// token is the one the issuer accepts; changing it makes the one a lookup holds stale.
	token atomic.Value
}

func newIssuer(t *testing.T, users map[string]string) *issuer {
	t.Helper()
	i := &issuer{users: users}
	i.token.Store("t-1")
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		if r.FormValue("grant_type") != "client_credentials" || r.FormValue("client_id") != "lookup" ||
			r.FormValue("client_secret") != "s3cret" || r.FormValue("scope") != "users:read" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		i.tokens.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": i.token.Load(), "token_type": "Bearer", "expires_in": 3600,
		})
	})
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		i.looked.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+i.token.Load().(string) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		name, ok := i.users[r.URL.Path[len("/users/"):]]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": r.URL.Path[len("/users/"):], "attributes": map[string]interface{}{"username": name, "email": "x"},
		})
	})
	i.server = httptest.NewServer(mux)
	t.Cleanup(i.server.Close)
	return i
}

// lookupFor is a lookup against the issuer, with a clock the test moves.
func lookupFor(i *issuer, now *time.Time) *Lookup {
	n := New()
	n.config = func() engineconfig.UserLookupConfig {
		return engineconfig.UserLookupConfig{
			URL: i.server.URL + "/users/{id}", TokenURL: i.server.URL + "/token",
			ClientID: "lookup", ClientSecret: "s3cret", Scopes: []string{"users:read"}, CacheTTL: 60,
		}
	}
	n.do = i.server.Client().Do
	n.now = func() time.Time { return *now }
	return n
}

// Names are read from the issuer with one token, kept for the configured time, and read again after.
func TestNamesAreLookedUpAtTheIssuerAndKept(t *testing.T) {
	now := time.Now()
	i := newIssuer(t, map[string]string{"u-1": "alice", "u-2": "bob"})
	n := lookupFor(i, &now)

	assert.True(t, n.Enabled())
	assert.Equal(t, map[string]string{"u-1": "alice", "u-2": "bob"},
		n.Names(context.Background(), "u-1", "u-2", "u-1", ""), "an unknown or empty id was named")
	assert.Equal(t, int32(1), i.tokens.Load(), "a token was asked for more than once")
	assert.Equal(t, int32(2), i.looked.Load(), "a user was asked for more than once")

	assert.Equal(t, "alice", n.Name(context.Background(), "u-1"))
	assert.Equal(t, int32(2), i.looked.Load(), "a kept name was looked up again")

	now = now.Add(61 * time.Second)
	assert.Equal(t, "alice", n.Name(context.Background(), "u-1"))
	assert.Equal(t, int32(3), i.looked.Load(), "a name was kept past its time")
}

// A token the issuer no longer accepts is replaced once, and an unknown user or a failing issuer
// leaves the user by id.
func TestAStaleTokenIsReplacedAndAFailedLookupLeavesTheID(t *testing.T) {
	now := time.Now()
	i := newIssuer(t, map[string]string{"u-1": "alice"})
	n := lookupFor(i, &now)

	n.Name(context.Background(), "u-1")
	i.token.Store("t-2")
	now = now.Add(61 * time.Second)
	assert.Equal(t, "alice", n.Name(context.Background(), "u-1"))
	assert.Equal(t, int32(2), i.tokens.Load(), "the refused token was not replaced")

	assert.Empty(t, n.Name(context.Background(), "u-9"))
	looked := i.looked.Load()
	assert.Empty(t, n.Name(context.Background(), "u-9"))
	assert.Equal(t, looked, i.looked.Load(), "an unknown user was asked for again")

	i.server.Close()
	assert.Empty(t, n.Name(context.Background(), "u-2"))
}

// Without a lookup configured, users keep their id alone and the issuer is never called.
func TestNoLookupLeavesUsersByID(t *testing.T) {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", &config.Config{})
	t.Cleanup(config.ResetServerRuntime)
	i := newIssuer(t, map[string]string{"u-1": "alice"})
	n := New()
	n.do = i.server.Client().Do

	assert.False(t, n.Enabled())
	assert.Empty(t, n.Name(context.Background(), "u-1"))
	assert.Empty(t, n.Names(context.Background(), "u-1"))
	assert.Equal(t, int32(0), i.tokens.Load())
}
