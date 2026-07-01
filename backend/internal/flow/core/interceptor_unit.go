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
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// InterceptorUnitInterface defines the contract for an interceptor binding. A binding
// associates a named interceptor from the registry with its flow-level configuration
// (mode, scope, properties), similar to how a task execution node binds an executor by name.
type InterceptorUnitInterface interface {
	GetName() string
	GetMode() providers.InterceptorMode
	GetScope() providers.InterceptorScope
	GetApplyTo() []string
	GetProperties() map[string]interface{}
	GetInterceptor() InterceptorInterface
	SetName(name string)
	SetMode(mode providers.InterceptorMode)
	SetScope(scope providers.InterceptorScope)
	SetApplyTo(applyTo []string)
	SetProperties(properties map[string]interface{})
	SetInterceptor(interceptor InterceptorInterface)
	Clone() InterceptorUnitInterface
}

// interceptorUnit is the runtime representation of an interceptor declaration used within
// the interception context. It binds a named interceptor from the registry to its flow-level
// configuration. The Interceptor field is resolved from the registry at initialization time
// and is not serialized.
type interceptorUnit struct {
	name        string
	mode        providers.InterceptorMode
	scope       providers.InterceptorScope
	applyTo     []string
	properties  map[string]interface{}
	interceptor InterceptorInterface
}

var _ InterceptorUnitInterface = (*interceptorUnit)(nil)

// newInterceptorUnit creates a new interceptor execution unit with the given properties.
func newInterceptorUnit(name string, mode providers.InterceptorMode, scope providers.InterceptorScope,
	applyTo []string, properties map[string]interface{}) *interceptorUnit {
	return &interceptorUnit{
		name:       name,
		mode:       mode,
		scope:      scope,
		applyTo:    applyTo,
		properties: properties,
	}
}

// GetName returns the name of the interceptor as referenced in the registry.
func (b *interceptorUnit) GetName() string {
	return b.name
}

// GetMode returns the execution mode (pre or post) of the interceptor.
func (b *interceptorUnit) GetMode() providers.InterceptorMode {
	return b.mode
}

// GetScope returns the scope at which the interceptor applies.
func (b *interceptorUnit) GetScope() providers.InterceptorScope {
	return b.scope
}

// GetApplyTo returns the list of step IDs this interceptor is scoped to.
func (b *interceptorUnit) GetApplyTo() []string {
	return b.applyTo
}

// GetProperties returns the configuration properties for the interceptor.
func (b *interceptorUnit) GetProperties() map[string]interface{} {
	return b.properties
}

// GetInterceptor returns the resolved interceptor instance.
func (b *interceptorUnit) GetInterceptor() InterceptorInterface {
	return b.interceptor
}

// SetName sets the name of the interceptor as referenced in the registry.
func (b *interceptorUnit) SetName(name string) {
	b.name = name
}

// SetMode sets the execution mode (pre or post) of the interceptor.
func (b *interceptorUnit) SetMode(mode providers.InterceptorMode) {
	b.mode = mode
}

// SetScope sets the scope at which the interceptor applies.
func (b *interceptorUnit) SetScope(scope providers.InterceptorScope) {
	b.scope = scope
}

// SetApplyTo sets the list of step IDs this interceptor is scoped to.
func (b *interceptorUnit) SetApplyTo(applyTo []string) {
	b.applyTo = applyTo
}

// SetProperties sets the configuration properties for the interceptor.
func (b *interceptorUnit) SetProperties(properties map[string]interface{}) {
	b.properties = properties
}

// SetInterceptor sets the resolved interceptor instance on the binding.
func (b *interceptorUnit) SetInterceptor(ic InterceptorInterface) {
	b.interceptor = ic
}

// Clone returns a shallow copy of the interceptor unit without the resolved interceptor reference.
func (b *interceptorUnit) Clone() InterceptorUnitInterface {
	return &interceptorUnit{
		name:       b.name,
		mode:       b.mode,
		scope:      b.scope,
		applyTo:    b.applyTo,
		properties: b.properties,
	}
}
