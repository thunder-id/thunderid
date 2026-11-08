// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package export

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// identifyingExporter is an exporter that can read a resource's ID from the resource.
type identifyingExporter struct {
	*mockResourceExporter
	id string
}

func (e identifyingExporter) GetResourceID(_ interface{}) string { return e.id }

func TestResourceIDOfReadsTheIDFromAnExporterThatCan(t *testing.T) {
	exporter := identifyingExporter{mockResourceExporter: &mockResourceExporter{}, id: "res-1"}

	assert.Equal(t, "res-1", resourceIDOf(exporter, struct{}{}))
}

func TestResourceIDOfIsEmptyForAnExporterThatCannot(t *testing.T) {
	assert.Empty(t, resourceIDOf(&mockResourceExporter{}, struct{}{}))
}

// An exporter that names values but cannot read an ID would name them with none, so a capture is refused
// rather than stored under a name that more than one resource could share.
func TestPlaceholderValuesRefuseAnExporterThatCannotReadAnID(t *testing.T) {
	exporter := &mockResourceExporter{
		resourceType: "thing", paramType: "Thing", validateName: "A Thing",
		rules: &declarativeresource.ResourceRules{Variables: []string{"Field"}},
	}
	svc := newExportService([]declarativeresource.ResourceExporter{exporter},
		newParameterizer(templatingRules{}, ValueReferences))

	_, _, err := svc.PlaceholderValues(context.Background(), "thing", struct{ Field string }{Field: "value"})

	require.Error(t, err)
	assert.ErrorIs(t, err, errResourceHasNoID)
}
