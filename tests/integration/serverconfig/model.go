// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	systemResourceServerID = "01900000-0000-7000-8000-000000000020"

	// sampleOrigin / otherOrigin are valid origins used to exercise the writable layer's read/write and
	// replace round-trips. The only registered config consumer is CORS, so stored values must be valid
	// CORS origins.
	sampleOrigin = "https://app.example.com"
	otherOrigin  = "https://other.example.com"

	// declarativeOrigin is declared in the read-only declarative fixture
	// (resources/declarative_resources/server_configs/cors.yaml); it must match that file.
	declarativeOrigin = "https://declarative.example.com"
)

var defaultResourceServerConfigURL = serverConfigURL + "/defaultResourceServer"

var corsConfigURL = serverConfigURL + "/cors"

var serverConfigURL = testServerURL + "/server-config"

var testServerURL = testutils.TestServerURL

// i18nMessage mirrors the i18n message structure returned in API error responses.
type i18nMessage struct {
	Key          string `json:"key"`
	DefaultValue string `json:"defaultValue"`
}

// apiErrorResponse mirrors apierror.ErrorResponse for decoding error responses.
type apiErrorResponse struct {
	Code        string      `json:"code"`
	Message     i18nMessage `json:"message"`
	Description i18nMessage `json:"description"`
}

// corsSectionValue mirrors the object-shaped cors section value ({"allowedOrigins":[...]}). Each origin
// element is kept raw for order-sensitive comparison (a literal is the quoted "https://...").
type corsSectionValue struct {
	AllowedOrigins []json.RawMessage `json:"allowedOrigins"`
}

// serverConfigLayers mirrors the layered GET/PUT response. Each layer is the cors section value object;
// an unset layer still carries an (empty) allowedOrigins list. Merged is always present.
type serverConfigLayers struct {
	ReadOnly corsSectionValue `json:"readOnly"`
	Writable corsSectionValue `json:"writable"`
	Merged   corsSectionValue `json:"merged"`
}

type defaultResourceServerValue struct {
	ResourceServerID string `json:"resourceServerId"`
}

type defaultResourceServerLayers struct {
	ReadOnly defaultResourceServerValue `json:"readOnly"`
	Writable defaultResourceServerValue `json:"writable"`
	Merged   defaultResourceServerValue `json:"merged"`
}
