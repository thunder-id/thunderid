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
	"github.com/thunder-id/thunderid/internal/flow/common"
)

// InterceptorUnitInterface defines the contract for an interceptor binding. A binding
// associates a named interceptor from the registry with its flow-level configuration
// (mode, scope, properties), similar to how a task execution node binds an executor by name.
type InterceptorUnitInterface interface {
	GetName() string
	GetMode() common.InterceptorMode
	GetScope() common.InterceptorScope
	GetApplyTo() []string
	GetProperties() map[string]interface{}
	GetInterceptor() InterceptorInterface
	SetName(name string)
	SetMode(mode common.InterceptorMode)
	SetScope(scope common.InterceptorScope)
	SetApplyTo(applyTo []string)
	SetProperties(properties map[string]interface{})
	SetInterceptor(interceptor InterceptorInterface)
}

// interceptorUnit is the runtime representation of an interceptor declaration used within
// the interception context. It binds a named interceptor from the registry to its flow-level
// configuration. The Interceptor field is resolved from the registry at initialization time
// and is not serialized.
type interceptorUnit struct {
	Name        string                  `json:"name"`
	Mode        common.InterceptorMode  `json:"mode"`
	Scope       common.InterceptorScope `json:"scope,omitempty"`
	ApplyTo     []string                `json:"applyTo,omitempty"`
	Properties  map[string]interface{}  `json:"properties,omitempty"`
	Interceptor InterceptorInterface    `json:"-"`
}

var _ InterceptorUnitInterface = (*interceptorUnit)(nil)

// newInterceptorUnit creates a new interceptor execution unit with the given properties.
func newInterceptorUnit(name string, mode common.InterceptorMode, scope common.InterceptorScope,
	applyTo []string, properties map[string]interface{}) *interceptorUnit {
	return &interceptorUnit{
		Name:       name,
		Mode:       mode,
		Scope:      scope,
		ApplyTo:    applyTo,
		Properties: properties,
	}
}

// GetName returns the name of the interceptor as referenced in the registry.
func (b *interceptorUnit) GetName() string {
	return b.Name
}

// GetMode returns the execution mode (pre or post) of the interceptor.
func (b *interceptorUnit) GetMode() common.InterceptorMode {
	return b.Mode
}

// GetScope returns the scope at which the interceptor applies.
func (b *interceptorUnit) GetScope() common.InterceptorScope {
	return b.Scope
}

// GetApplyTo returns the list of step IDs this interceptor is scoped to.
func (b *interceptorUnit) GetApplyTo() []string {
	return b.ApplyTo
}

// GetProperties returns the configuration properties for the interceptor.
func (b *interceptorUnit) GetProperties() map[string]interface{} {
	return b.Properties
}

// GetInterceptor returns the resolved interceptor instance.
func (b *interceptorUnit) GetInterceptor() InterceptorInterface {
	return b.Interceptor
}

// SetName sets the name of the interceptor as referenced in the registry.
func (b *interceptorUnit) SetName(name string) {
	b.Name = name
}

// SetMode sets the execution mode (pre or post) of the interceptor.
func (b *interceptorUnit) SetMode(mode common.InterceptorMode) {
	b.Mode = mode
}

// SetScope sets the scope at which the interceptor applies.
func (b *interceptorUnit) SetScope(scope common.InterceptorScope) {
	b.Scope = scope
}

// SetApplyTo sets the list of step IDs this interceptor is scoped to.
func (b *interceptorUnit) SetApplyTo(applyTo []string) {
	b.ApplyTo = applyTo
}

// SetProperties sets the configuration properties for the interceptor.
func (b *interceptorUnit) SetProperties(properties map[string]interface{}) {
	b.Properties = properties
}

// SetInterceptor sets the resolved interceptor instance on the binding.
func (b *interceptorUnit) SetInterceptor(ic InterceptorInterface) {
	b.Interceptor = ic
}

// InterceptorInterface defines the contract for flow interceptors.
type InterceptorInterface interface {
	// Execute runs the interceptor logic and returns a result.
	Execute(ctx *InterceptorContext) (*common.InterceptorResponse, error)

	// GetName returns the unique name of the interceptor.
	GetName() string

	// IsDefault returns true if this is a default (always enforced) interceptor.
	IsDefault() bool

	// GetPriority returns the execution order within the same mode (lower runs first).
	GetPriority() int
}

// interceptor represents the basic implementation of an interceptor.
type interceptor struct {
	Name      string
	isDefault bool
	Priority  int
}

var _ InterceptorInterface = (*interceptor)(nil)

// newInterceptor creates a new instance of interceptor with the given properties.
func newInterceptor(name string, isDefault bool, priority int) InterceptorInterface {
	return &interceptor{
		Name:      name,
		isDefault: isDefault,
		Priority:  priority,
	}
}

// GetName returns the name of the interceptor.
func (i *interceptor) GetName() string {
	return i.Name
}

// IsDefault returns true if this is a default (always enforced) interceptor.
func (i *interceptor) IsDefault() bool {
	return i.isDefault
}

// GetPriority returns the execution order within the same mode (lower runs first).
func (i *interceptor) GetPriority() int {
	return i.Priority
}

// Execute runs the interceptor logic and returns a result.
func (i *interceptor) Execute(ctx *InterceptorContext) (*common.InterceptorResponse, error) {
	// Placeholder implementation. Concrete interceptors should override this method.
	return nil, nil
}
