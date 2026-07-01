/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package executor

import (
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type ExecutorRegistryTestSuite struct {
	suite.Suite
	registry ExecutorRegistryInterface
}

func TestExecutorRegistrySuite(t *testing.T) {
	suite.Run(t, new(ExecutorRegistryTestSuite))
}

func (suite *ExecutorRegistryTestSuite) SetupTest() {
	suite.registry = newExecutorRegistry()
}

func createMockExecutorForRegistry(t *testing.T, name string,
	executorType providers.ExecutorType) providers.Executor {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(name).Maybe()
	mockExec.On("GetType").Return(executorType).Maybe()
	return mockExec
}

func (suite *ExecutorRegistryTestSuite) TestnewExecutorRegistry_CreatesEmptyRegistry() {
	registry := newExecutorRegistry()

	assert.NotNil(suite.T(), registry)
	assert.False(suite.T(), registry.IsRegistered("any-executor"))
}

func (suite *ExecutorRegistryTestSuite) TestRegisterExecutor_SuccessfulRegistration() {
	mockExecutor := createMockExecutorForRegistry(suite.T(), "test-executor",
		providers.ExecutorTypeAuthentication)

	suite.registry.RegisterExecutor("test-executor", mockExecutor)

	assert.True(suite.T(), suite.registry.IsRegistered("test-executor"))
}

func (suite *ExecutorRegistryTestSuite) TestRegisterExecutor_EmptyName() {
	mockExecutor := createMockExecutorForRegistry(suite.T(), "test-executor",
		providers.ExecutorTypeAuthentication)

	suite.registry.RegisterExecutor("", mockExecutor)

	assert.False(suite.T(), suite.registry.IsRegistered(""))
}

func (suite *ExecutorRegistryTestSuite) TestRegisterExecutor_NilExecutor() {
	suite.registry.RegisterExecutor("nil-executor", nil)

	assert.False(suite.T(), suite.registry.IsRegistered("nil-executor"))
}

func (suite *ExecutorRegistryTestSuite) TestRegisterExecutor_DuplicateRegistration() {
	mockExecutor1 := createMockExecutorForRegistry(suite.T(), "test-executor",
		providers.ExecutorTypeAuthentication)
	mockExecutor2 := createMockExecutorForRegistry(suite.T(), "test-executor",
		providers.ExecutorTypeUtility)

	suite.registry.RegisterExecutor("test-executor", mockExecutor1)
	suite.registry.RegisterExecutor("test-executor", mockExecutor2)

	retrieved, err := suite.registry.GetExecutor("test-executor")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-executor", retrieved.GetName())
	assert.Equal(suite.T(), providers.ExecutorTypeAuthentication, retrieved.GetType())
}

func (suite *ExecutorRegistryTestSuite) TestRegisterExecutor_MultipleExecutors() {
	executor1 := createMockExecutorForRegistry(suite.T(), "executor1",
		providers.ExecutorTypeAuthentication)
	executor2 := createMockExecutorForRegistry(suite.T(), "executor2",
		providers.ExecutorTypeUtility)
	executor3 := createMockExecutorForRegistry(suite.T(), "executor3",
		providers.ExecutorTypeRegistration)

	suite.registry.RegisterExecutor("executor1", executor1)
	suite.registry.RegisterExecutor("executor2", executor2)
	suite.registry.RegisterExecutor("executor3", executor3)

	assert.True(suite.T(), suite.registry.IsRegistered("executor1"))
	assert.True(suite.T(), suite.registry.IsRegistered("executor2"))
	assert.True(suite.T(), suite.registry.IsRegistered("executor3"))
}

func (suite *ExecutorRegistryTestSuite) TestGetExecutor_ExistingExecutor() {
	mockExecutor := createMockExecutorForRegistry(suite.T(), "test-executor",
		providers.ExecutorTypeAuthentication)
	suite.registry.RegisterExecutor("test-executor", mockExecutor)

	retrieved, err := suite.registry.GetExecutor("test-executor")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), retrieved)
	assert.Equal(suite.T(), "test-executor", retrieved.GetName())
	assert.Equal(suite.T(), providers.ExecutorTypeAuthentication, retrieved.GetType())
}

func (suite *ExecutorRegistryTestSuite) TestGetExecutor_NonExistentExecutor() {
	retrieved, err := suite.registry.GetExecutor("non-existent")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), retrieved)
	assert.Contains(suite.T(), err.Error(), "not found")
}

func (suite *ExecutorRegistryTestSuite) TestGetExecutor_EmptyName() {
	retrieved, err := suite.registry.GetExecutor("")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), retrieved)
}

func (suite *ExecutorRegistryTestSuite) TestIsRegistered_RegisteredExecutor() {
	mockExecutor := createMockExecutorForRegistry(suite.T(), "test-executor",
		providers.ExecutorTypeAuthentication)
	suite.registry.RegisterExecutor("test-executor", mockExecutor)

	isRegistered := suite.registry.IsRegistered("test-executor")

	assert.True(suite.T(), isRegistered)
}

func (suite *ExecutorRegistryTestSuite) TestIsRegistered_UnregisteredExecutor() {
	isRegistered := suite.registry.IsRegistered("non-existent")

	assert.False(suite.T(), isRegistered)
}

func (suite *ExecutorRegistryTestSuite) TestIsRegistered_EmptyName() {
	isRegistered := suite.registry.IsRegistered("")

	assert.False(suite.T(), isRegistered)
}

func (suite *ExecutorRegistryTestSuite) TestRegistryIsolation() {
	registry1 := newExecutorRegistry()
	registry2 := newExecutorRegistry()

	executor := createMockExecutorForRegistry(suite.T(), "test-executor",
		providers.ExecutorTypeAuthentication)
	registry1.RegisterExecutor("test-executor", executor)

	assert.True(suite.T(), registry1.IsRegistered("test-executor"))
	assert.False(suite.T(), registry2.IsRegistered("test-executor"))
}

func (suite *ExecutorRegistryTestSuite) TestConcurrentRegistration() {
	executor1 := createMockExecutorForRegistry(suite.T(), "executor1",
		providers.ExecutorTypeAuthentication)
	executor2 := createMockExecutorForRegistry(suite.T(), "executor2",
		providers.ExecutorTypeUtility)

	done := make(chan bool)

	go func() {
		suite.registry.RegisterExecutor("executor1", executor1)
		done <- true
	}()

	go func() {
		suite.registry.RegisterExecutor("executor2", executor2)
		done <- true
	}()

	<-done
	<-done

	assert.True(suite.T(), suite.registry.IsRegistered("executor1"))
	assert.True(suite.T(), suite.registry.IsRegistered("executor2"))
}

func (suite *ExecutorRegistryTestSuite) TestConcurrentRetrieval() {
	executor := createMockExecutorForRegistry(suite.T(), "test-executor",
		providers.ExecutorTypeAuthentication)
	suite.registry.RegisterExecutor("test-executor", executor)

	results := make(chan providers.Executor, 2)
	errors := make(chan error, 2)

	go func() {
		exec, err := suite.registry.GetExecutor("test-executor")
		results <- exec
		errors <- err
	}()

	go func() {
		exec, err := suite.registry.GetExecutor("test-executor")
		results <- exec
		errors <- err
	}()

	for i := 0; i < 2; i++ {
		exec := <-results
		err := <-errors
		assert.NoError(suite.T(), err)
		assert.NotNil(suite.T(), exec)
		assert.Equal(suite.T(), "test-executor", exec.GetName())
	}
}

func (suite *ExecutorRegistryTestSuite) TestRegisterExecutor_DifferentExecutorTypes() {
	tests := []struct {
		name     string
		execType providers.ExecutorType
	}{
		{"auth_executor", providers.ExecutorTypeAuthentication},
		{"utility_executor", providers.ExecutorTypeUtility},
		{"registration_executor", providers.ExecutorTypeRegistration},
	}

	for _, tt := range tests {
		mockExecutor := createMockExecutorForRegistry(suite.T(), tt.name, tt.execType)
		suite.registry.RegisterExecutor(tt.name, mockExecutor)

		retrieved, err := suite.registry.GetExecutor(tt.name)
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), tt.execType, retrieved.GetType())
	}
}

func (suite *ExecutorRegistryTestSuite) TestGetExecutor_NonExistentAfterRegistration() {
	executor1 := createMockExecutorForRegistry(suite.T(), "executor1",
		providers.ExecutorTypeAuthentication)
	suite.registry.RegisterExecutor("executor1", executor1)

	retrieved, err := suite.registry.GetExecutor("executor2")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), retrieved)
}
