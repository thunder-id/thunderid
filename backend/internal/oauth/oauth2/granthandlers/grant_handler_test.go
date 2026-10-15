// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type GrantHandlerTestSuite struct {
	suite.Suite
}

func TestGrantHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(GrantHandlerTestSuite))
}

func (suite *GrantHandlerTestSuite) TestTokenBuildErrorResponse() {
	t := suite.T()
	logger := log.GetLogger()
	ended := tokenBuildErrorResponse(t.Context(), logger,
		errors.Join(errors.New("wrapped"), tokenservice.ErrIssuanceRefused), "Failed to generate token")
	assert.Equal(t, constants.ErrorInvalidGrant, ended.Error)
	assert.NotContains(t, ended.ErrorDescription, "lifespan")

	other := tokenBuildErrorResponse(t.Context(), logger, errors.New("signing failed"), "Failed to generate token")
	assert.Equal(t, constants.ErrorServerError, other.Error)
	assert.Equal(t, "Failed to generate token", other.ErrorDescription)
}
