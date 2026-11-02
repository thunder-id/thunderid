// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package idp

import (
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// BasicIDPDTO represents a basic data transfer object for an identity provider.
type BasicIDPDTO struct {
	ID           string
	Name         string
	Description  string
	Type         providers.IDPType
	OUID         string
	IsReadOnly   bool
	IDJagEnabled *bool
}
