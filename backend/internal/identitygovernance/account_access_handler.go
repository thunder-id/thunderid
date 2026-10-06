// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/serverconfig"
)

// accountAccessHandler decodes, validates and merges the accountAccess section over the deployment
// base.
type accountAccessHandler struct {
	base accountAccessValue
}

// Decode parses a raw section value. Empty input yields the zero value (an absent layer). Unknown
// fields are rejected so a misspelled key does not silently drop protection.
func (h accountAccessHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return accountAccessValue{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var cfg accountAccessValue
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate refuses a write while a declarative document exists, then checks the incoming value on
// its own and laid over the base. A nil readOnly means this is the declarative load.
func (h accountAccessHandler) Validate(incoming, readOnly, _ any) error {
	if ro, ok := readOnly.(accountAccessValue); ok && !ro.isEmpty() {
		return errDeclarativeAccountAccessLocked
	}
	cfg, ok := incoming.(accountAccessValue)
	if !ok {
		return fmt.Errorf("%s: unexpected config type %T", serverconfig.ConfigNameAccountAccess, incoming)
	}
	if err := validateStructure(cfg); err != nil {
		return err
	}
	return validateCoherence(mergeAccountAccessValue(h.base, cfg))
}

// Merge lays the declarative document over the base when one exists, otherwise the writable layer.
func (h accountAccessHandler) Merge(readOnly, writable any) any {
	if ro, ok := readOnly.(accountAccessValue); ok && !ro.isEmpty() {
		return mergeAccountAccessValue(h.base, ro)
	}
	wr, _ := writable.(accountAccessValue)
	return mergeAccountAccessValue(h.base, wr)
}
