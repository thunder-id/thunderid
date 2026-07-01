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

package core

import (
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
)

const (
	testInterceptorName = "test-interceptor"
)

type InterceptorTestSuite struct {
	suite.Suite
}

func TestInterceptorTestSuite(t *testing.T) {
	suite.Run(t, new(InterceptorTestSuite))
}

// newInterceptor / interceptor tests

func (s *InterceptorTestSuite) TestNewInterceptor() {
	ic := newInterceptor(testInterceptorName, false, 100)

	s.NotNil(ic)
	s.Equal(testInterceptorName, ic.GetName())
	s.False(ic.IsDefault())
	s.Equal(100, ic.GetPriority())
}

func (s *InterceptorTestSuite) TestGetName() {
	ic := newInterceptor(testInterceptorName, true, 1)
	s.Equal(testInterceptorName, ic.GetName())
}

func (s *InterceptorTestSuite) TestIsDefault_True() {
	ic := newInterceptor(testInterceptorName, true, 1)
	s.True(ic.IsDefault())
}

func (s *InterceptorTestSuite) TestIsDefault_False() {
	ic := newInterceptor(testInterceptorName, false, 1)
	s.False(ic.IsDefault())
}

func (s *InterceptorTestSuite) TestGetPriority() {
	ic := newInterceptor(testInterceptorName, true, 42)
	s.Equal(42, ic.GetPriority())
}

func (s *InterceptorTestSuite) TestExecute_ReturnsNil() {
	ic := newInterceptor(testInterceptorName, true, 1)
	ctx := &InterceptorContext{ExecutionID: "exec-001"}

	resp, err := ic.Execute(ctx)

	s.Nil(resp)
	s.Nil(err)
}

// InterceptorResponse tests

func (s *InterceptorTestSuite) TestInterceptorResponse_Pass() {
	result := &common.InterceptorResponse{
		Status: common.InterceptorStatusComplete,
		EngineOutputs: map[string]string{
			"challengeToken": "rotated-token",
		},
	}

	s.Equal(common.InterceptorStatusComplete, result.Status)
	s.Nil(result.Error)
	s.Equal("rotated-token", result.EngineOutputs["challengeToken"])
}

func (s *InterceptorTestSuite) TestInterceptorResponse_Fail() {
	failErr := &tidcommon.ServiceError{
		Code: "INT-001",
	}
	result := &common.InterceptorResponse{
		Status: common.InterceptorStatusFailure,
		Error:  failErr,
	}

	s.Equal(common.InterceptorStatusFailure, result.Status)
	s.Equal("INT-001", result.Error.Code)
	s.Nil(result.EngineOutputs)
}

// Interface compliance

func (s *InterceptorTestSuite) TestInterceptor_ImplementsInterface() {
	var _ InterceptorInterface = (*interceptor)(nil)
}
