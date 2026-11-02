// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcedependencymock"
)

type HTTPConnectionServiceTestSuite struct {
	suite.Suite
	ctx       context.Context
	mockStore *httpConnectionStoreInterfaceMock
	mockOU    *oumock.OrganizationUnitServiceInterfaceMock
	service   *httpConnectionService
}

func TestHTTPConnectionServiceTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionServiceTestSuite))
}

func (s *HTTPConnectionServiceTestSuite) SetupTest() {
	initTestRuntime(s.T(), false)
	s.ctx = context.Background()
	s.mockStore = newHttpConnectionStoreInterfaceMock(s.T())
	s.mockOU = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	// Every connection names the organization unit it belongs to; unless a test is about the owner, "ou-1" exists.
	s.mockOU.On("ResolveOwner", mock.Anything, "ou-1", "", mock.Anything).Return("ou-1", nil).Maybe()
	s.mockOU.EXPECT().ResolveOwnerForUpdate(mock.Anything, "", "", mock.Anything, mock.Anything).
		RunAndReturn(func(
			_ context.Context, _, _, currentID string, _ ou.OwnerErrors,
		) (string, *tidcommon.ServiceError) {
			return currentID, nil
		}).Maybe()
	s.mockOU.On("ResolveOwnerForUpdate", mock.Anything, "ou-1", "", "ou-1", mock.Anything).Return("ou-1", nil).Maybe()
	s.service = newHTTPConnectionService(s.mockStore, s.mockOU)
}

func (s *HTTPConnectionServiceTestSuite) noneAuth() []cmodels.Property {
	return authProperties(s.T(), outboundauth.TypeNone, nil)
}

func (s *HTTPConnectionServiceTestSuite) connection(url string, auth []cmodels.Property) HTTPConnection {
	return HTTPConnection{Name: "Risk", OUID: "ou-1", URL: url, AuthProperties: auth}
}

// expectCreate captures the connection handed to the store.
func (s *HTTPConnectionServiceTestSuite) expectCreate() *HTTPConnection {
	stored := &HTTPConnection{}
	s.mockStore.On("CreateHTTPConnection", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { *stored = args.Get(1).(HTTPConnection) }).Return(nil).Once()
	return stored
}

func (s *HTTPConnectionServiceTestSuite) assertCreateRejected(connection HTTPConnection, want tidcommon.ServiceError) {
	s.T().Helper()
	created, svcErr := s.service.CreateHTTPConnection(s.ctx, connection)
	s.Nil(created)
	s.Require().NotNil(svcErr)
	s.Equal(want.Code, svcErr.Code)
}

func (s *HTTPConnectionServiceTestSuite) TestCreateNormalizesAndStores() {
	stored := s.expectCreate()
	auth := basicAuthProperties(s.T())

	created, svcErr := s.service.CreateHTTPConnection(s.ctx, HTTPConnection{
		OUID: "ou-1", Name: " Risk ", Description: "Risk engine", URL: " HTTPS://Risk.Example.COM:443/evaluate?x=1 ",
		Headers:        []Header{{Name: " X-Tenant ", Value: "acme"}},
		AuthProperties: auth,
	})

	s.Require().Nil(svcErr)
	s.NotEmpty(created.ID)
	s.Equal(HTTPConnection{
		ID: created.ID, Name: "Risk", Description: "Risk engine", OUID: "ou-1",
		URL:       "https://risk.example.com/evaluate?x=1",
		TimeoutMS: defaultTimeoutMS, Headers: []Header{{Name: "X-Tenant", Value: "acme"}}, AuthProperties: auth,
	}, *created)
	s.Equal(*created, *stored)
}

func (s *HTTPConnectionServiceTestSuite) TestCreateWithoutAuthenticationStoresNone() {
	stored := s.expectCreate()

	_, svcErr := s.service.CreateHTTPConnection(s.ctx, HTTPConnection{
		OUID: "ou-1", Name: "Risk", URL: "http://localhost:9000/x"})

	s.Require().Nil(svcErr)
	config, err := outboundauth.FromProperties(stored.AuthProperties)
	s.Require().NoError(err)
	s.Equal(outboundauth.TypeNone, config.Type)
}

func (s *HTTPConnectionServiceTestSuite) TestURLIsCanonicalized() {
	cases := map[string]string{
		"https://risk.example.com:443/a":         "https://risk.example.com/a",
		"http://risk.example.com:80/a":           "http://risk.example.com/a",
		"https://risk.example.com:8443/a":        "https://risk.example.com:8443/a",
		"https://RISK.example.com":               "https://risk.example.com",
		"https://[::1]:443/a":                    "https://[::1]/a",
		"https://[2001:DB8::1]:8443/a":           "https://[2001:db8::1]:8443/a",
		"https://10.0.0.1/a":                     "https://10.0.0.1/a",
		"https://risk.example.com:00443/a":       "https://risk.example.com/a",
		"https://risk.example.com:08443/a":       "https://risk.example.com:8443/a",
		"https://risk.example.com/a%2Fb?q=1#top": "https://risk.example.com/a%2Fb?q=1#top",
	}
	for raw, want := range cases {
		s.Run(raw, func() {
			s.SetupTest()
			stored := s.expectCreate()
			_, svcErr := s.service.CreateHTTPConnection(s.ctx, s.connection(raw, s.noneAuth()))
			s.Require().Nil(svcErr)
			s.Equal(want, stored.URL)
		})
	}
}

func (s *HTTPConnectionServiceTestSuite) TestInvalidURLIsRejected() {
	for _, raw := range []string{
		"", "/evaluate", "ftp://risk.example.com", "https:///evaluate", "risk.example.com",
		"https://user:pass@risk.example.com/x", "https://user@risk.example.com/x", "https://@risk.example.com/x",
		"https://risk.example.com:0/x", "https://risk.example.com:65536/x", "https://risk.example.com:99999/x",
		"https://risk..example.com/x", "https://-risk.example.com/x", "https://risk-.example.com/x",
		"https://risk example.com/x", "https://" + strings.Repeat("a", 64) + ".example.com/x",
		"https://rïsk.example.com/x", "https://[fe80::1%25en0]/x",
	} {
		s.Run(raw, func() {
			s.assertCreateRejected(s.connection(raw, s.noneAuth()), ErrorInvalidURL)
		})
	}
}

func (s *HTTPConnectionServiceTestSuite) TestAuthenticationRequiresHTTPS() {
	s.assertCreateRejected(s.connection("http://risk.example.com/x",
		basicAuthProperties(s.T())), ErrorInvalidURL)

	s.expectCreate()
	_, svcErr := s.service.CreateHTTPConnection(s.ctx, s.connection("http://risk.example.com/x", s.noneAuth()))
	s.Nil(svcErr)
}

func (s *HTTPConnectionServiceTestSuite) TestInvalidAuthenticationIsRejected() {
	cases := map[string][]cmodels.Property{
		"basic without password": authProperties(s.T(), outboundauth.TypeBasic,
			map[string]string{outboundauth.FieldBasicUsername: "svc-user"}),
		"basic without username": authProperties(s.T(), outboundauth.TypeBasic,
			map[string]string{outboundauth.FieldBasicPassword: testPassword}),
		"basic username with a colon": authProperties(s.T(), outboundauth.TypeBasic,
			map[string]string{
				outboundauth.FieldBasicUsername: "svc:user",
				outboundauth.FieldBasicPassword: testPassword,
			}),
	}
	for name, auth := range cases {
		s.Run(name, func() {
			s.assertCreateRejected(s.connection(testURL, auth), ErrorInvalidAuthentication)
		})
	}
}

func (s *HTTPConnectionServiceTestSuite) TestBearerAndAPIKeyAreAccepted() {
	cases := map[string][]cmodels.Property{
		"bearer": authProperties(s.T(), outboundauth.TypeBearer,
			map[string]string{outboundauth.FieldBearerToken: testPassword}),
		"api_key": authProperties(s.T(), outboundauth.TypeAPIKey,
			map[string]string{"X-Tenant-Key": testPassword, "X-Region-Key": "eu"}),
	}
	for name, auth := range cases {
		s.Run(name, func() {
			s.SetupTest()
			stored := s.expectCreate()
			_, svcErr := s.service.CreateHTTPConnection(s.ctx, s.connection(testURL, auth))
			s.Require().Nil(svcErr)
			s.Equal(auth, stored.AuthProperties)
		})
	}
}

func (s *HTTPConnectionServiceTestSuite) TestEveryCredentialMethodRequiresHTTPS() {
	for name, auth := range map[string][]cmodels.Property{
		"basic": basicAuthProperties(s.T()),
		"bearer": authProperties(s.T(), outboundauth.TypeBearer,
			map[string]string{outboundauth.FieldBearerToken: testPassword}),
		"api_key": authProperties(s.T(), outboundauth.TypeAPIKey, map[string]string{"X-Tenant-Key": testPassword}),
	} {
		s.Run(name, func() {
			s.assertCreateRejected(s.connection("http://risk.example.com/x", auth), ErrorInvalidURL)
		})
	}
}

// A secret that reaches the service as the mask was never resolved against a stored value, so
// storing it would replace the real secret with the placeholder.
func (s *HTTPConnectionServiceTestSuite) TestMaskedSecretsAreRejected() {
	cases := map[string][]cmodels.Property{
		"basic password": authProperties(s.T(), outboundauth.TypeBasic, map[string]string{
			outboundauth.FieldBasicUsername: "svc-user", outboundauth.FieldBasicPassword: testMaskedSecret}),
		"bearer token": authProperties(s.T(), outboundauth.TypeBearer,
			map[string]string{outboundauth.FieldBearerToken: testMaskedSecret}),
		"api_key value": authProperties(s.T(), outboundauth.TypeAPIKey,
			map[string]string{"X-Tenant-Key": testMaskedSecret}),
	}
	for name, auth := range cases {
		s.Run(name, func() {
			s.assertCreateRejected(s.connection(testURL, auth), ErrorInvalidAuthentication)
		})
	}
}

func (s *HTTPConnectionServiceTestSuite) TestInvalidAPIKeyHeadersAreRejected() {
	cases := map[string]map[string]string{
		"invalid name":            {"Bad Header": testPassword},
		"value line break":        {"X-Tenant-Key": "a\r\nX-Injected: b"},
		"HTTP-managed name":       {"Host": testPassword},
		"repeated in other case":  {"X-Tenant-Key": testPassword, "x-tenant-key": testPassword},
		"proxy authorization":     {"Proxy-Authorization": testPassword},
		"content type lower case": {"content-type": testPassword},
	}
	for name, headers := range cases {
		s.Run(name, func() {
			s.assertCreateRejected(s.connection(testURL, authProperties(s.T(), outboundauth.TypeAPIKey, headers)),
				ErrorInvalidAuthentication)
		})
	}
}

// A plain header named like an API-key header would be overwritten on the call, or would send a
// second value for the credential, so the two lists may not share a name in any case.
func (s *HTTPConnectionServiceTestSuite) TestPlainHeaderMayNotRepeatAnAPIKeyHeader() {
	auth := authProperties(s.T(), outboundauth.TypeAPIKey, map[string]string{"X-Tenant-Key": testPassword})
	connection := s.connection(testURL, auth)
	connection.Headers = []Header{{Name: "x-TENANT-key", Value: "plain"}}

	s.assertCreateRejected(connection, ErrorInvalidHeaders)
}

func (s *HTTPConnectionServiceTestSuite) TestNameAndDescriptionLimits() {
	s.assertCreateRejected(HTTPConnection{OUID: "ou-1", Name: " ", URL: testURL}, ErrorInvalidName)
	s.assertCreateRejected(HTTPConnection{OUID: "ou-1", Name: strings.Repeat("n", maxNameLength+1), URL: testURL},
		ErrorInvalidName)
	s.assertCreateRejected(HTTPConnection{OUID: "ou-1", Name: "Risk", URL: testURL,
		Description: strings.Repeat("d", maxDescriptionLength+1)}, ErrorInvalidDescription)

	s.expectCreate()
	_, svcErr := s.service.CreateHTTPConnection(s.ctx, HTTPConnection{
		OUID: "ou-1", Name: strings.Repeat("é", maxNameLength), URL: testURL,
		Description: strings.Repeat("é", maxDescriptionLength)})
	s.Nil(svcErr, "limits count characters, not bytes")
}

func (s *HTTPConnectionServiceTestSuite) TestTimeoutBounds() {
	for _, timeout := range []int{-1, maxTimeoutMS + 1} {
		s.assertCreateRejected(HTTPConnection{OUID: "ou-1", Name: "Risk", URL: testURL, TimeoutMS: timeout},
			ErrorInvalidTimeout)
	}
	for _, timeout := range []int{1, maxTimeoutMS} {
		stored := s.expectCreate()
		_, svcErr := s.service.CreateHTTPConnection(s.ctx, HTTPConnection{OUID: "ou-1", Name: "Risk", URL: testURL,
			TimeoutMS: timeout})
		s.Require().Nil(svcErr)
		s.Equal(timeout, stored.TimeoutMS)
	}
}

func (s *HTTPConnectionServiceTestSuite) TestInvalidHeadersAreRejected() {
	cases := map[string][]Header{
		"invalid name":        {{Name: "Bad Header", Value: "v"}},
		"empty name":          {{Name: "", Value: "v"}},
		"repeated name":       {{Name: "X-Tenant", Value: "a"}, {Name: "x-tenant", Value: "b"}},
		"value line break":    {{Name: "X-Tenant", Value: "a\r\nX-Injected: b"}},
		"authorization":       {{Name: "authorization", Value: "Bearer token"}},
		"proxy authorization": {{Name: "Proxy-Authorization", Value: "v"}},
	}
	for _, name := range []string{"Cookie", "Host", "Content-Length", "CONTENT-TYPE", "Transfer-Encoding",
		"Connection", "TE", "Trailer", "Upgrade", "Keep-Alive", "Expect", "x-api-KEY", "Api-Key", "X-AUTH-TOKEN",
		"x-access-token"} {
		cases["reserved "+name] = []Header{{Name: name, Value: "v"}}
	}
	for name, headers := range cases {
		s.Run(name, func() {
			s.assertCreateRejected(HTTPConnection{OUID: "ou-1", Name: "Risk", URL: testURL, Headers: headers},
				ErrorInvalidHeaders)
		})
	}
}

func (s *HTTPConnectionServiceTestSuite) TestNameConflictIsReported() {
	s.mockStore.On("CreateHTTPConnection", mock.Anything, mock.Anything).
		Return(errors.Join(errNameConflict, errors.New("unique index"))).Once()

	s.assertCreateRejected(HTTPConnection{OUID: "ou-1", Name: "Risk", URL: testURL}, ErrorAlreadyExists)
}

func (s *HTTPConnectionServiceTestSuite) TestStoreFailuresAreServerErrors() {
	failure := errors.New("database unavailable")
	s.mockStore.On("CreateHTTPConnection", mock.Anything, mock.Anything).Return(failure)
	s.mockStore.On("ListHTTPConnections", mock.Anything).Return(nil, failure)
	s.mockStore.On("GetHTTPConnection", mock.Anything, "id").Return(nil, failure)
	s.mockStore.On("GetHTTPConnection", mock.Anything, "writable").
		Return(&HTTPConnection{ID: "writable", OUID: "ou-1"}, nil)
	s.mockStore.On("UpdateHTTPConnection", mock.Anything, mock.Anything).Return(failure)

	s.assertCreateRejected(HTTPConnection{OUID: "ou-1", Name: "Risk", URL: testURL}, tidcommon.InternalServerError)
	_, svcErr := s.service.ListHTTPConnections(s.ctx)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	_, svcErr = s.service.GetHTTPConnection(s.ctx, "id")
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	_, svcErr = s.service.UpdateHTTPConnection(s.ctx, "id", HTTPConnection{Name: "Risk", URL: testURL})
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	_, svcErr = s.service.UpdateHTTPConnection(s.ctx, "writable", HTTPConnection{Name: "Risk", URL: testURL})
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *HTTPConnectionServiceTestSuite) TestGetAndList() {
	stored := &HTTPConnection{ID: "id", Name: "Risk", URL: testURL}
	s.mockStore.On("GetHTTPConnection", mock.Anything, "id").Return(stored, nil)
	s.mockStore.On("GetHTTPConnection", mock.Anything, "missing").Return(nil, nil)
	s.mockStore.On("ListHTTPConnections", mock.Anything).Return([]HTTPConnection{*stored}, nil)

	got, svcErr := s.service.GetHTTPConnection(s.ctx, "id")
	s.Require().Nil(svcErr)
	s.Equal(stored, got)

	_, svcErr = s.service.GetHTTPConnection(s.ctx, "missing")
	s.Equal(ErrorNotFound.Code, svcErr.Code)
	_, svcErr = s.service.GetHTTPConnection(s.ctx, " ")
	s.Equal(ErrorNotFound.Code, svcErr.Code)

	listed, svcErr := s.service.ListHTTPConnections(s.ctx)
	s.Require().Nil(svcErr)
	s.Equal([]HTTPConnection{*stored}, listed)
}

func (s *HTTPConnectionServiceTestSuite) TestUpdateValidatesAndStores() {
	s.mockStore.On("GetHTTPConnection", mock.Anything, "id").
		Return(&HTTPConnection{ID: "id", OUID: "ou-1"}, nil).Once()
	s.mockStore.On("UpdateHTTPConnection", mock.Anything, mock.MatchedBy(func(c HTTPConnection) bool {
		return c.ID == "id" && c.URL == "https://risk.example.com/v2"
	})).Return(nil).Once()

	updated, svcErr := s.service.UpdateHTTPConnection(s.ctx, "id", HTTPConnection{ID: "ignored", Name: "Risk",
		URL: "https://risk.example.com:443/v2"})

	s.Require().Nil(svcErr)
	s.Equal("id", updated.ID)

	_, svcErr = s.service.UpdateHTTPConnection(s.ctx, "id", s.connection("http://risk.example.com/x",
		basicAuthProperties(s.T())))
	s.Equal(ErrorInvalidURL.Code, svcErr.Code)
}

func (s *HTTPConnectionServiceTestSuite) TestUpdateReportsMissingAndConflict() {
	s.mockStore.On("GetHTTPConnection", mock.Anything, "missing").Return(nil, nil)
	s.mockStore.On("GetHTTPConnection", mock.Anything, "gone").
		Return(&HTTPConnection{ID: "gone", OUID: "ou-1"}, nil)
	s.mockStore.On("GetHTTPConnection", mock.Anything, "taken").
		Return(&HTTPConnection{ID: "taken", OUID: "ou-1"}, nil)
	s.mockStore.On("UpdateHTTPConnection", mock.Anything, mock.MatchedBy(func(c HTTPConnection) bool {
		return c.ID == "gone"
	})).Return(errConnectionNotFound)
	s.mockStore.On("UpdateHTTPConnection", mock.Anything, mock.MatchedBy(func(c HTTPConnection) bool {
		return c.ID == "taken"
	})).Return(errNameConflict)

	_, svcErr := s.service.UpdateHTTPConnection(s.ctx, "missing", HTTPConnection{Name: "Risk", URL: testURL})
	s.Equal(ErrorNotFound.Code, svcErr.Code)
	_, svcErr = s.service.UpdateHTTPConnection(s.ctx, "gone", HTTPConnection{Name: "Risk", URL: testURL})
	s.Equal(ErrorNotFound.Code, svcErr.Code)
	_, svcErr = s.service.UpdateHTTPConnection(s.ctx, "taken", HTTPConnection{Name: "Risk", URL: testURL})
	s.Equal(ErrorAlreadyExists.Code, svcErr.Code)
}

func (s *HTTPConnectionServiceTestSuite) TestDeleteOfAMissingConnectionSucceeds() {
	for _, id := range []string{"missing", " "} {
		s.Run(id, func() {
			s.SetupTest()
			s.mockStore.On("DeleteHTTPConnection", mock.Anything, id).Return(errConnectionNotFound).Once()
			s.service.SetDependencyRegistry(s.registryReporting(id))

			s.Nil(s.service.DeleteHTTPConnection(s.ctx, id))
		})
	}
}

func (s *HTTPConnectionServiceTestSuite) TestDeleteStoreFailureIsAServerError() {
	s.mockStore.On("DeleteHTTPConnection", mock.Anything, "id").Return(errors.New("database unavailable")).Once()
	s.service.SetDependencyRegistry(s.registryReporting("id"))

	svcErr := s.service.DeleteHTTPConnection(s.ctx, "id")

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError, *svcErr)
}

// registryReporting returns a dependency registry that reports the given usages of connection id.
func (s *HTTPConnectionServiceTestSuite) registryReporting(id string,
	usages ...resourcedependency.ResourceDependency) *resourcedependencymock.RegistryMock {
	total := len(usages)
	registry := resourcedependencymock.NewRegistryMock(s.T())
	registry.EXPECT().GetDependencies(mock.Anything, resourcedependency.ResourceTypeHTTPConnection, id).
		Return(&resourcedependency.DependenciesResponse{
			TotalResults: &total, Count: total, Summary: map[string]int{}, Usages: usages,
		}, nil)
	return registry
}

func flowUsage(id, name string) resourcedependency.ResourceDependency {
	return resourcedependency.ResourceDependency{
		ResourceType: resourcedependency.ResourceTypeFlow, ID: id, DisplayName: name,
		BehaviorOnDelete: resourcedependency.BehaviorRestrict,
	}
}

func (s *HTTPConnectionServiceTestSuite) TestUsagesListTheFlowsThatReferenceTheConnection() {
	s.mockStore.On("GetHTTPConnection", mock.Anything, "id").Return(&HTTPConnection{ID: "id"}, nil)
	s.service.SetDependencyRegistry(s.registryReporting("id", flowUsage("flow-1", "Default sign-in")))

	usages, svcErr := s.service.GetHTTPConnectionUsages(s.ctx, "id")

	s.Require().Nil(svcErr)
	s.Equal([]resourcedependency.ResourceDependency{flowUsage("flow-1", "Default sign-in")}, usages.Usages)
}

func (s *HTTPConnectionServiceTestSuite) TestUsagesOfAMissingConnectionIsNotFound() {
	s.mockStore.On("GetHTTPConnection", mock.Anything, "missing").Return(nil, nil)

	_, svcErr := s.service.GetHTTPConnectionUsages(s.ctx, "missing")

	s.Equal(ErrorNotFound.Code, svcErr.Code)
}

func (s *HTTPConnectionServiceTestSuite) TestUsagesAreUnknownWithoutARegistry() {
	s.mockStore.On("GetHTTPConnection", mock.Anything, "id").Return(&HTTPConnection{ID: "id"}, nil)

	usages, svcErr := s.service.GetHTTPConnectionUsages(s.ctx, "id")

	s.Require().Nil(svcErr)
	s.Nil(usages.TotalResults)
	s.Empty(usages.Usages)
}

func (s *HTTPConnectionServiceTestSuite) TestDeleteIsRefusedWhileAFlowUsesTheConnection() {
	s.service.SetDependencyRegistry(s.registryReporting("id", flowUsage("flow-1", "Default sign-in")))

	svcErr := s.service.DeleteHTTPConnection(s.ctx, "id")

	s.Require().NotNil(svcErr)
	s.Equal(ErrorHasBlockingDependencies.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.String(), "1 flow(s)")
	s.mockStore.AssertNotCalled(s.T(), "DeleteHTTPConnection", mock.Anything, mock.Anything)
}

func (s *HTTPConnectionServiceTestSuite) TestDeleteSucceedsWithoutUsages() {
	s.mockStore.On("DeleteHTTPConnection", mock.Anything, "id").Return(nil).Once()
	s.service.SetDependencyRegistry(s.registryReporting("id"))

	s.Nil(s.service.DeleteHTTPConnection(s.ctx, "id"))
}

func (s *HTTPConnectionServiceTestSuite) TestDeleteFailsClosedWhenUsagesCannotBeDetermined() {
	s.Run("no registry", func() {
		s.service.SetDependencyRegistry(nil)
		s.Equal(tidcommon.InternalServerError.Code, s.service.DeleteHTTPConnection(s.ctx, "id").Code)
	})
	s.Run("registry error", func() {
		registry := resourcedependencymock.NewRegistryMock(s.T())
		registry.EXPECT().GetDependencies(mock.Anything, resourcedependency.ResourceTypeHTTPConnection, "id").
			Return(nil, errors.New("flow store unavailable"))
		s.service.SetDependencyRegistry(registry)
		s.Equal(tidcommon.InternalServerError.Code, s.service.DeleteHTTPConnection(s.ctx, "id").Code)
	})
	s.Run("a provider failed to report", func() {
		registry := resourcedependencymock.NewRegistryMock(s.T())
		registry.EXPECT().GetDependencies(mock.Anything, resourcedependency.ResourceTypeHTTPConnection, "id").
			Return(&resourcedependency.DependenciesResponse{TotalResults: nil}, nil)
		s.service.SetDependencyRegistry(registry)
		s.Equal(tidcommon.InternalServerError.Code, s.service.DeleteHTTPConnection(s.ctx, "id").Code)
	})
	s.mockStore.AssertNotCalled(s.T(), "DeleteHTTPConnection", mock.Anything, mock.Anything)
}

func (s *HTTPConnectionServiceTestSuite) TestResolvedConnectionCarriesHeadersTimeoutAndBasicCredential() {
	stored := &HTTPConnection{
		ID: "id", Name: "Risk", URL: testURL, TimeoutMS: 2500,
		Headers:        []Header{{Name: "X-Tenant", Value: "acme"}},
		AuthProperties: basicAuthProperties(s.T()),
	}
	s.mockStore.On("GetHTTPConnection", mock.Anything, "id").Return(stored, nil)

	resolved, svcErr := s.service.GetResolvedConnection(s.ctx, "id")

	s.Require().Nil(svcErr)
	s.Equal(testURL, resolved.URL)
	s.Equal(2500*time.Millisecond, resolved.Timeout)
	s.Equal("acme", resolved.Headers.Get("X-Tenant"))
	username, password, ok := (&http.Request{Header: resolved.Headers}).BasicAuth()
	s.True(ok)
	s.Equal("svc-user", username)
	s.Equal(testPassword, password)
}

func (s *HTTPConnectionServiceTestSuite) TestResolvedConnectionCarriesBearerAndAPIKeyCredentials() {
	s.mockStore.On("GetHTTPConnection", mock.Anything, "bearer").Return(&HTTPConnection{
		ID: "bearer", URL: testURL, TimeoutMS: defaultTimeoutMS,
		AuthProperties: authProperties(s.T(), outboundauth.TypeBearer,
			map[string]string{outboundauth.FieldBearerToken: testPassword}),
	}, nil)
	s.mockStore.On("GetHTTPConnection", mock.Anything, "api_key").Return(&HTTPConnection{
		ID: "api_key", URL: testURL, TimeoutMS: defaultTimeoutMS,
		Headers:        []Header{{Name: "X-Tenant", Value: "acme"}},
		AuthProperties: authProperties(s.T(), outboundauth.TypeAPIKey, map[string]string{"x-tenant-key": testPassword}),
	}, nil)

	resolved, svcErr := s.service.GetResolvedConnection(s.ctx, "bearer")
	s.Require().Nil(svcErr)
	s.Equal("Bearer "+testPassword, resolved.Headers.Get("Authorization"))

	resolved, svcErr = s.service.GetResolvedConnection(s.ctx, "api_key")
	s.Require().Nil(svcErr)
	s.Equal(http.Header{"X-Tenant": {"acme"}, "X-Tenant-Key": {testPassword}}, resolved.Headers)
}

func (s *HTTPConnectionServiceTestSuite) TestResolveRefusesACredentialOverClearHTTP() {
	s.mockStore.On("GetHTTPConnection", mock.Anything, "id").Return(&HTTPConnection{
		ID: "id", URL: "http://risk.example.com/evaluate", TimeoutMS: defaultTimeoutMS,
		AuthProperties: basicAuthProperties(s.T()),
	}, nil)

	resolved, svcErr := s.service.GetResolvedConnection(s.ctx, "id")

	s.Nil(resolved)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *HTTPConnectionServiceTestSuite) TestResolvedConnectionWithoutAuthenticationSendsNoCredential() {
	s.mockStore.On("GetHTTPConnection", mock.Anything, "id").Return(&HTTPConnection{
		ID: "id", URL: "http://localhost:3000/hook", TimeoutMS: defaultTimeoutMS, AuthProperties: s.noneAuth(),
	}, nil)

	resolved, svcErr := s.service.GetResolvedConnection(s.ctx, "id")

	s.Require().Nil(svcErr)
	s.Equal(http.Header{}, resolved.Headers)
}

func (s *HTTPConnectionServiceTestSuite) TestResolveFailsForAMissingOrUnusableConnection() {
	s.mockStore.On("GetHTTPConnection", mock.Anything, "missing").Return(nil, nil)
	s.mockStore.On("GetHTTPConnection", mock.Anything, "unsupported").Return(&HTTPConnection{
		ID: "unsupported", URL: testURL, AuthProperties: s.unsupportedAuth(),
	}, nil)

	_, svcErr := s.service.GetResolvedConnection(s.ctx, "missing")
	s.Equal(ErrorNotFound.Code, svcErr.Code)
	_, svcErr = s.service.GetResolvedConnection(s.ctx, "unsupported")
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// unsupportedAuth is a stored authentication whose type the HTTP transport cannot present.
func (s *HTTPConnectionServiceTestSuite) unsupportedAuth() []cmodels.Property {
	prop, err := cmodels.NewProperty("authentication_type", "digest", false)
	s.Require().NoError(err)
	return []cmodels.Property{*prop}
}

func (s *HTTPConnectionServiceTestSuite) TestWritesAreRejectedInDeclarativeMode() {
	initTestRuntime(s.T(), true)

	s.assertCreateRejected(HTTPConnection{OUID: "ou-1", Name: "Risk", URL: testURL},
		declarativeresource.ErrorDeclarativeResourceCreateOperation)
	_, svcErr := s.service.UpdateHTTPConnection(s.ctx, "id", HTTPConnection{Name: "Risk", URL: testURL})
	s.Equal(declarativeresource.ErrorDeclarativeResourceUpdateOperation.Code, svcErr.Code)
	s.Equal(declarativeresource.ErrorDeclarativeResourceDeleteOperation.Code,
		s.service.DeleteHTTPConnection(s.ctx, "id").Code)
}

func (s *HTTPConnectionServiceTestSuite) TestCreateResolvesOU() {
	s.mockOU.On("ResolveOwner", mock.Anything, "missing", "", mock.Anything).
		Return("", &ErrorInvalidOU).Once()
	s.mockOU.On("ResolveOwner", mock.Anything, "", "", mock.Anything).
		Return("", &ErrorOURequired).Once()
	stored := s.expectCreate()

	byID := s.connection("https://risk.example.com", s.noneAuth())
	created, svcErr := s.service.CreateHTTPConnection(s.ctx, byID)
	s.Nil(svcErr)
	s.Equal("ou-1", created.OUID)
	s.Equal("ou-1", stored.OUID)

	noOU := s.connection("https://risk.example.com", s.noneAuth())
	noOU.OUID = ""
	s.assertCreateRejected(noOU, ErrorOURequired)

	unknown := s.connection("https://risk.example.com", s.noneAuth())
	unknown.OUID = "missing"
	s.assertCreateRejected(unknown, ErrorInvalidOU)
}

// An import or a declarative file names the owner by handle; the service passes it on to be resolved, on create and
// on update.
func (s *HTTPConnectionServiceTestSuite) TestAnOwnerNamedByHandleIsPassedOn() {
	s.mockOU.EXPECT().ResolveOwner(mock.Anything, "", "acme", mock.Anything).Return("ou-1", nil).Once()
	stored := s.expectCreate()

	created, svcErr := s.service.CreateHTTPConnection(s.ctx,
		HTTPConnection{OUHandle: "acme", Name: "Risk", URL: testURL})
	s.Require().Nil(svcErr)
	s.Equal("ou-1", created.OUID)
	s.Equal("ou-1", stored.OUID)

	current := &HTTPConnection{ID: "http-1", OUID: "ou-1", Name: "Risk", URL: testURL}
	s.mockStore.On("GetHTTPConnectionByName", mock.Anything, "Risk").Return(current, nil)
	s.mockOU.EXPECT().ResolveOwnerForUpdate(mock.Anything, "", "acme", "ou-1", mock.Anything).
		Return("ou-1", nil).Once()
	s.mockOU.EXPECT().ResolveOwnerForUpdate(mock.Anything, "", "other", "ou-1", mock.Anything).
		Return("", &ErrorOUUpdateNotAllowed).Once()
	s.Nil(s.service.ValidateHTTPConnectionUpdate(s.ctx, current,
		&HTTPConnection{OUHandle: "acme", Name: "Risk", URL: testURL}))
	svcErr = s.service.ValidateHTTPConnectionUpdate(s.ctx, current,
		&HTTPConnection{OUHandle: "other", Name: "Risk", URL: testURL})
	s.Require().NotNil(svcErr)
	s.Equal(ErrorOUUpdateNotAllowed.Code, svcErr.Code)
}

func (s *HTTPConnectionServiceTestSuite) TestUpdateKeepsOU() {
	for _, test := range []struct {
		name      string
		requested string
		wantErr   *tidcommon.ServiceError
	}{
		{name: "omitted", requested: ""},
		{name: "same", requested: "ou-1"},
		{name: "changed", requested: "ou-2", wantErr: &ErrorOUUpdateNotAllowed},
	} {
		s.Run(test.name, func() {
			s.SetupTest()
			current := s.connection("https://risk.example.com", s.noneAuth())
			current.ID = "http-1"
			s.mockStore.On("GetHTTPConnection", mock.Anything, "http-1").Return(&current, nil).Once()
			if test.wantErr != nil {
				s.mockOU.On("ResolveOwnerForUpdate", mock.Anything, "ou-2", "", "ou-1", mock.Anything).
					Return("", test.wantErr).Once()
			} else {
				s.mockStore.On("UpdateHTTPConnection", mock.Anything, mock.MatchedBy(func(c HTTPConnection) bool {
					return c.OUID == "ou-1"
				})).Return(nil).Once()
			}
			request := s.connection("https://risk.example.com", s.noneAuth())
			request.OUID = test.requested

			updated, svcErr := s.service.UpdateHTTPConnection(s.ctx, "http-1", request)

			if test.wantErr != nil {
				s.Nil(updated)
				s.Require().NotNil(svcErr)
				s.Equal(test.wantErr.Code, svcErr.Code)
				return
			}
			s.Nil(svcErr)
			s.Equal("ou-1", updated.OUID)
		})
	}
}

func (s *HTTPConnectionServiceTestSuite) TestCreateIsRefusedWhenOnlyTheConnectionStoreIsDeclarative() {
	s.service = newHTTPConnectionService(newFileBasedStore(), s.mockOU)

	s.assertCreateRejected(HTTPConnection{OUID: "ou-1", Name: "Risk", URL: testURL},
		declarativeresource.ErrorDeclarativeResourceCreateOperation)
}

func (s *HTTPConnectionServiceTestSuite) TestCreateConflictsAndRefusalsAreClientErrors() {
	for name, tc := range map[string]struct {
		storeErr error
		want     tidcommon.ServiceError
	}{
		"id taken": {errors.Join(errIDConflict, errors.New("primary key")), ErrorAlreadyExists},
		"declarative target": {errHTTPConnectionIsImmutable,
			declarativeresource.ErrorDeclarativeResourceCreateOperation},
	} {
		s.Run(name, func() {
			s.SetupTest()
			s.mockStore.On("CreateHTTPConnection", mock.Anything, mock.Anything).Return(tc.storeErr).Once()

			s.assertCreateRejected(HTTPConnection{ID: "imported-id", OUID: "ou-1", Name: "Risk", URL: testURL},
				tc.want)
		})
	}
}

func (s *HTTPConnectionServiceTestSuite) TestListOverTheCompositeLimitIsAClientError() {
	s.mockStore.On("ListHTTPConnections", mock.Anything).Return(nil, errResultLimitExceededInCompositeMode)

	_, svcErr := s.service.ListHTTPConnections(s.ctx)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorResultLimitExceededInCompositeMode, *svcErr)
}

// Flows reference a connection by ID, so an imported one must keep the document's.
func (s *HTTPConnectionServiceTestSuite) TestCreateKeepsASuppliedID() {
	stored := s.expectCreate()

	created, svcErr := s.service.CreateHTTPConnection(s.ctx, HTTPConnection{ID: "imported-id", OUID: "ou-1",
		Name: "Risk", URL: testURL})

	s.Require().Nil(svcErr)
	s.Equal("imported-id", created.ID)
	s.Equal("imported-id", stored.ID)
}

func (s *HTTPConnectionServiceTestSuite) TestDeclarativeConnectionsAreReadOnly() {
	s.mockStore.On("GetHTTPConnection", mock.Anything, "file-1").
		Return(&HTTPConnection{ID: "file-1", OUID: "ou-1", IsReadOnly: true}, nil)
	s.mockStore.On("UpdateHTTPConnection", mock.Anything, mock.Anything).Return(errHTTPConnectionIsImmutable)
	s.mockStore.On("DeleteHTTPConnection", mock.Anything, "file-1").Return(errHTTPConnectionIsImmutable)
	s.service.SetDependencyRegistry(s.registryReporting("file-1"))

	_, svcErr := s.service.UpdateHTTPConnection(s.ctx, "file-1", HTTPConnection{Name: "Risk", URL: testURL})
	s.Equal(declarativeresource.ErrorDeclarativeResourceUpdateOperation.Code, svcErr.Code)
	s.Equal(declarativeresource.ErrorDeclarativeResourceDeleteOperation.Code,
		s.service.DeleteHTTPConnection(s.ctx, "file-1").Code)
}

// Header names are case-insensitive (RFC 9110), so a refused name and a repeated name are each
// matched in any casing.
func (s *HTTPConnectionServiceTestSuite) TestHeaderNamesAreComparedInAnyCase() {
	for _, name := range []string{"Authorization", "authorization", "AUTHORIZATION", "aUtHoRiZaTiOn"} {
		s.Run(name, func() {
			s.assertCreateRejected(HTTPConnection{OUID: "ou-1", Name: "Risk", URL: testURL,
				Headers: []Header{{Name: name, Value: "Basic c3ZjOnB3"}}}, ErrorInvalidHeaders)
		})
	}
	s.assertCreateRejected(HTTPConnection{OUID: "ou-1", Name: "Risk", URL: testURL,
		Headers: []Header{{Name: "x-tenant", Value: "a"}, {Name: "X-TENANT", Value: "b"}}}, ErrorInvalidHeaders)
}

func (s *HTTPConnectionServiceTestSuite) TestValidateHTTPConnectionAppliesTheServiceRules() {
	connection := HTTPConnection{Name: " Risk ", URL: "HTTPS://Risk.Example.COM:443/x"}
	s.Require().Nil(s.service.ValidateHTTPConnection(s.ctx, &connection))
	s.Equal("Risk", connection.Name)
	s.Equal("https://risk.example.com/x", connection.URL)
	s.Equal(defaultTimeoutMS, connection.TimeoutMS)

	for name, tc := range map[string]struct {
		invalid HTTPConnection
		want    tidcommon.ServiceError
	}{
		"credential over http": {HTTPConnection{Name: "Risk", URL: "http://risk.example.com/x",
			AuthProperties: basicAuthProperties(s.T())}, ErrorInvalidURL},
		"reserved header": {HTTPConnection{Name: "Risk", URL: testURL,
			Headers: []Header{{Name: "Authorization", Value: "Basic c3ZjOnB3"}}}, ErrorInvalidHeaders},
		"missing name": {HTTPConnection{URL: testURL}, ErrorInvalidName},
	} {
		s.Run(name, func() {
			svcErr := s.service.ValidateHTTPConnection(s.ctx, &tc.invalid)
			s.Require().NotNil(svcErr)
			s.Equal(tc.want.Code, svcErr.Code)
		})
	}
}

// withDeclared serves the service over the composite store, with one declared connection, file-1
// named Declared, and returns the database store beneath it.
func (s *HTTPConnectionServiceTestSuite) withDeclared() *httpConnectionStoreInterfaceMock {
	fileStore := &fileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeHTTPConnection),
	}
	s.Require().NoError(fileStore.Create("file-1", &HTTPConnection{ID: "file-1", Name: "Declared", OUID: "ou-1",
		URL: testURL}))
	dbStore := newHttpConnectionStoreInterfaceMock(s.T())
	s.service = newHTTPConnectionService(newCompositeStore(fileStore, dbStore), s.mockOU)
	return dbStore
}

func (s *HTTPConnectionServiceTestSuite) TestValidateHTTPConnectionCreateRefusesWhatCreateRefuses() {
	stored := &HTTPConnection{ID: "db-1", Name: "Stored", OUID: "ou-1", URL: testURL}
	for name, tc := range map[string]struct {
		connection HTTPConnection
		want       tidcommon.ServiceError
	}{
		"the ID of a declared connection": {HTTPConnection{ID: "file-1", OUID: "ou-1", Name: "Risk",
			URL: testURL}, ErrorAlreadyExists},
		"the ID of a stored connection": {HTTPConnection{ID: "db-1", OUID: "ou-1", Name: "Risk", URL: testURL},
			ErrorAlreadyExists},
		"the name of a declared connection": {HTTPConnection{ID: "new-1", OUID: "ou-1", Name: "Declared",
			URL: testURL}, ErrorAlreadyExists},
		"the name of a stored connection": {HTTPConnection{OUID: "ou-1", Name: "Stored", URL: testURL},
			ErrorAlreadyExists},
		"an invalid connection": {HTTPConnection{ID: "new-1", OUID: "ou-1", URL: testURL}, ErrorInvalidName},
		"no owner": {HTTPConnection{ID: "new-1", Name: "Risk", URL: testURL},
			ErrorOURequired},
		"an unknown owner": {HTTPConnection{ID: "new-1", OUID: "ou-missing", Name: "Risk", URL: testURL},
			ErrorInvalidOU},
	} {
		s.Run(name, func() {
			dbStore := s.withDeclared()
			dbStore.On("GetHTTPConnection", mock.Anything, "file-1").Return(nil, nil).Maybe()
			dbStore.On("GetHTTPConnection", mock.Anything, "db-1").Return(stored, nil).Maybe()
			dbStore.On("GetHTTPConnection", mock.Anything, "new-1").Return(nil, nil).Maybe()
			dbStore.On("GetHTTPConnectionByName", mock.Anything, "Declared").Return(nil, nil).Maybe()
			dbStore.On("GetHTTPConnectionByName", mock.Anything, "Stored").Return(stored, nil).Maybe()
			s.mockOU.On("ResolveOwner", mock.Anything, "", "", mock.Anything).Return("", &ErrorOURequired).Maybe()
			s.mockOU.On("ResolveOwner", mock.Anything, "ou-missing", "", mock.Anything).
				Return("", &ErrorInvalidOU).Maybe()

			svcErr := s.service.ValidateHTTPConnectionCreate(s.ctx, &tc.connection)

			s.Require().NotNil(svcErr)
			s.Equal(tc.want.Code, svcErr.Code)
		})
	}
}

func (s *HTTPConnectionServiceTestSuite) TestValidateHTTPConnectionUpdateRefusesWhatUpdateRefuses() {
	stored := &HTTPConnection{ID: "db-1", Name: "Risk", OUID: "ou-1", URL: testURL}
	other := &HTTPConnection{ID: "db-2", Name: "Other", OUID: "ou-1", URL: testURL}
	for name, tc := range map[string]struct {
		id, name, ouID string
		want           *tidcommon.ServiceError
	}{
		"a declared connection": {"file-1", "Declared", "",
			&declarativeresource.ErrorDeclarativeResourceUpdateOperation},
		"another owner of a declared connection": {"file-1", "Declared", "ou-2",
			&ErrorOUUpdateNotAllowed},
		"another owner":                         {"db-1", "Risk", "ou-2", &ErrorOUUpdateNotAllowed},
		"the name of another stored connection": {"db-1", "Other", "", &ErrorAlreadyExists},
		"the name of a declared connection":     {"db-1", "Declared", "", &ErrorAlreadyExists},
		"an invalid connection":                 {"db-1", "", "", &ErrorInvalidName},
		"its own name":                          {"db-1", "Risk", "", nil},
		"its own owner":                         {"db-1", "Risk", "ou-1", nil},
	} {
		s.Run(name, func() {
			dbStore := s.withDeclared()
			dbStore.On("GetHTTPConnection", mock.Anything, "file-1").Return(nil, nil).Maybe()
			dbStore.On("GetHTTPConnection", mock.Anything, "db-1").Return(stored, nil).Maybe()
			dbStore.On("GetHTTPConnectionByName", mock.Anything, "Risk").Return(stored, nil).Maybe()
			dbStore.On("GetHTTPConnectionByName", mock.Anything, "Other").Return(other, nil).Maybe()
			dbStore.On("GetHTTPConnectionByName", mock.Anything, "Declared").Return(nil, nil).Maybe()
			s.mockOU.On("ResolveOwnerForUpdate", mock.Anything, "ou-2", "", "ou-1", mock.Anything).
				Return("", &ErrorOUUpdateNotAllowed).Maybe()

			existing, svcErr := s.service.GetHTTPConnection(s.ctx, tc.id)
			s.Require().Nil(svcErr)

			svcErr = s.service.ValidateHTTPConnectionUpdate(s.ctx, existing,
				&HTTPConnection{Name: tc.name, OUID: tc.ouID, URL: testURL})

			if tc.want == nil {
				s.Nil(svcErr)
				return
			}
			s.Require().NotNil(svcErr)
			s.Equal(tc.want.Code, svcErr.Code)
		})
	}
}

func (s *HTTPConnectionServiceTestSuite) TestValidateIsRefusedInDeclarativeMode() {
	initTestRuntime(s.T(), true)

	s.Equal(declarativeresource.ErrorDeclarativeResourceCreateOperation.Code,
		s.service.ValidateHTTPConnectionCreate(s.ctx, &HTTPConnection{OUID: "ou-1", Name: "Risk", URL: testURL}).Code)
	s.Equal(declarativeresource.ErrorDeclarativeResourceUpdateOperation.Code,
		s.service.ValidateHTTPConnectionUpdate(s.ctx, &HTTPConnection{ID: "id"},
			&HTTPConnection{Name: "Risk", URL: testURL}).Code)
}

func (s *HTTPConnectionServiceTestSuite) TestValidateLookupFailuresAreServerErrors() {
	failure := errors.New("database unavailable")
	s.mockStore.On("GetHTTPConnection", mock.Anything, "conn-1").Return(nil, failure)
	s.mockStore.On("GetHTTPConnectionByName", mock.Anything, "Risk").Return(nil, failure)

	s.Equal(tidcommon.InternalServerError.Code, s.service.ValidateHTTPConnectionCreate(s.ctx,
		&HTTPConnection{ID: "conn-1", OUID: "ou-1", Name: "Risk", URL: testURL}).Code)
	s.Equal(tidcommon.InternalServerError.Code, s.service.ValidateHTTPConnectionCreate(s.ctx,
		&HTTPConnection{OUID: "ou-1", Name: "Risk", URL: testURL}).Code)
	s.Equal(tidcommon.InternalServerError.Code, s.service.ValidateHTTPConnectionUpdate(s.ctx,
		&HTTPConnection{ID: "conn-1"}, &HTTPConnection{Name: "Risk", URL: testURL}).Code)
}

func (s *HTTPConnectionServiceTestSuite) TestAPIKeyHeaderNamesAreStoredCanonical() {
	stored := s.expectCreate()

	_, svcErr := s.service.CreateHTTPConnection(s.ctx, s.connection(testURL,
		authProperties(s.T(), outboundauth.TypeAPIKey, map[string]string{"x-tenant-key": testPassword})))

	s.Require().Nil(svcErr)
	authConfig, err := outboundauth.FromProperties(stored.AuthProperties)
	s.Require().NoError(err)
	s.Equal(map[string]string{"X-Tenant-Key": testPassword}, authConfig.Properties)
}
