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

package flowmgt

const (
	// defaultPageSize is the default number of items per page for paginated responses
	defaultPageSize = 30
	// maxPageSize is the maximum number of items per page for paginated responses
	maxPageSize = 100
	// maxAllowedVersionHistory is the maximum number of versions to keep for a flow definition
	maxAllowedVersionHistory = 50
	// defaultVersionHistory is the default number of versions to keep for a flow definition
	defaultVersionHistory = 10
)

const (
	// provisioningNodeID is the node ID for the inferred provisioning node
	provisioningNodeID = "prov_node"
	// userTypeResolverNodeID is the node ID for the inferred user type resolver node
	userTypeResolverNodeID = "ut_res_node"
	// userTypePromptNodeID is the node ID for the inferred user type prompt node
	userTypePromptNodeID = "ut_prompt_node"
	// phoneInputPromptNodeID is the node ID for the inferred phone input prompt node
	phoneInputPromptNodeID = "phone_prompt_node"
	// defaultNodeWidth is the default width for a node layout
	defaultNodeWidth = 100
	// defaultNodeHeight is the default height for a node layout
	defaultNodeHeight = 120
	// defaultNodeXPos is the default X position for a node layout
	defaultNodeXPos = 0
	// defaultNodeYPos is the default Y position for a node layout
	defaultNodeYPos = 0
)

const (
	// nodePropertyKeyIDPID is the node property key that references an identity provider by its ID.
	nodePropertyKeyIDPID = "idpId"
	// nodePropertyKeyNotificationSenderID is the node property key that references a notification
	// sender by its ID.
	nodePropertyKeyNotificationSenderID = "senderId"
)

// authToRegLabelTerms maps authentication UI label terms to their registration equivalents.
// Ordered by specificity (longest/most-specific first) to avoid partial matches.
var authToRegLabelTerms = []struct{ auth, reg string }{
	{"Authentication", "Registration"},
	{"Authenticate", "Register"},
	{"Sign-in", "Sign-up"},
	{"Sign In", "Sign Up"},
	{"Signin", "Signup"},
	{"Log In", "Register"},
	{"Login", "Register"},
}
