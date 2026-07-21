/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package connection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type MappingTestSuite struct {
	suite.Suite
}

func TestMappingSuite(t *testing.T) {
	suite.Run(t, new(MappingTestSuite))
}

func (s *MappingTestSuite) SetupTest() {
	initConfigWithTestCryptoKey()
}

func (s *MappingTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *MappingTestSuite) TestAppendPropertySkipsEmpty() {
	props, err := appendProperty(nil, idp.PropClientID, "", false)
	s.NoError(err)
	s.Empty(props)

	props, err = appendProperty(props, idp.PropClientID, "client-1", false)
	s.NoError(err)
	s.Len(props, 1)
	s.False(props[0].IsSecret())
	v, err := props[0].GetValue()
	s.NoError(err)
	s.Equal("client-1", v)
}

func (s *MappingTestSuite) TestAppendPropertyEncryptsSecret() {
	props, err := appendProperty(nil, idp.PropClientSecret, "s3cret", true)
	s.NoError(err)
	s.Len(props, 1)
	s.True(props[0].IsSecret())
	v, err := props[0].GetValue()
	s.NoError(err)
	s.Equal("s3cret", v)
}

func (s *MappingTestSuite) TestPropertyValuesMasksSecret() {
	props := []cmodels.Property{
		mustProperty(s.T(), idp.PropClientID, "client-1", false),
		mustProperty(s.T(), idp.PropClientSecret, "s3cret", true),
	}
	values, err := propertyValues(props)
	s.NoError(err)
	s.Equal("client-1", values[idp.PropClientID])
	s.Equal(maskedSecretValue, values[idp.PropClientSecret])
}

func (s *MappingTestSuite) TestMergeStoredSecretsKeepsOmitted() {
	// Secret omitted from the request → carried over from the stored connection.
	existing := []cmodels.Property{mustProperty(s.T(), idp.PropClientSecret, "stored", true)}

	merged := mergeStoredSecrets(nil, existing)
	s.Len(merged, 1)
	v, err := merged[0].GetValue()
	s.NoError(err)
	s.Equal("stored", v)
}

func (s *MappingTestSuite) TestMergeStoredSecretsUsesProvidedValue() {
	// Secret present in the request → used verbatim; stored value not carried over.
	incoming := []cmodels.Property{mustProperty(s.T(), idp.PropClientSecret, "new", true)}
	existing := []cmodels.Property{mustProperty(s.T(), idp.PropClientSecret, "stored", true)}

	merged := mergeStoredSecrets(incoming, existing)
	s.Len(merged, 1)
	v, err := merged[0].GetValue()
	s.NoError(err)
	s.Equal("new", v)
}

func (s *MappingTestSuite) TestMergeStoredSecretsOnlyBackfillsSecrets() {
	// A non-secret property omitted from the request is NOT carried over.
	existing := []cmodels.Property{mustProperty(s.T(), idp.PropRedirectURI, "https://app/cb", false)}

	merged := mergeStoredSecrets(nil, existing)
	s.Empty(merged)
}

func (s *MappingTestSuite) TestScopes() {
	s.Equal("openid,email", joinScopes([]string{"openid", "email"}))
	s.Equal([]string{"openid", "email"}, splitScopes("openid,email"))
	s.Nil(splitScopes(""))
}

func (s *MappingTestSuite) TestWriteServiceErrorStatusMapping() {
	cases := []struct {
		svcErr *tidcommon.ServiceError
		want   int
	}{
		{&idp.ErrorIDPNotFound, http.StatusNotFound},
		{&idp.ErrorIDPAlreadyExists, http.StatusConflict},
		{&idp.ErrorIDPHasBlockingDependencies, http.StatusConflict},
		{&idp.ErrorInvalidIDPID, http.StatusBadRequest},
		{&notification.ErrorSenderNotFound, http.StatusNotFound},
		{&notification.ErrorDuplicateSenderName, http.StatusConflict},
		{&notification.ErrorSenderHasBlockingDependencies, http.StatusConflict},
		{&notification.ErrorInvalidProvider, http.StatusBadRequest},
		{&tidcommon.InternalServerError, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		rr := httptest.NewRecorder()
		writeServiceError(context.Background(), rr, tc.svcErr)
		s.Equal(tc.want, rr.Code, tc.svcErr.Code)
	}
}

func (s *MappingTestSuite) TestWriteInvalidBody() {
	rr := httptest.NewRecorder()
	writeInvalidBody(context.Background(), rr)
	s.Equal(http.StatusBadRequest, rr.Code)
}
