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

package openid4vci

import (
	"context"
	"encoding/json"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/tests/mocks/usermock"
)

type ClaimsTestSuite struct {
	suite.Suite
}

func TestClaimsTestSuite(t *testing.T) {
	suite.Run(t, new(ClaimsTestSuite))
}

func (s *ClaimsTestSuite) TestResolveClaimsSelectsConfiguredClaims() {
	ctx := context.Background()
	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	attrs, _ := json.Marshal(map[string]interface{}{
		"given_name":  "Ada",
		"family_name": "Lovelace",
		"extra":       "ignored",
	})
	userSvc.EXPECT().GetUser(ctx, "u1", false).
		Return(&user.User{ID: "u1", Attributes: attrs}, nil)
	svc := &service{userService: userSvc}

	claims, err := svc.resolveClaims(ctx, "u1", []string{"given_name", "family_name", "missing"})
	s.Require().NoError(err)
	s.Equal("Ada", claims["given_name"])
	s.Equal("Lovelace", claims["family_name"])
	s.NotContains(claims, "extra")
	s.NotContains(claims, "missing")
}

func (s *ClaimsTestSuite) TestResolveClaimsUserNotFound() {
	ctx := context.Background()
	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	userSvc.EXPECT().GetUser(ctx, "u1", false).
		Return(nil, &tidcommon.ServiceError{Code: "not-found"})
	svc := &service{userService: userSvc}

	_, err := svc.resolveClaims(ctx, "u1", []string{"given_name"})
	s.ErrorIs(err, ErrUserNotFound)
}

func (s *ClaimsTestSuite) TestResolveClaimsBadAttributes() {
	ctx := context.Background()
	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	userSvc.EXPECT().GetUser(ctx, "u1", false).
		Return(&user.User{ID: "u1", Attributes: json.RawMessage("not-json")}, nil)
	svc := &service{userService: userSvc}

	_, err := svc.resolveClaims(ctx, "u1", []string{"given_name"})
	s.ErrorIs(err, ErrIssuance)
}

func (s *ClaimsTestSuite) TestResolveClaimsNoAttributes() {
	ctx := context.Background()
	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	userSvc.EXPECT().GetUser(ctx, "u1", false).
		Return(&user.User{ID: "u1"}, nil)
	svc := &service{userService: userSvc}

	claims, err := svc.resolveClaims(ctx, "u1", []string{"given_name"})
	s.Require().NoError(err)
	s.Empty(claims)
}
