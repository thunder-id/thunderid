// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A capture names a connection's values from the connection it is handed, so the exporter must read the ID
// from it.
func TestConnectionExporterReadsTheResourceID(t *testing.T) {
	exporter := newConnectionExporter(nil, nil, nil, nil)

	assert.Equal(t, "conn-1", exporter.GetResourceID(&connectionExportModel{ID: "conn-1"}))
	assert.Empty(t, exporter.GetResourceID("not a connection"))
	assert.Empty(t, exporter.GetResourceID(nil))
}
