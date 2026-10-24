// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type HTTPConnectionServiceTestSuite struct {
	suite.Suite
	ctx       context.Context
	mockStore *httpConnectionStoreInterfaceMock
	service   *httpConnectionService
}

func TestHTTPConnectionServiceTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionServiceTestSuite))
}

func (s *HTTPConnectionServiceTestSuite) SetupTest() {
	initTestRuntime(s.T(), false)
	s.ctx = context.Background()
	s.mockStore = newHttpConnectionStoreInterfaceMock(s.T())
	s.service = newHTTPConnectionService(s.mockStore)
}

func (s *HTTPConnectionServiceTestSuite) noneAuth() []cmodels.Property {
	return authProperties(s.T(), outboundauth.TypeNone, nil)
}

func (s *HTTPConnectionServiceTestSuite) connection(url string, auth []cmodels.Property) HTTPConnection {
	return HTTPConnection{Name: "Risk", URL: url, AuthProperties: auth}
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
		Name: " Risk ", Description: "Risk engine", URL: " HTTPS://Risk.Example.COM:443/evaluate?x=1 ",
		Headers:        []Header{{Name: " X-Tenant ", Value: "acme"}},
		AuthProperties: auth,
	})

	s.Require().Nil(svcErr)
	s.NotEmpty(created.ID)
	s.Equal(HTTPConnection{
		ID: created.ID, Name: "Risk", Description: "Risk engine", URL: "https://risk.example.com/evaluate?x=1",
		TimeoutMS: defaultTimeoutMS, Headers: []Header{{Name: "X-Tenant", Value: "acme"}}, AuthProperties: auth,
	}, *created)
	s.Equal(*created, *stored)
}

func (s *HTTPConnectionServiceTestSuite) TestCreateWithoutAuthenticationStoresNone() {
	stored := s.expectCreate()

	_, svcErr := s.service.CreateHTTPConnection(s.ctx, HTTPConnection{Name: "Risk", URL: "http://localhost:9000/x"})

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
	s.assertCreateRejected(HTTPConnection{Name: " ", URL: testURL}, ErrorInvalidName)
	s.assertCreateRejected(HTTPConnection{Name: strings.Repeat("n", maxNameLength+1), URL: testURL},
		ErrorInvalidName)
	s.assertCreateRejected(HTTPConnection{Name: "Risk", URL: testURL,
		Description: strings.Repeat("d", maxDescriptionLength+1)}, ErrorInvalidDescription)

	s.expectCreate()
	_, svcErr := s.service.CreateHTTPConnection(s.ctx, HTTPConnection{Name: strings.Repeat("é", maxNameLength),
		URL: testURL, Description: strings.Repeat("é", maxDescriptionLength)})
	s.Nil(svcErr, "limits count characters, not bytes")
}

func (s *HTTPConnectionServiceTestSuite) TestTimeoutBounds() {
	for _, timeout := range []int{-1, maxTimeoutMS + 1} {
		s.assertCreateRejected(HTTPConnection{Name: "Risk", URL: testURL, TimeoutMS: timeout}, ErrorInvalidTimeout)
	}
	for _, timeout := range []int{1, maxTimeoutMS} {
		stored := s.expectCreate()
		_, svcErr := s.service.CreateHTTPConnection(s.ctx, HTTPConnection{Name: "Risk", URL: testURL,
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
			s.assertCreateRejected(HTTPConnection{Name: "Risk", URL: testURL, Headers: headers}, ErrorInvalidHeaders)
		})
	}
}

func (s *HTTPConnectionServiceTestSuite) TestNameConflictIsReported() {
	s.mockStore.On("CreateHTTPConnection", mock.Anything, mock.Anything).
		Return(errors.Join(errNameConflict, errors.New("unique index"))).Once()

	s.assertCreateRejected(HTTPConnection{Name: "Risk", URL: testURL}, ErrorAlreadyExists)
}

func (s *HTTPConnectionServiceTestSuite) TestStoreFailuresAreServerErrors() {
	failure := errors.New("database unavailable")
	s.mockStore.On("CreateHTTPConnection", mock.Anything, mock.Anything).Return(failure)
	s.mockStore.On("ListHTTPConnections", mock.Anything).Return(nil, failure)
	s.mockStore.On("GetHTTPConnection", mock.Anything, "id").Return(nil, failure)
	s.mockStore.On("UpdateHTTPConnection", mock.Anything, mock.Anything).Return(failure)
	s.mockStore.On("DeleteHTTPConnection", mock.Anything, "id").Return(failure)

	s.assertCreateRejected(HTTPConnection{Name: "Risk", URL: testURL}, tidcommon.InternalServerError)
	_, svcErr := s.service.ListHTTPConnections(s.ctx)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	_, svcErr = s.service.GetHTTPConnection(s.ctx, "id")
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	_, svcErr = s.service.UpdateHTTPConnection(s.ctx, "id", HTTPConnection{Name: "Risk", URL: testURL})
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.Equal(tidcommon.InternalServerError.Code, s.service.DeleteHTTPConnection(s.ctx, "id").Code)
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

func (s *HTTPConnectionServiceTestSuite) TestUpdateAndDeleteReportMissingAndConflict() {
	s.mockStore.On("UpdateHTTPConnection", mock.Anything, mock.MatchedBy(func(c HTTPConnection) bool {
		return c.ID == "missing"
	})).Return(errConnectionNotFound)
	s.mockStore.On("UpdateHTTPConnection", mock.Anything, mock.MatchedBy(func(c HTTPConnection) bool {
		return c.ID == "taken"
	})).Return(errNameConflict)
	s.mockStore.On("DeleteHTTPConnection", mock.Anything, "missing").Return(errConnectionNotFound)
	s.mockStore.On("DeleteHTTPConnection", mock.Anything, "id").Return(nil)

	_, svcErr := s.service.UpdateHTTPConnection(s.ctx, "missing", HTTPConnection{Name: "Risk", URL: testURL})
	s.Equal(ErrorNotFound.Code, svcErr.Code)
	_, svcErr = s.service.UpdateHTTPConnection(s.ctx, "taken", HTTPConnection{Name: "Risk", URL: testURL})
	s.Equal(ErrorAlreadyExists.Code, svcErr.Code)
	s.Equal(ErrorNotFound.Code, s.service.DeleteHTTPConnection(s.ctx, "missing").Code)
	s.Equal(ErrorNotFound.Code, s.service.DeleteHTTPConnection(s.ctx, " ").Code)
	s.Nil(s.service.DeleteHTTPConnection(s.ctx, "id"))
}

func (s *HTTPConnectionServiceTestSuite) TestWritesAreRejectedInDeclarativeMode() {
	initTestRuntime(s.T(), true)

	s.assertCreateRejected(HTTPConnection{Name: "Risk", URL: testURL},
		declarativeresource.ErrorDeclarativeResourceCreateOperation)
	_, svcErr := s.service.UpdateHTTPConnection(s.ctx, "id", HTTPConnection{Name: "Risk", URL: testURL})
	s.Equal(declarativeresource.ErrorDeclarativeResourceUpdateOperation.Code, svcErr.Code)
	s.Equal(declarativeresource.ErrorDeclarativeResourceDeleteOperation.Code,
		s.service.DeleteHTTPConnection(s.ctx, "id").Code)
}
