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

package flowexec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
)

// FlowStoreTestSuite exercises the flowStore adapter against a real in-memory runtime store,
// verifying the marshal/namespace/key round-trip and the not-found/update semantics.
type FlowStoreTestSuite struct {
	suite.Suite
	store flowStoreInterface
	ctx   context.Context
}

func TestFlowStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FlowStoreTestSuite))
}

func (s *FlowStoreTestSuite) SetupTest() {
	s.store = newFlowStore(inmemory.Initialize("test-deployment"))
	s.ctx = context.Background()
}

func (s *FlowStoreTestSuite) TestStoreAndGet() {
	model := FlowContextDB{ExecutionID: "exec-1", Context: `{"alg":"AES-GCM","ciphertext":"abc"}`}
	s.Require().NoError(s.store.StoreFlowContext(s.ctx, model, 60))

	got, err := s.store.GetFlowContext(s.ctx, "exec-1")
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Equal("exec-1", got.ExecutionID)
	s.Equal(`{"alg":"AES-GCM","ciphertext":"abc"}`, got.Context)
}

func (s *FlowStoreTestSuite) TestGet_NotFound_ReturnsNil() {
	got, err := s.store.GetFlowContext(s.ctx, "missing")
	s.Require().NoError(err)
	s.Nil(got)
}

func (s *FlowStoreTestSuite) TestUpdate_PreservesRoundTrip() {
	model := FlowContextDB{ExecutionID: "exec-2", Context: "v1"}
	s.Require().NoError(s.store.StoreFlowContext(s.ctx, model, 60))

	model.Context = "v2"
	s.Require().NoError(s.store.UpdateFlowContext(s.ctx, model))

	got, err := s.store.GetFlowContext(s.ctx, "exec-2")
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Equal("v2", got.Context)
}

func (s *FlowStoreTestSuite) TestUpdate_Missing_ReturnsError() {
	err := s.store.UpdateFlowContext(s.ctx, FlowContextDB{ExecutionID: "missing", Context: "v"})
	s.Error(err)
}

func (s *FlowStoreTestSuite) TestDelete() {
	model := FlowContextDB{ExecutionID: "exec-3", Context: "v"}
	s.Require().NoError(s.store.StoreFlowContext(s.ctx, model, 60))
	s.Require().NoError(s.store.DeleteFlowContext(s.ctx, "exec-3"))

	got, err := s.store.GetFlowContext(s.ctx, "exec-3")
	s.Require().NoError(err)
	s.Nil(got)
}

func (s *FlowStoreTestSuite) TestDelete_MissingIsIdempotent() {
	s.NoError(s.store.DeleteFlowContext(s.ctx, "missing"))
}
