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

package session

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// Initialize builds the SSO session Service. Store construction stays inside this package: callers
// receive only the Service and never hold a store. Timeouts fall back per field to the built-in
// defaults so an unset (zero) value never makes sessions expire immediately.
func Initialize(dbProvider provider.DBProviderInterface, deploymentID string,
	timeouts Timeouts, criteriaRevoker CriteriaRevoker) (Service, error) {
	transactioner, err := dbProvider.GetRuntimePersistentDBTransactioner()
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime persistent DB transactioner for the SSO session service: %w", err)
	}

	def := DefaultTimeouts()
	if timeouts.Idle <= 0 {
		timeouts.Idle = def.Idle
	}
	if timeouts.Absolute <= 0 {
		timeouts.Absolute = def.Absolute
	}

	store := newStore(dbProvider, deploymentID)
	return &service{
		store:           store,
		resolver:        newResolver(store),
		transactioner:   transactioner,
		criteriaRevoker: criteriaRevoker,
		timeouts:        timeouts,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SSOSessionService")),
	}, nil
}
