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

package thunderidengine

import (
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
)

// ExecutorInterface is the contract for custom flow executors registered with the engine.
type ExecutorInterface = core.ExecutorInterface

// NodeContext carries per-node execution state for an executor.
type NodeContext = core.NodeContext

// ExecutorResponse is returned from executor execution.
type ExecutorResponse = common.ExecutorResponse

// ExecutorType classifies an executor implementation.
type ExecutorType = common.ExecutorType

// Input describes a required or optional executor input.
type Input = common.Input

// ExecutionPolicy configures executor behavior for a node.
type ExecutionPolicy = core.ExecutionPolicy

// ExecutorRegistry registers and resolves executors by name.
type ExecutorRegistry interface {
	GetExecutor(name string) (ExecutorInterface, error)
	RegisterExecutor(name string, ex ExecutorInterface)
	IsRegistered(name string) bool
}
