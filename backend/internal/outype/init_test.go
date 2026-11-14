// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outype

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) TestInitialize_ServesOUTypesThroughTheEntityTypeService() {
	entityTypes := entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	entityTypes.On("GetEntityType", mock.Anything, entitytype.TypeCategoryOU, testOUTypeID, false).
		Return(&entitytype.EntityType{ID: testOUTypeID}, nil).Once()
	mux := http.NewServeMux()

	service := Initialize(mux, entityTypes)

	s.NotNil(service)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/organization-unit-types/"+testOUTypeID, nil))
	s.Equal(http.StatusOK, w.Code)
}
