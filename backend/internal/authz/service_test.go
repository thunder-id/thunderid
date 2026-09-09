// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authz/engine"
	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/resource"
	userpkg "github.com/thunder-id/thunderid/internal/user"
	enginemock "github.com/thunder-id/thunderid/tests/mocks/authz/engine"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/usermock"
)

type AuthorizationServiceTestSuite struct {
	suite.Suite
	mockEngine *enginemock.AuthorizationEngineMock
	service    providers.AuthorizationProvider
}

func TestAuthorizationServiceTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationServiceTestSuite))
}

func TestResourceServerRouteKey(t *testing.T) {
	require.Equal(t, "internal-id", resourceServerRouteKey(engine.ResourceServer{ID: " internal-id "}))
	require.Empty(t, resourceServerRouteKey(engine.ResourceServer{}))
}

func (suite *AuthorizationServiceTestSuite) SetupTest() {
	suite.mockEngine = enginemock.NewAuthorizationEngineMock(suite.T())
	suite.service = newAuthorizationService(suite.mockEngine, nil, nil, nil)
}

func (suite *AuthorizationServiceTestSuite) TestEvaluateAccessSuccess() {
	request := providers.AccessEvaluationRequest{
		Subject:        providers.Subject{ID: "user1", GroupIDs: []string{"group1"}},
		ResourceServer: providers.AccessEvaluationResourceServer{ID: "document"},
		Permission:     providers.Permission{Name: "read"},
	}

	suite.mockEngine.On("EvaluateAccessBatch", mock.Anything,
		mock.MatchedBy(func(req engine.AccessEvaluationsRequest) bool {
			return len(req.Evaluations) == 1 &&
				req.Evaluations[0].Subject.ID == "user1" &&
				req.Evaluations[0].Subject.GroupIDs[0] == "group1" &&
				req.Evaluations[0].ResourceServer.ID == "document" &&
				req.Evaluations[0].Permission.Name == "read"
		})).
		Return(&engine.AccessEvaluationsResponse{
			Evaluations: []engine.AccessEvaluationResponse{{Decision: true}},
		}, nil)

	response, err := suite.service.EvaluateAccess(context.Background(), request)

	suite.Nil(err)
	suite.NotNil(response)
	suite.True(response.Decision)
}

func (suite *AuthorizationServiceTestSuite) TestEvaluateAccessBatchSuccess() {
	request := providers.AccessEvaluationsRequest{
		Evaluations: []providers.AccessEvaluationRequest{
			{
				Subject:        providers.Subject{ID: "user1", GroupIDs: []string{"group1"}},
				ResourceServer: providers.AccessEvaluationResourceServer{ID: "document"},
				Permission:     providers.Permission{Name: "read"},
			},
			{
				Subject:        providers.Subject{ID: "user1", GroupIDs: []string{"group1"}},
				ResourceServer: providers.AccessEvaluationResourceServer{ID: "document"},
				Permission:     providers.Permission{Name: "delete"},
			},
		},
	}

	suite.mockEngine.On("EvaluateAccessBatch", mock.Anything, mock.Anything).
		Return(&engine.AccessEvaluationsResponse{
			Evaluations: []engine.AccessEvaluationResponse{
				{Decision: true},
				{Decision: false},
			},
		}, nil)

	response, err := suite.service.EvaluateAccessBatch(context.Background(), request)

	suite.Nil(err)
	suite.NotNil(response)
	suite.Len(response.Evaluations, 2)
	suite.True(response.Evaluations[0].Decision)
	suite.False(response.Evaluations[1].Decision)
}

func (suite *AuthorizationServiceTestSuite) TestEvaluateAccessBatchReturnsContext() {
	request := providers.AccessEvaluationsRequest{
		Evaluations: []providers.AccessEvaluationRequest{
			{
				Subject:        providers.Subject{ID: "user1"},
				ResourceServer: providers.AccessEvaluationResourceServer{ID: "document"},
				Permission:     providers.Permission{Name: "read"},
			},
		},
	}
	decisionContext := map[string]interface{}{
		"reason": "requires_step_up",
	}

	suite.mockEngine.On("EvaluateAccessBatch", mock.Anything, mock.Anything).
		Return(&engine.AccessEvaluationsResponse{
			Evaluations: []engine.AccessEvaluationResponse{
				{Decision: false, Context: decisionContext},
			},
		}, nil)

	response, err := suite.service.EvaluateAccessBatch(context.Background(), request)

	suite.Nil(err)
	suite.NotNil(response)
	suite.Equal(decisionContext, response.Evaluations[0].Context)
}

func (suite *AuthorizationServiceTestSuite) TestEvaluateAccessPassesPropertiesToEngine() {
	subjectProperties := map[string]interface{}{"department": "Sales"}
	resourceProperties := map[string]interface{}{"owner": "user1"}
	actionProperties := map[string]interface{}{"method": "GET"}
	request := providers.AccessEvaluationRequest{
		Subject: providers.Subject{
			Type:       "user",
			ID:         "user1",
			Properties: subjectProperties,
		},
		ResourceServer: providers.AccessEvaluationResourceServer{
			ID:         "document",
			Properties: resourceProperties,
		},
		Permission: providers.Permission{
			Name:       "read",
			Properties: actionProperties,
		},
	}

	suite.mockEngine.On("EvaluateAccessBatch", mock.Anything,
		mock.MatchedBy(func(req engine.AccessEvaluationsRequest) bool {
			if len(req.Evaluations) != 1 {
				return false
			}
			evaluation := req.Evaluations[0]
			return suite.Equal(subjectProperties, evaluation.Subject.Properties) &&
				suite.Equal(resourceProperties, evaluation.ResourceServer.Properties) &&
				suite.Equal(actionProperties, evaluation.Permission.Properties)
		})).
		Return(&engine.AccessEvaluationsResponse{
			Evaluations: []engine.AccessEvaluationResponse{{Decision: true}},
		}, nil)

	response, err := suite.service.EvaluateAccess(context.Background(), request)

	suite.Nil(err)
	suite.NotNil(response)
	suite.True(response.Decision)
}

func (suite *AuthorizationServiceTestSuite) TestEvaluateAccessBatchEmpty() {
	response, err := suite.service.EvaluateAccessBatch(context.Background(), providers.AccessEvaluationsRequest{})

	suite.Nil(err)
	suite.NotNil(response)
	suite.Empty(response.Evaluations)
	suite.mockEngine.AssertNotCalled(suite.T(), "EvaluateAccessBatch")
}

func (suite *AuthorizationServiceTestSuite) TestEvaluateAccessBatchEngineError() {
	request := providers.AccessEvaluationsRequest{
		Evaluations: []providers.AccessEvaluationRequest{
			{
				Subject:        providers.Subject{ID: "user1"},
				ResourceServer: providers.AccessEvaluationResourceServer{ID: "document"},
				Permission:     providers.Permission{Name: "read"},
			},
		},
	}

	suite.mockEngine.On("EvaluateAccessBatch", mock.Anything, mock.Anything).
		Return((*engine.AccessEvaluationsResponse)(nil), errors.New("engine failed"))

	response, err := suite.service.EvaluateAccessBatch(context.Background(), request)

	suite.Nil(response)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *AuthorizationServiceTestSuite) TestEvaluateAccessEmptyEngineResponse() {
	request := providers.AccessEvaluationRequest{
		Subject:        providers.Subject{ID: "user1"},
		ResourceServer: providers.AccessEvaluationResourceServer{ID: "document"},
		Permission:     providers.Permission{Name: "read"},
	}

	suite.mockEngine.On("EvaluateAccessBatch", mock.Anything, mock.Anything).
		Return(&engine.AccessEvaluationsResponse{}, nil)

	response, err := suite.service.EvaluateAccess(context.Background(), request)

	suite.Nil(err)
	suite.NotNil(response)
	suite.False(response.Decision)
}

func TestAuthorizationServiceEnrichesUserPropertiesAndCachesUsers(t *testing.T) {
	userService := usermock.NewUserServiceInterfaceMock(t)
	userService.EXPECT().GetUser(mock.Anything, "user1", false).Return(&providers.User{
		OUID:       "ou-1",
		Attributes: json.RawMessage(`{"email":"alice@example.com"}`),
	}, (*tidcommon.ServiceError)(nil)).Once()

	service := &authorizationService{userService: userService}
	request := providers.AccessEvaluationsRequest{Evaluations: []providers.AccessEvaluationRequest{
		{
			Subject: providers.Subject{
				Type:       providers.EntityCategoryUser.String(),
				ID:         "user1",
				Properties: map[string]interface{}{"source": "request"},
			},
		},
		{Subject: providers.Subject{Type: providers.EntityCategoryUser.String(), ID: "user1"}},
		{Subject: providers.Subject{Type: "agent", ID: "agent1"}},
	}}

	enriched, svcErr := service.enrichRequest(context.Background(), request)
	require.Nil(t, svcErr)
	require.Equal(t, map[string]interface{}{
		"email":  "alice@example.com",
		"ouId":   "ou-1",
		"source": "request",
	}, enriched.Evaluations[0].Subject.Properties)
	require.Equal(t, map[string]interface{}{"email": "alice@example.com", "ouId": "ou-1"},
		enriched.Evaluations[1].Subject.Properties)
	require.Equal(t, request.Evaluations[2].Subject, enriched.Evaluations[2].Subject)
}

func TestAuthorizationServiceEnrichRequestRejectsInvalidUserAttributes(t *testing.T) {
	userService := usermock.NewUserServiceInterfaceMock(t)
	userService.EXPECT().GetUser(mock.Anything, "user1", false).Return(&providers.User{
		Attributes: json.RawMessage(`{"email":`),
	}, (*tidcommon.ServiceError)(nil))

	service := &authorizationService{userService: userService}
	_, svcErr := service.enrichRequest(context.Background(), providers.AccessEvaluationsRequest{
		Evaluations: []providers.AccessEvaluationRequest{{
			Subject: providers.Subject{Type: providers.EntityCategoryUser.String(), ID: "user1"},
		}},
	})

	require.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

func TestAuthorizationServiceEnrichRequestMapsUserLookupError(t *testing.T) {
	userService := usermock.NewUserServiceInterfaceMock(t)
	userService.EXPECT().GetUser(mock.Anything, "user1", false).Return((*providers.User)(nil), &tidcommon.ServiceError{
		Code: "USR-1000",
	})

	service := &authorizationService{userService: userService}
	_, svcErr := service.enrichRequest(context.Background(), providers.AccessEvaluationsRequest{
		Evaluations: []providers.AccessEvaluationRequest{{
			Subject: providers.Subject{Type: providers.EntityCategoryUser.String(), ID: "user1"},
		}},
	})

	require.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

func TestAuthorizationServiceEnrichRequestContinuesForUnknownUser(t *testing.T) {
	userService := usermock.NewUserServiceInterfaceMock(t)
	userService.EXPECT().GetUser(mock.Anything, "unknown", false).
		Return((*providers.User)(nil), &userpkg.ErrorUserNotFound).Once()
	userService.EXPECT().GetUser(mock.Anything, "user1", false).Return(&providers.User{
		OUID:       "ou-1",
		Attributes: json.RawMessage(`{"email":"alice@example.com"}`),
	}, (*tidcommon.ServiceError)(nil)).Once()

	service := &authorizationService{userService: userService}
	request := providers.AccessEvaluationsRequest{Evaluations: []providers.AccessEvaluationRequest{
		{Subject: providers.Subject{Type: providers.EntityCategoryUser.String(), ID: "unknown"}},
		{Subject: providers.Subject{Type: providers.EntityCategoryUser.String(), ID: "user1"}},
	}}

	enriched, svcErr := service.enrichRequest(context.Background(), request)
	require.Nil(t, svcErr)
	require.Len(t, enriched.Evaluations, 2)
	require.Equal(t, request.Evaluations[0].Subject, enriched.Evaluations[0].Subject)
	require.Equal(t, map[string]interface{}{
		"email": "alice@example.com",
		"ouId":  "ou-1",
	}, enriched.Evaluations[1].Subject.Properties)
}

func TestAuthorizationServiceResolveEngineUsesResourceServerConnection(t *testing.T) {
	ctx := context.Background()
	oldGetter := authzenpdp.GetAuthZENPDPRuntimeConfig
	t.Cleanup(func() { authzenpdp.GetAuthZENPDPRuntimeConfig = oldGetter })
	authzenpdp.GetAuthZENPDPRuntimeConfig = func(
		_ context.Context, id string,
	) (*authzenpdp.AuthZENPDPRuntimeConfig, error) {
		require.Equal(t, "pdp-1", id)
		return &authzenpdp.AuthZENPDPRuntimeConfig{
			TimeoutMS:     1000,
			ID:            "pdp-1",
			Endpoint:      "http://localhost:9000/access/v1/evaluation",
			BatchEndpoint: "http://localhost:9000/access/v1/evaluations",
		}, nil
	}
	resourceService := resourcemock.NewResourceServiceInterfaceMock(t)
	resourceService.EXPECT().GetResourceServer(mock.Anything, "rs-1").
		Return(&providers.ResourceServer{
			ID:         "rs-1",
			Identifier: "https://api.example.com",
			AuthorizationEngine: providers.AuthorizationEngineConfig{
				Type: providers.AuthorizationEngineTypeExternalAuthZENPDP,
				Properties: providers.AuthorizationEngineProperties{
					PDPConnectionID: " pdp-1 ",
				},
			},
		}, (*tidcommon.ServiceError)(nil))
	service := &authorizationService{
		resourceService: resourceService,
		httpClient:      http.DefaultClient,
	}

	externalEngine, identifier, ok, err := service.resolveEngine(ctx, engine.ResourceServer{ID: "rs-1"})

	require.NoError(t, err)
	require.Equal(t, "https://api.example.com", identifier)
	require.True(t, ok)
	require.NotNil(t, externalEngine)
}

func TestAuthorizationServiceResolveEngineFallsBackForMissingResourceServer(t *testing.T) {
	resourceService := resourcemock.NewResourceServiceInterfaceMock(t)
	resourceService.EXPECT().GetResourceServer(mock.Anything, "rs-1").
		Return(nil, &resource.ErrorResourceServerNotFound)
	service := &authorizationService{
		resourceService: resourceService,
	}

	externalEngine, identifier, ok, err := service.resolveEngine(context.Background(), engine.ResourceServer{
		ID: "rs-1",
	})

	require.NoError(t, err)
	require.Empty(t, identifier)
	require.False(t, ok)
	require.Nil(t, externalEngine)
}

func TestAuthorizationServiceResolveEngineFallsBackForMissingPDPConnection(t *testing.T) {
	oldGetter := authzenpdp.GetAuthZENPDPRuntimeConfig
	t.Cleanup(func() { authzenpdp.GetAuthZENPDPRuntimeConfig = oldGetter })
	authzenpdp.GetAuthZENPDPRuntimeConfig = func(
		_ context.Context, _ string,
	) (*authzenpdp.AuthZENPDPRuntimeConfig, error) {
		return nil, nil
	}
	resourceService := resourcemock.NewResourceServiceInterfaceMock(t)
	resourceService.EXPECT().GetResourceServer(mock.Anything, "rs-1").Return(&providers.ResourceServer{
		ID: "rs-1",
		AuthorizationEngine: providers.AuthorizationEngineConfig{
			Type: providers.AuthorizationEngineTypeExternalAuthZENPDP,
			Properties: providers.AuthorizationEngineProperties{
				PDPConnectionID: "pdp-1",
			},
		},
	}, (*tidcommon.ServiceError)(nil))
	service := &authorizationService{
		resourceService: resourceService,
	}

	externalEngine, identifier, ok, err := service.resolveEngine(context.Background(), engine.ResourceServer{
		ID: "rs-1",
	})

	require.NoError(t, err)
	require.Empty(t, identifier)
	require.False(t, ok)
	require.Nil(t, externalEngine)
}

func TestAuthorizationServiceEvaluateAccessBatchRoutesExternalPDPByResourceServerID(t *testing.T) {
	ctx := context.Background()
	pdpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/access/v1/evaluations":
			var request struct {
				Evaluations []struct {
					Resource struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"resource"`
				} `json:"evaluations"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Len(t, request.Evaluations, 1)
			require.Equal(t, "https://api.example.com", request.Evaluations[0].Resource.Type)
			require.Equal(t, "booking-1", request.Evaluations[0].Resource.ID)
			_, _ = w.Write([]byte(`{"evaluations":[{"decision":true}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(pdpServer.Close)
	oldGetter := authzenpdp.GetAuthZENPDPRuntimeConfig
	t.Cleanup(func() { authzenpdp.GetAuthZENPDPRuntimeConfig = oldGetter })
	authzenpdp.GetAuthZENPDPRuntimeConfig = func(
		_ context.Context, id string,
	) (*authzenpdp.AuthZENPDPRuntimeConfig, error) {
		require.Equal(t, "pdp-1", id)
		return &authzenpdp.AuthZENPDPRuntimeConfig{
			TimeoutMS:     1000,
			ID:            "pdp-1",
			Endpoint:      pdpServer.URL + "/access/v1/evaluation",
			BatchEndpoint: pdpServer.URL + "/access/v1/evaluations",
		}, nil
	}
	resourceService := resourcemock.NewResourceServiceInterfaceMock(t)
	resourceService.EXPECT().GetResourceServer(mock.Anything, "rs-1").
		Return(&providers.ResourceServer{
			ID:         "rs-1",
			Identifier: "https://api.example.com",
			AuthorizationEngine: providers.AuthorizationEngineConfig{
				Type: providers.AuthorizationEngineTypeExternalAuthZENPDP,
				Properties: providers.AuthorizationEngineProperties{
					PDPConnectionID: "pdp-1",
				},
			},
		}, (*tidcommon.ServiceError)(nil))
	defaultEngine := &authorizationTestEngine{decision: false}
	service := &authorizationService{
		engine:          defaultEngine,
		resourceService: resourceService,
		httpClient:      pdpServer.Client(),
	}

	response, err := service.evaluateWithResolvedEngines(ctx, engine.AccessEvaluationsRequest{
		Evaluations: []engine.AccessEvaluationRequest{{
			Subject:        engine.Subject{ID: "user1"},
			ResourceServer: engine.ResourceServer{ID: "rs-1", ResourceID: "booking-1"},
			Permission:     engine.Permission{Name: "read"},
		}},
	})

	require.NoError(t, err)
	require.Len(t, response.Evaluations, 1)
	require.True(t, response.Evaluations[0].Decision)
}

func TestAuthorizationServiceEvaluateAccessBatchResolvesEnginesAndPreservesOrder(t *testing.T) {
	ctx := context.Background()
	pdpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluations", r.URL.Path)
		_, _ = w.Write([]byte(`{"evaluations":[{"decision":true},{"decision":false}]}`))
	}))
	t.Cleanup(pdpServer.Close)
	oldGetter := authzenpdp.GetAuthZENPDPRuntimeConfig
	t.Cleanup(func() { authzenpdp.GetAuthZENPDPRuntimeConfig = oldGetter })
	authzenpdp.GetAuthZENPDPRuntimeConfig = func(
		_ context.Context, id string,
	) (*authzenpdp.AuthZENPDPRuntimeConfig, error) {
		require.Equal(t, "pdp-1", id)
		return &authzenpdp.AuthZENPDPRuntimeConfig{
			TimeoutMS:     1000,
			ID:            "pdp-1",
			Endpoint:      pdpServer.URL + "/access/v1/evaluation",
			BatchEndpoint: pdpServer.URL + "/access/v1/evaluations",
		}, nil
	}
	resourceService := resourcemock.NewResourceServiceInterfaceMock(t)
	resourceService.EXPECT().GetResourceServer(mock.Anything, "local-rs").
		Return(&providers.ResourceServer{
			ID: "local-rs",
			AuthorizationEngine: providers.AuthorizationEngineConfig{
				Type: "rbac",
			},
		}, (*tidcommon.ServiceError)(nil)).
		Once()
	resourceService.EXPECT().GetResourceServer(mock.Anything, "rs-1").
		Return(&providers.ResourceServer{
			ID:         "rs-1",
			Identifier: "https://api.example.com",
			AuthorizationEngine: providers.AuthorizationEngineConfig{
				Type: providers.AuthorizationEngineTypeExternalAuthZENPDP,
				Properties: providers.AuthorizationEngineProperties{
					PDPConnectionID: "pdp-1",
				},
			},
		}, (*tidcommon.ServiceError)(nil)).
		Once()
	defaultEngine := &authorizationTestEngine{decision: false}
	service := &authorizationService{
		engine:          defaultEngine,
		resourceService: resourceService,
		httpClient:      pdpServer.Client(),
	}

	response, err := service.evaluateWithResolvedEngines(ctx, engine.AccessEvaluationsRequest{
		Evaluations: []engine.AccessEvaluationRequest{
			{
				ResourceServer: engine.ResourceServer{ID: "local-rs"},
				Permission:     engine.Permission{Name: "read"},
			},
			{
				ResourceServer: engine.ResourceServer{ID: "rs-1", ResourceID: "booking-1"},
				Permission:     engine.Permission{Name: "read"},
			},
			{
				ResourceServer: engine.ResourceServer{ID: "local-rs"},
				Permission:     engine.Permission{Name: "write"},
			},
			{
				ResourceServer: engine.ResourceServer{ID: "rs-1", ResourceID: "booking-2"},
				Permission:     engine.Permission{Name: "cancel"},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, []engine.AccessEvaluationResponse{
		{Decision: false},
		{Decision: true},
		{Decision: false},
		{Decision: false},
	}, response.Evaluations)
	require.Equal(t, 1, defaultEngine.called)
}

type authorizationTestEngine struct {
	decision bool
	called   int
}

func (e *authorizationTestEngine) EvaluateAccess(
	_ context.Context,
	_ engine.AccessEvaluationRequest,
) (*engine.AccessEvaluationResponse, error) {
	return &engine.AccessEvaluationResponse{Decision: e.decision}, nil
}

func (e *authorizationTestEngine) EvaluateAccessBatch(
	_ context.Context,
	request engine.AccessEvaluationsRequest,
) (*engine.AccessEvaluationsResponse, error) {
	e.called++
	responses := make([]engine.AccessEvaluationResponse, 0, len(request.Evaluations))
	for range request.Evaluations {
		responses = append(responses, engine.AccessEvaluationResponse{Decision: e.decision})
	}
	return &engine.AccessEvaluationsResponse{Evaluations: responses}, nil
}
