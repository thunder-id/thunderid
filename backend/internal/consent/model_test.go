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

package consent

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConsentModelTestSuite struct {
	suite.Suite
}

func TestConsentModelTestSuite(t *testing.T) {
	suite.Run(t, new(ConsentModelTestSuite))
}

func (s *ConsentModelTestSuite) TestConsentStatus_IsValid() {
	cases := []struct {
		status ConsentStatus
		valid  bool
	}{
		{ConsentStatusActive, true},
		{ConsentStatusExpired, true},
		{"UNKNOWN", false},
		{"", false},
	}
	for _, tc := range cases {
		s.Equal(tc.valid, tc.status.IsValid())
	}
}

func (s *ConsentModelTestSuite) TestNamespace_IsValid() {
	cases := []struct {
		namespace Namespace
		valid     bool
	}{
		{NamespaceAttribute, true},
		{NamespacePermission, true},
		{"other", false},
		{"", false},
	}
	for _, tc := range cases {
		s.Equal(tc.valid, tc.namespace.IsValid())
	}
}

func (s *ConsentModelTestSuite) TestConsentAuthorizationType_IsValid() {
	cases := []struct {
		authType ConsentAuthorizationType
		valid    bool
	}{
		{AuthorizationTypeAuthorization, true},
		{AuthorizationTypeReAuthorization, true},
		{"OTHER", false},
		{"", false},
	}
	for _, tc := range cases {
		s.Equal(tc.valid, tc.authType.IsValid())
	}
}

func (s *ConsentModelTestSuite) TestConsentAuthorizationStatus_IsValid() {
	cases := []struct {
		status ConsentAuthorizationStatus
		valid  bool
	}{
		{AuthorizationStatusCreated, true},
		{AuthorizationStatusApproved, true},
		{AuthorizationStatusRejected, true},
		{"OTHER", false},
		{"", false},
	}
	for _, tc := range cases {
		s.Equal(tc.valid, tc.status.IsValid())
	}
}
