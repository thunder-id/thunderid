// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// A capture names a resource server's identifier from the server it is handed, so the exporter must read
// the ID from it.
func TestResourceServerExporterReadsTheResourceID(t *testing.T) {
	exporter := newResourceServerExporter(nil)

	assert.Equal(t, "rs-1", exporter.GetResourceID(&providers.ResourceServer{ID: "rs-1"}))
	assert.Empty(t, exporter.GetResourceID("not a resource server"))
	assert.Empty(t, exporter.GetResourceID(nil))
}
