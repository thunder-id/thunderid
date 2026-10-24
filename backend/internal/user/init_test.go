// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/sharing"
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

func TestRegisterRoutes_SharingPolicyRoutes(t *testing.T) {
	policy := sharing.Policy{ID: "policy-1", ResourceType: UserSharingType, ResourceID: "user-1"}
	cases := []struct {
		method, path, body string
		expect             func(m *UserServiceInterfaceMock)
		status             int
	}{
		{
			method: http.MethodPost, path: "/users/user-1/sharing-policies", body: `{"childUnits":{"mode":"all"}}`,
			expect: func(m *UserServiceInterfaceMock) {
				m.On("CreateSharingPolicy", mock.Anything, "user-1", mock.Anything).
					Return(policy, (*tidcommon.ServiceError)(nil)).Once()
			},
			status: http.StatusCreated,
		},
		{
			method: http.MethodGet, path: "/users/user-1/sharing-policies",
			expect: func(m *UserServiceInterfaceMock) {
				m.On("ListSharingPolicies", mock.Anything, "user-1", 30, 0).
					Return(sharing.PolicyList{}, (*tidcommon.ServiceError)(nil)).Once()
			},
			status: http.StatusOK,
		},
		{
			method: http.MethodGet, path: "/users/user-1/sharing-policies/policy-1",
			expect: func(m *UserServiceInterfaceMock) {
				m.On("GetSharingPolicy", mock.Anything, "user-1", "policy-1").
					Return(policy, (*tidcommon.ServiceError)(nil)).Once()
			},
			status: http.StatusOK,
		},
		{
			method: http.MethodPut, path: "/users/user-1/sharing-policies/policy-1",
			body: `{"childUnits":{"mode":"all"},"version":1}`,
			expect: func(m *UserServiceInterfaceMock) {
				m.On("UpdateSharingPolicy", mock.Anything, "user-1", "policy-1", mock.Anything).
					Return(policy, (*tidcommon.ServiceError)(nil)).Once()
			},
			status: http.StatusOK,
		},
		{
			method: http.MethodDelete, path: "/users/user-1/sharing-policies/policy-1",
			expect: func(m *UserServiceInterfaceMock) {
				m.On("DeleteSharingPolicy", mock.Anything, "user-1", "policy-1").
					Return((*tidcommon.ServiceError)(nil)).Once()
			},
			status: http.StatusNoContent,
		},
		{
			method: http.MethodGet, path: "/users/user-1/overlay-rules?ouId=ou-child",
			expect: func(m *UserServiceInterfaceMock) {
				overlay := sharing.ResolvedOverlay{OUID: "ou-child", Visible: true}
				m.On("ResolveSharingOverlay", mock.Anything, "user-1", "ou-child").
					Return(overlay, (*tidcommon.ServiceError)(nil)).Once()
			},
			status: http.StatusOK,
		},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			mux := http.NewServeMux()
			mockSvc := NewUserServiceInterfaceMock(t)
			tc.expect(mockSvc)
			registerRoutes(mux, newUserHandler(mockSvc))

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))

			require.Equal(t, tc.status, rr.Code)
		})
	}
}

// A path below a user must never reach the handler for the user itself: before sharing routes
// existed, PUT and DELETE read only the first segment, so any sub-path updated or deleted the user.
func TestRegisterRoutes_UserSubPathsDoNotReachTheUser(t *testing.T) {
	cases := []struct{ method, path string }{
		{http.MethodPut, "/users/user-1/anything"},
		{http.MethodDelete, "/users/user-1/anything"},
		{http.MethodDelete, "/users/user-1/sharing-policies"},
		{http.MethodPut, "/users/user-1/sharing-policies/"},
		{http.MethodDelete, "/users/user-1/sharing-policies/policy-1/extra"},
		{http.MethodPost, "/users/user-1/sharing-policies/policy-1"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			mux := http.NewServeMux()
			registerRoutes(mux, newUserHandler(NewUserServiceInterfaceMock(t)))

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`)))

			require.Equal(t, http.StatusNotFound, rr.Code)
		})
	}
}

func TestRegisterRoutes_SelfOrganizationUnits(t *testing.T) {
	mux := http.NewServeMux()
	mockSvc := NewUserServiceInterfaceMock(t)
	mockSvc.On("GetAccessibleOrganizationUnits", mock.Anything, "siblings", 30, 0).
		Return(&AccessibleOrganizationUnitListResponse{Current: "ou-p"}, (*tidcommon.ServiceError)(nil)).Once()
	registerRoutes(mux, newUserHandler(mockSvc))

	authCtx := security.NewSecurityContextForTest(testUserID123, "ou-p", "", nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/users/me/ous?scope=siblings", nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}
