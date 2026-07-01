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

package consent

// Wire-level discriminator values for ConsentPurposePrompt.Type. These are the strings the UI
// reads to choose between attribute and permission rendering.
const (
	consentPromptTypeAttributes  = "attributes"
	consentPromptTypePermissions = "permissions"
)

// consentSessionData holds the consent session state that is signed into a JWT token.
// It captures the purposes and their elements from the resolve step so that the record step
// can verify that the user's decisions match exactly what was prompted.
type consentSessionData struct {
	// Purposes holds the per-purpose element information from the resolve step
	Purposes []consentSessionPurpose `json:"purposes"`
}

// consentSessionPurpose represents a single purpose's elements within the consent session.
type consentSessionPurpose struct {
	// PurposeName is the unique name of the consent purpose
	PurposeName string `json:"purposeName"`
	// Essential holds the names of mandatory elements for this purpose
	Essential []string `json:"essential"`
	// Optional holds the names of optional elements for this purpose
	Optional []string `json:"optional"`
}
