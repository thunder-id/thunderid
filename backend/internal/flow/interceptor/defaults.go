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
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
)

// DefaultInterceptors lists all default (always-enforced) interceptors.
// These declarations are automatically injected into every flow instance.
var DefaultInterceptors []core.InterceptorUnitInterface

// DefaultInterceptorNames holds the unique names of all default interceptors for quick lookup.
var DefaultInterceptorNames map[string]struct{}

// initDefaultInterceptorExecutionUnits builds the default interceptor execution units using the flow factory.
func initDefaultInterceptorExecutionUnits(factory core.FlowFactoryInterface) {
	DefaultInterceptors = []core.InterceptorUnitInterface{
		factory.CreateInterceptorUnit(
			ChallengeTokenInterceptor, common.InterceptorModePreRequest, "", nil, nil),
		factory.CreateInterceptorUnit(
			ChallengeTokenInterceptor, common.InterceptorModePostRequest, "", nil, nil),
	}

	DefaultInterceptorNames = make(map[string]struct{}, len(DefaultInterceptors))
	for _, d := range DefaultInterceptors {
		DefaultInterceptorNames[d.GetName()] = struct{}{}
	}
}
