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

package interceptor

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// ----- Registry -----
// Registry implements an in-memory registry for interceptors,
// supporting registration and retrieval by name.
// It is designed to be thread-safe and to prevent duplicate registrations.

// InterceptorRegistryInterface defines registry operations for interceptors.
type InterceptorRegistryInterface interface {
	RegisterInterceptor(name string, interceptor core.InterceptorInterface)
	GetInterceptor(name string) (core.InterceptorInterface, error)
	IsRegistered(name string) bool
}

// interceptorRegistry is the default implementation of InterceptorRegistryInterface.
type interceptorRegistry struct {
	mu           sync.RWMutex
	interceptors map[string]core.InterceptorInterface
}

// newInterceptorRegistry creates a new instance of interceptorRegistry.
func newInterceptorRegistry() InterceptorRegistryInterface {
	return &interceptorRegistry{
		interceptors: make(map[string]core.InterceptorInterface),
	}
}

// RegisterInterceptor registers an interceptor instance.
func (r *interceptorRegistry) RegisterInterceptor(name string, ic core.InterceptorInterface) {
	// Interceptors are registered at server startup, outside any request,
	// so there is no request context (or trace ID) to propagate.
	ctx := context.Background()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InterceptorRegistry"))
	logger.Debug(ctx, "Registering interceptor", log.String("interceptorName", name))

	if ic == nil {
		logger.Warn(ctx, "Skipping registration of nil interceptor")
		return
	}
	if name == "" {
		logger.Warn(ctx, "Skipping registration of interceptor with empty name")
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.interceptors[name]; ok {
		logger.Warn(ctx, "Interceptor already registered", log.String("interceptorName", name))
		return
	}
	r.interceptors[name] = ic
}

// GetInterceptor retrieves an interceptor by name.
func (r *interceptorRegistry) GetInterceptor(name string) (core.InterceptorInterface, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ic, ok := r.interceptors[name]
	if !ok {
		return nil, fmt.Errorf("interceptor '%s' not found", name)
	}
	return ic, nil
}

// IsRegistered checks if an interceptor with the given name is registered.
func (r *interceptorRegistry) IsRegistered(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.interceptors[name]
	return ok
}

// ----- Registration -----
// The registry is populated during initialization through registrar functions,
// which encapsulate the logic for creating and registering specific interceptors.
// This design allows for flexible configuration of which interceptors are active in the system,
// while ensuring that all registered interceptors are valid and properly initialized.

// InterceptorDependencies holds the dependencies required for interceptor initialization.
type InterceptorDependencies struct {
	FlowFactory    core.FlowFactoryInterface
	CaptchaService providers.CaptchaValidationProvider
}

// builtinRegistrars maps each built-in interceptor name to its registration function.
var builtinRegistrars = map[string]func(InterceptorDependencies, InterceptorRegistryInterface) error{
	ChallengeTokenInterceptor: registerChallengeTokenInterceptor,
	CaptchaInterceptor:        registerCaptchaInterceptor,
}

// registerInterceptors registers the given interceptors in the registry. If the list is empty,
// all built-in interceptors are registered.
func registerInterceptors(deps InterceptorDependencies, registry InterceptorRegistryInterface,
	interceptorNames []string) error {
	if len(interceptorNames) == 0 {
		interceptorNames = slices.Collect(maps.Keys(builtinRegistrars))
	}

	sanitized, err := sanitizeAndValidate(interceptorNames)
	if err != nil {
		return err
	}

	logger := log.GetLogger()
	for _, name := range sanitized {
		if registry.IsRegistered(name) {
			logger.Debug(context.Background(), "Interceptor already registered; skipping",
				log.String("interceptorName", name))
			continue
		}

		logger.Debug(context.Background(), "Registering interceptor", log.String("interceptorName", name))
		registrar := builtinRegistrars[name]
		if err := registrar(deps, registry); err != nil {
			return fmt.Errorf("failed to register interceptor %s: %w", name, err)
		}
	}
	return nil
}

// sanitizeAndValidate trims whitespace, removes empty entries, deduplicates, and validates that
// each interceptor name has a known built-in registrar.
func sanitizeAndValidate(names []string) ([]string, error) {
	seen := make(map[string]bool, len(names))
	result := make([]string, 0, len(names))

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("empty interceptor name found in configuration")
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate interceptor name in configuration: %s", name)
		}
		seen[name] = true

		if _, ok := builtinRegistrars[name]; !ok {
			return nil, fmt.Errorf("unknown interceptor: %s", name)
		}
		result = append(result, name)
	}

	return result, nil
}

// registerChallengeTokenInterceptor registers the challenge token interceptor in the registry.
func registerChallengeTokenInterceptor(deps InterceptorDependencies, registry InterceptorRegistryInterface) error {
	if deps.FlowFactory == nil {
		return fmt.Errorf("FlowFactory dependency is required for %s", ChallengeTokenInterceptor)
	}
	registry.RegisterInterceptor(ChallengeTokenInterceptor, newChallengeTokenInterceptor(deps.FlowFactory))
	return nil
}

// registerCaptchaInterceptor registers the captcha interceptor in the registry.
//
//nolint:unparam // error return kept for signature consistency with other register* functions
func registerCaptchaInterceptor(deps InterceptorDependencies, registry InterceptorRegistryInterface) error {
	if deps.FlowFactory == nil || deps.CaptchaService == nil {
		log.GetLogger().Debug(context.Background(), "Skipping captcha interceptor registration: missing dependencies",
			log.String("interceptorName", CaptchaInterceptor))
		return nil
	}
	registry.RegisterInterceptor(CaptchaInterceptor, newCaptchaInterceptor(deps.FlowFactory, deps.CaptchaService))
	return nil
}

// ----- Default Interceptors -----

// DefaultInterceptors lists all default (always-enforced) interceptors.
var DefaultInterceptors []core.InterceptorUnitInterface

// DefaultInterceptorNames holds the unique names of all default interceptors for quick lookup.
var DefaultInterceptorNames map[string]struct{}

// DefaultInterceptorsByMode holds default interceptor units pre-grouped by mode.
var DefaultInterceptorsByMode map[providers.InterceptorMode][]core.InterceptorUnitInterface

// initDefaultInterceptorUnits builds the default interceptor execution units using the flow factory
// and groups them by mode for efficient lookup.
func initDefaultInterceptorUnits(factory core.FlowFactoryInterface) {
	defaults := []core.InterceptorUnitInterface{
		factory.CreateInterceptorUnit(
			ChallengeTokenInterceptor, providers.InterceptorModePreRequest, "", nil, nil),
		factory.CreateInterceptorUnit(
			ChallengeTokenInterceptor, providers.InterceptorModePostRequest, "", nil, nil),
	}

	DefaultInterceptors = defaults
	DefaultInterceptorNames = make(map[string]struct{}, len(defaults))
	DefaultInterceptorsByMode = make(map[providers.InterceptorMode][]core.InterceptorUnitInterface)
	for _, d := range defaults {
		DefaultInterceptorNames[d.GetName()] = struct{}{}
		DefaultInterceptorsByMode[d.GetMode()] = append(DefaultInterceptorsByMode[d.GetMode()], d)
	}
}

// GetDefaultInterceptorUnits returns cloned copies of default interceptor units for the given mode.
// Each call creates new instances so concurrent requests do not share mutable state.
func GetDefaultInterceptorUnits(mode providers.InterceptorMode) []core.InterceptorUnitInterface {
	units := DefaultInterceptorsByMode[mode]
	cloned := make([]core.InterceptorUnitInterface, len(units))
	for i, u := range units {
		cloned[i] = u.Clone()
	}
	return cloned
}
