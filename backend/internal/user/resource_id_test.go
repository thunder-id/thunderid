// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A capture names a user's values from the user it is handed, so the exporter must read the ID from it.
func TestUserExporterReadsTheResourceID(t *testing.T) {
	exporter := newUserExporter(nil, nil, nil)

	assert.Equal(t, "user-1", exporter.GetResourceID(&userDeclarativeResource{ID: "user-1"}))
	assert.Empty(t, exporter.GetResourceID("not a user"))
	assert.Empty(t, exporter.GetResourceID(nil))
}
