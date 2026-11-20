// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"encoding/base64"
	"errors"
	"strings"
	"unicode/utf8"
)

// urlSafeIDStringPrefix marks a URL-safe ID that encodes the ID string rather than the bytes of a UUID.
// It is not in the base64url alphabet, so it can't start a UUID encoding.
const urlSafeIDStringPrefix = "~"

// errInvalidURLSafeID is returned when a value is not one EncodeURLSafeID produces.
var errInvalidURLSafeID = errors.New("invalid URL-safe ID")

// EncodeURLSafeID encodes an ID into a short value that is safe in a URL. A lowercase UUID is encoded as
// the unpadded base64url of its 16 bytes. Any other ID is encoded as "~" followed by the unpadded base64url
// of the ID string. It returns an empty string for an ID that can't be encoded: an empty ID, or one that isn't
// valid UTF-8 or contains a NUL character.
func EncodeURLSafeID(id string) string {
	if id == strings.ToLower(id) {
		if raw, isUUID := uuidBytes(id); isUUID {
			return base64.RawURLEncoding.EncodeToString(raw)
		}
	}
	if !isURLSafeIDString(id) {
		return ""
	}
	return urlSafeIDStringPrefix + base64.RawURLEncoding.EncodeToString([]byte(id))
}

// DecodeURLSafeID returns the ID a value from EncodeURLSafeID encodes. It rejects any value the encoder would
// not produce, so each ID has exactly one encoding.
func DecodeURLSafeID(value string) (string, error) {
	var id string
	if encoded, isString := strings.CutPrefix(value, urlSafeIDStringPrefix); isString {
		decoded, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return "", errInvalidURLSafeID
		}
		id = string(decoded)
	} else {
		raw, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			return "", errInvalidURLSafeID
		}
		uuid, isUUID := uuidFromBytes(raw)
		if !isUUID {
			return "", errInvalidURLSafeID
		}
		id = uuid
	}

	if id == "" || EncodeURLSafeID(id) != value {
		return "", errInvalidURLSafeID
	}
	return id, nil
}

// isURLSafeIDString reports whether an ID can be encoded in its string form.
func isURLSafeIDString(id string) bool {
	return id != "" && utf8.ValidString(id) && !strings.ContainsRune(id, 0)
}
