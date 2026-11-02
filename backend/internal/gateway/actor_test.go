// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// configureUserIDClaim puts the user id claim in the runtime configuration, as loading a config file
// would, and restores what was there after the test.
func configureUserIDClaim(t *testing.T, idClaim string) {
	t.Helper()
	config.ResetServerRuntime()
	cfg := &config.Config{}
	cfg.Server.SecurityConfig.TrustedIssuer = engineconfig.TrustedIssuerConfig{UserIDClaim: idClaim}
	_ = config.InitializeServerRuntime("", cfg)
	t.Cleanup(func() {
		config.ResetServerRuntime()
		configureGatewayBound(shippedGatewayBound)
	})
}

// By default the caller is their subject, recorded by id alone, and an actor set for the work stands
// in for the caller.
func TestTheActorIsTheCallerUnlessSetForTheWork(t *testing.T) {
	configureUserIDClaim(t, "")
	assert.Nil(t, actorOf(context.Background()), "no caller named someone")
	assert.Equal(t, &Actor{ID: "u-1"}, actorOf(signedIn("u-1", map[string]interface{}{"username": "alice"})),
		"a claim named the user beyond their id")

	assert.Nil(t, actorOf(signedIn(strings.Repeat("x", 256), nil)), "an id its column cannot hold named someone")
	assert.Equal(t, &Actor{ID: strings.Repeat("x", 255)}, actorOf(signedIn(strings.Repeat("x", 255), nil)))

	set := &Actor{ID: "u-2", Name: "bob"}
	assert.Equal(t, set, actorOf(withActor(signedIn("u-1", nil), set)))
	assert.Equal(t, &Actor{ID: "u-1"}, actorOf(withActor(signedIn("u-1", nil), nil)))
}

// The claim a user's id is read from is configured.
func TestTheUserIDClaimIsConfigured(t *testing.T) {
	configureUserIDClaim(t, "uid")
	claims := map[string]interface{}{"uid": "user-7", "username": "alice"}

	assert.Equal(t, &Actor{ID: "user-7"}, actorOf(signedIn("u-1", claims)))
	assert.Nil(t, actorOf(signedIn("u-1", nil)), "a token without the id claim named someone")
}

// An actor is stored as its id alone, and reads back by id.
func TestAnActorIsStoredAsItsID(t *testing.T) {
	assert.Nil(t, actorColumn(nil))
	assert.Equal(t, "u-1", actorColumn(&Actor{ID: "u-1", Name: "alice"}))
	assert.Nil(t, actorFrom(nil))
	assert.Equal(t, &Actor{ID: "u-1"}, actorFrom([]byte("u-1")))
	assert.Equal(t, &Actor{ID: "u-1", Name: "alice"}, shown(&Actor{ID: "u-1", Name: "alice"}))
	assert.Nil(t, shown(&Actor{ID: "u-1"}), "an actor without a name was given out")
	assert.Nil(t, shown(nil))
}
