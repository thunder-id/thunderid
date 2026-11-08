// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// A capture names an application's values from the application it is handed, in either shape the exporter
// reads, so the exporter must read the ID from both.
func TestApplicationExporterReadsTheResourceID(t *testing.T) {
	exporter := newApplicationExporter(nil, nil)

	assert.Equal(t, "app-1", exporter.GetResourceID(&providers.Application{ID: "app-1"}))
	assert.Equal(t, "app-2", exporter.GetResourceID(&applicationDeclarativeResource{
		Application: providers.Application{ID: "app-2"},
	}))
	assert.Empty(t, exporter.GetResourceID("not an application"))
	assert.Empty(t, exporter.GetResourceID(nil))
}
