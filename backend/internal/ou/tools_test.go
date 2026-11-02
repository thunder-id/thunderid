// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"
	"testing"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type OUToolsTestSuite struct {
	suite.Suite
}

func TestOUToolsTestSuite(t *testing.T) {
	suite.Run(t, new(OUToolsTestSuite))
}

func (suite *OUToolsTestSuite) TestListOrganizationUnits_Success() {
	mockService := NewOrganizationUnitServiceInterfaceMock(suite.T())
	tools := &ouTools{ouService: mockService}

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expectedOUs := []OrganizationUnitBasic{
		{
			ID:          "ou1",
			Handle:      "engineering",
			Name:        "Engineering",
			Description: "Engineering OU",
			LogoURL:     "https://example.com/logo.png",
			IsReadOnly:  false,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:         "ou2",
			Handle:     "marketing",
			Name:       "Marketing",
			IsReadOnly: true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}

	mockService.On("GetOrganizationUnitList", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&OrganizationUnitListResponse{
			TotalResults:      2,
			OrganizationUnits: expectedOUs,
		}, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.listOrganizationUnits(ctx, req, nil)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), output)
	assert.Equal(suite.T(), 2, output.TotalResults)
	assert.Len(suite.T(), output.OrganizationUnits, 2)

	item := output.OrganizationUnits[0]
	assert.Equal(suite.T(), "ou1", item.ID)
	assert.Equal(suite.T(), "engineering", item.Handle)
	assert.Equal(suite.T(), "Engineering", item.Name)
	assert.Equal(suite.T(), "Engineering OU", item.Description)
	assert.Equal(suite.T(), "https://example.com/logo.png", item.LogoURL)
	assert.False(suite.T(), item.IsReadOnly)
	assert.Equal(suite.T(), "2026-01-01T00:00:00Z", item.CreatedAt)
	assert.Equal(suite.T(), "2026-01-01T00:00:00Z", item.UpdatedAt)

	item2 := output.OrganizationUnits[1]
	assert.Equal(suite.T(), "ou2", item2.ID)
	assert.True(suite.T(), item2.IsReadOnly)

	mockService.AssertExpectations(suite.T())
}

func (suite *OUToolsTestSuite) TestListOrganizationUnits_Error() {
	mockService := NewOrganizationUnitServiceInterfaceMock(suite.T())
	tools := &ouTools{ouService: mockService}

	mockService.On("GetOrganizationUnitList", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "database error"},
		})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.listOrganizationUnits(ctx, req, nil)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to list organization units")

	mockService.AssertExpectations(suite.T())
}
