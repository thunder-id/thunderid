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
	"fmt"
	"sync"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// ExecutorRegistryInterface defines registry operations for executors.
type ExecutorRegistryInterface interface {
	GetExecutor(name string) (core.ExecutorInterface, error)
	RegisterExecutor(name string, ex core.ExecutorInterface)
	IsRegistered(name string) bool
}

// executorRegistry is the default implementation of ExecutorRegistryInterface.
type executorRegistry struct {
	mu        sync.RWMutex
	executors map[string]core.ExecutorInterface
}

// newExecutorRegistry creates a new instance of executorRegistry.
func newExecutorRegistry() ExecutorRegistryInterface {
	return &executorRegistry{
		executors: make(map[string]core.ExecutorInterface),
	}
}

// RegisterExecutor registers an executor instance.
func (r *executorRegistry) RegisterExecutor(name string, exec core.ExecutorInterface) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ExecutorRegistry"))
	logger.Debug("Registering executor", log.String("executorName", exec.GetName()))

	if exec == nil {
		logger.Warn("Skipping registration of nil executor")
		return
	}
	if name == "" {
		logger.Warn("Skipping registration of executor with empty name")
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.executors[name]; ok {
		logger.Warn("Executor already registered", log.String("executorName", name))
		return
	}
	r.executors[name] = exec
}

// GetExecutor retrieves executor instance from the executor registry.
func (r *executorRegistry) GetExecutor(name string) (core.ExecutorInterface, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ex, ok := r.executors[name]
	if !ok {
		return nil, fmt.Errorf("executor '%s' not found", name)
	}
	return ex, nil
}

// IsRegistered checks if an executor with the given name is registered.
func (r *executorRegistry) IsRegistered(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.executors[name]
	return ok
}
