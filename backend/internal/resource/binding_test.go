// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/resource"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

var (
	errNotFound = &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "RES-1003"}
	errServer   = &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "SSE-5000"}
)

func TestResolveBoundResourceServer(t *testing.T) {
	perms := []string{"read"}
	byPerm := &providers.ResourceServer{ID: "rs-perm"}
	byIdent := &providers.ResourceServer{ID: "rs-ident"}
	def := &providers.ResourceServer{ID: "rs-default"}

	tests := []struct {
		name       string
		identifier string
		perms      []string
		setup      func(m *resourcemock.ResourceServiceInterfaceMock)
		wantID     string
		wantErr    *tidcommon.ServiceError
	}{
		{
			name: "identifier resolves without consulting permissions", identifier: "https://a", perms: perms,
			setup: func(m *resourcemock.ResourceServiceInterfaceMock) {
				m.On("GetResourceServerByIdentifier", mock.Anything, "https://a").Return(byIdent, nil)
			},
			wantID: "rs-ident",
		},
		{
			name: "unknown identifier does not fall back", identifier: "https://a", perms: perms,
			setup: func(m *resourcemock.ResourceServiceInterfaceMock) {
				m.On("GetResourceServerByIdentifier", mock.Anything, "https://a").Return(nil, errNotFound)
			},
			wantErr: errNotFound,
		},
		{
			name: "permissions resolve before default", perms: perms,
			setup: func(m *resourcemock.ResourceServiceInterfaceMock) {
				m.On("GetResourceServerByPermissions", mock.Anything, "", perms).Return(byPerm, nil)
			},
			wantID: "rs-perm",
		},
		{
			name: "permissions not found falls back to default", perms: perms,
			setup: func(m *resourcemock.ResourceServiceInterfaceMock) {
				m.On("GetResourceServerByPermissions", mock.Anything, "", perms).Return(nil, errNotFound)
				m.On("GetResourceServerByIdentifier", mock.Anything, "").Return(def, nil)
			},
			wantID: "rs-default",
		},
		{
			name: "ambiguous permissions do not fall back", perms: perms,
			setup: func(m *resourcemock.ResourceServiceInterfaceMock) {
				m.On("GetResourceServerByPermissions", mock.Anything, "", perms).
					Return(nil, &tidcommon.ErrorAmbiguousPermissions)
			},
			wantErr: &tidcommon.ErrorAmbiguousPermissions,
		},
		{
			name: "server error does not fall back", perms: perms,
			setup: func(m *resourcemock.ResourceServiceInterfaceMock) {
				m.On("GetResourceServerByPermissions", mock.Anything, "", perms).Return(nil, errServer)
			},
			wantErr: errServer,
		},
		{
			name: "no permissions resolves default",
			setup: func(m *resourcemock.ResourceServiceInterfaceMock) {
				m.On("GetResourceServerByIdentifier", mock.Anything, "").Return(def, nil)
			},
			wantID: "rs-default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := resourcemock.NewResourceServiceInterfaceMock(t)
			tt.setup(m)

			rs, err := resource.ResolveBoundResourceServer(context.Background(), m, tt.identifier, tt.perms)

			if tt.wantErr != nil {
				assert.Nil(t, rs)
				assert.Equal(t, tt.wantErr, err)
				return
			}
			assert.Nil(t, err)
			assert.Equal(t, tt.wantID, rs.ID)
		})
	}
}
