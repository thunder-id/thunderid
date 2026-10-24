// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

// Header is a single HTTP header name and value.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HTTPConnection is an HTTP endpoint connection. AuthProperties holds its outbound authentication
// in the outboundauth property form, with secret values encrypted.
type HTTPConnection struct {
	ID             string
	Name           string
	Description    string
	URL            string
	TimeoutMS      int
	Headers        []Header
	AuthProperties []cmodels.Property
}
