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

package assert

import authncm "github.com/thunder-id/thunderid/internal/authn/common"

// AssuranceLevel represents the level of assurance for authentication.
type AssuranceLevel string

// Level returns the hierarchical level value for comparison purposes.
// AAL and IAL share the same numeric scale (0-3) where higher numbers indicate stronger assurance.
// Level 0 represents unknown or no authentication.
func (al AssuranceLevel) Level() int {
	switch al {
	case AALLevel1, IALLevel1:
		return 1
	case AALLevel2, IALLevel2:
		return 2
	case AALLevel3, IALLevel3:
		return 3
	default:
		return 0
	}
}

// AssuranceContext contains authentication assurance information.
type AssuranceContext struct {
	AAL            AssuranceLevel                   `json:"aal"`
	IAL            AssuranceLevel                   `json:"ial"`
	Authenticators []authncm.AuthenticatorReference `json:"authenticators"`
}

// AssertionResult contains the result of an authentication assertion.
type AssertionResult struct {
	Context *AssuranceContext
}
