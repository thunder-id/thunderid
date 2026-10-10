// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/security"
)

func TestRegisterRoutes_SelfUserMetadata(t *testing.T) {
	mux := http.NewServeMux()
	mockSvc := NewUserServiceInterfaceMock(t)
	expectedSchema := &entitytype.EntityType{
		Handle: "employee",
		Schema: json.RawMessage(`{"email":{"type":"string"}}`),
	}
	mockSvc.On("GetUserMetadata", mock.Anything, testUserID123).Return(expectedSchema, nil)

	handler := newUserHandler(mockSvc)
	registerRoutes(mux, handler)

	// Test GET /users/me/meta
	authCtx := security.NewSecurityContextForTest(testUserID123, "", "", nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/users/me/meta", nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// Test OPTIONS /users/me/meta
	optionsReq := httptest.NewRequest(http.MethodOptions, "/users/me/meta", nil)
	optionsReq.Header.Set("Origin", "http://localhost:3000")
	optionsReq.Header.Set("Access-Control-Request-Method", "GET")
	optionsRr := httptest.NewRecorder()

	mux.ServeHTTP(optionsRr, optionsReq)
	require.Equal(t, http.StatusNoContent, optionsRr.Code)
}
