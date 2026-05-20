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

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// flowInferenceServiceInterface defines the interface for flow inference services
type flowInferenceServiceInterface interface {
	InferRegistrationFlow(authFlow *FlowDefinition) (*FlowDefinition, error)
}

// flowInferenceService implements FlowInferenceServiceInterface
type flowInferenceService struct {
	logger *log.Logger
}

// newFlowInferenceService creates a new flow inference service instance
func newFlowInferenceService() flowInferenceServiceInterface {
	return &flowInferenceService{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowInferenceService")),
	}
}

// InferRegistrationFlow creates a registration flow definition from an authentication flow
func (s *flowInferenceService) InferRegistrationFlow(authFlow *FlowDefinition) (*FlowDefinition, error) {
	s.logger.Debug("Inferring registration flow from authentication flow",
		log.String("authFlowName", authFlow.Name))

	regFlowName := s.generateRegistrationFlowName(authFlow.Name)
	hasLayout := s.hasLayoutInformation(authFlow.Nodes)

	// Deep copy nodes to avoid modifying the original flow
	regNodes, err := s.cloneNodes(authFlow.Nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to clone nodes: %w", err)
	}

	s.cleanAuthenticationProperties(regNodes)

	if !s.hasProvisioningNode(regNodes) {
		if err := s.insertProvisioningNode(&regNodes, hasLayout); err != nil {
			return nil, err
		}
		s.logger.Debug("Inserted provisioning node into registration flow")
	} else {
		s.logger.Debug("Provisioning node already exists, skipping insertion")
	}

	startNodeID, err := s.findStartNode(regNodes)
	if err != nil {
		return nil, err
	}

	if !s.hasUserTypeResolverNode(regNodes) {
		userTypePromptNode := s.createUserTypePromptNode(userTypeResolverNodeID, hasLayout)
		userTypeResolverNode := s.createUserTypeResolverNode(userTypePromptNodeID, hasLayout)

		// Insert resolver after START
		if err := s.insertNodeAfterStart(&regNodes, userTypeResolverNode, startNodeID); err != nil {
			return nil, err
		}

		// Append the prompt node to the flow
		regNodes = append(regNodes, userTypePromptNode)

		s.logger.Debug("Inserted user type resolver node with prompt into registration flow")
	} else {
		s.logger.Debug("User type resolver node already exists, skipping insertion")
	}

	// Insert phone input prompt if SMS OTP send nodes are present
	s.insertPhoneInputPromptIfNeeded(&regNodes, hasLayout)

	return &FlowDefinition{
		Name:     regFlowName,
		FlowType: common.FlowTypeRegistration,
		Handle:   authFlow.Handle,
		Nodes:    regNodes,
	}, nil
}

// generateRegistrationFlowName generates a registration flow name from an auth flow name.
func (s *flowInferenceService) generateRegistrationFlowName(authFlowName string) string {
	// List of authentication-related terms to replace (ordered by specificity)
	authTerms := []string{"Authentication", "Authenticate", "Sign-in", "Signin", "Sign in", "Login", "Auth"}
	regTerm := "Registration"

	// Try to replace any authentication term with "Registration" (case-insensitive)
	for _, term := range authTerms {
		lowerName := strings.ToLower(authFlowName)
		lowerTerm := strings.ToLower(term)

		if index := strings.Index(lowerName, lowerTerm); index != -1 {
			// Replace preserving the structure of the original name
			return authFlowName[:index] + regTerm + authFlowName[index+len(term):]
		}
	}

	// If no auth term found, append suffix
	return authFlowName + " - Registration"
}

// cloneNodes creates a deep copy of the nodes array
func (s *flowInferenceService) cloneNodes(nodes []NodeDefinition) ([]NodeDefinition, error) {
	// Use JSON marshaling for deep copy
	data, err := json.Marshal(nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal nodes: %w", err)
	}

	var clonedNodes []NodeDefinition
	if err := json.Unmarshal(data, &clonedNodes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nodes: %w", err)
	}

	return clonedNodes, nil
}

// cleanAuthenticationProperties removes authentication-specific properties from nodes
// and sets appropriate registration-specific defaults
func (s *flowInferenceService) cleanAuthenticationProperties(nodes []NodeDefinition) {
	for i := range nodes {
		node := &nodes[i]

		if node.Properties != nil {
			// Remove authentication-specific properties that don't apply to registration
			delete(node.Properties, common.NodePropertyAllowAuthenticationWithoutLocalUser)
		}

		// Clean up PROMPT node meta components for registration context
		if node.Type == string(common.NodeTypePrompt) {
			s.cleanPromptNodeMeta(node)
		}
	}
}

// replaceAuthLabel checks if label contains an auth-related term and returns the registration
// equivalent. Returns (newLabel, true) if replaced, ("", false) if no match found.
func replaceAuthLabel(label string) (string, bool) {
	lowerLabel := strings.ToLower(label)
	for _, pair := range authToRegLabelTerms {
		lowerTerm := strings.ToLower(pair.auth)
		if index := strings.Index(lowerLabel, lowerTerm); index != -1 {
			return label[:index] + pair.reg + label[index+len(pair.auth):], true
		}
	}
	return "", false
}

// cleanPromptNodeMeta removes auth-specific UI components from a PROMPT node's meta
// and updates labels to be appropriate for a registration flow.
func (s *flowInferenceService) cleanPromptNodeMeta(node *NodeDefinition) {
	meta, ok := node.Meta.(map[string]interface{})
	if !ok {
		return
	}

	components, ok := meta["components"].([]interface{})
	if !ok {
		return
	}

	for i, comp := range components {
		c, ok := comp.(map[string]interface{})
		if !ok {
			continue
		}

		// Update auth heading labels to registration equivalents — match by type, not ID (IDs may be random)
		if c["type"] == "TEXT" {
			if label, ok := c["label"].(string); ok {
				if newLabel, replaced := replaceAuthLabel(label); replaced {
					c["label"] = newLabel
					components[i] = c
				}
			}
		}

		// Remove sign-up link from inside BLOCK components — match by type and label content, not ID
		if c["type"] == "BLOCK" {
			blockComponents, ok := c["components"].([]interface{})
			if !ok {
				continue
			}
			filtered := make([]interface{}, 0, len(blockComponents))
			for _, bc := range blockComponents {
				bcMap, ok := bc.(map[string]interface{})
				if !ok {
					filtered = append(filtered, bc)
					continue
				}
				if bcMap["type"] == "RICH_TEXT" {
					if label, ok := bcMap["label"].(string); ok &&
						(strings.Contains(label, "sign_up_url") || strings.Contains(label, "forgot_password_url")) {
						continue
					}
				}
				// Rename auth submit action button labels to registration equivalents
				if bcMap["type"] == "ACTION" && bcMap["eventType"] == "SUBMIT" {
					if label, ok := bcMap["label"].(string); ok {
						if newLabel, replaced := replaceAuthLabel(label); replaced {
							bcMap["label"] = newLabel
						}
					}
				}
				filtered = append(filtered, bc)
			}
			c["components"] = filtered
			components[i] = c
		}
	}

	meta["components"] = components
}

// findStartNode finds the START node in the flow
func (s *flowInferenceService) findStartNode(nodes []NodeDefinition) (string, error) {
	for _, node := range nodes {
		if node.Type == string(common.NodeTypeStart) {
			return node.ID, nil
		}
	}
	return "", fmt.Errorf("no START node found in flow")
}

// findEndNode finds the END node in the flow
func (s *flowInferenceService) findEndNode(nodes []NodeDefinition) (string, error) {
	for _, node := range nodes {
		if node.Type == string(common.NodeTypeEnd) {
			return node.ID, nil
		}
	}
	return "", fmt.Errorf("no END node found in flow")
}

// hasLayoutInformation checks if any node in the flow has layout information
func (s *flowInferenceService) hasLayoutInformation(nodes []NodeDefinition) bool {
	for _, node := range nodes {
		if node.Layout != nil && (node.Layout.Size != nil || node.Layout.Position != nil) {
			return true
		}
	}
	return false
}

// addDefaultLayout adds default layout information to a node
func (s *flowInferenceService) addDefaultLayout(node *NodeDefinition) {
	node.Layout = &NodeLayout{
		Size: &NodeSize{
			Width:  defaultNodeWidth,
			Height: defaultNodeHeight,
		},
		Position: &NodePosition{
			X: defaultNodeXPos,
			Y: defaultNodeYPos,
		},
	}
}

// findAuthAssertNode finds the AuthAssertExecutor node in the flow and returns its ID
func (s *flowInferenceService) findAuthAssertNode(nodes []NodeDefinition) (string, bool) {
	for _, node := range nodes {
		if node.Executor != nil && node.Executor.Name == executor.ExecutorNameAuthAssert {
			return node.ID, true
		}
	}
	return "", false
}

// hasProvisioningNode checks if a provisioning node already exists in the flow
func (s *flowInferenceService) hasProvisioningNode(nodes []NodeDefinition) bool {
	for _, node := range nodes {
		if node.Executor != nil && node.Executor.Name == executor.ExecutorNameProvisioning {
			return true
		}
	}
	return false
}

// insertProvisioningNode inserts the provisioning node before AuthAssertExecutor if it exists,
// otherwise before the END node
func (s *flowInferenceService) insertProvisioningNode(nodes *[]NodeDefinition, includeLayout bool) error {
	authAssertNodeID, hasAuthAssert := s.findAuthAssertNode(*nodes)

	var targetNodeID string
	if hasAuthAssert {
		targetNodeID = authAssertNodeID
		s.logger.Debug("Found AuthAssertExecutor, inserting provisioning node before it")
	} else {
		endNodeID, err := s.findEndNode(*nodes)
		if err != nil {
			return err
		}
		targetNodeID = endNodeID
		s.logger.Debug("No AuthAssertExecutor found, inserting provisioning node before END")
	}

	provisioningNode := s.createProvisioningNode(targetNodeID, includeLayout)

	return s.insertNodeBefore(nodes, provisioningNode, targetNodeID)
}

// createProvisioningNode creates a TASK_EXECUTION node with ProvisioningExecutor
func (s *flowInferenceService) createProvisioningNode(nextNodeID string, includeLayout bool) NodeDefinition {
	node := NodeDefinition{
		ID:   provisioningNodeID,
		Type: string(common.NodeTypeTaskExecution),
		Executor: &ExecutorDefinition{
			Name: executor.ExecutorNameProvisioning,
		},
		OnSuccess: nextNodeID,
	}

	if includeLayout {
		s.addDefaultLayout(&node)
	}

	return node
}

// hasUserTypeResolverNode checks if a user type resolver node already exists in the flow
func (s *flowInferenceService) hasUserTypeResolverNode(nodes []NodeDefinition) bool {
	for _, node := range nodes {
		if node.Executor != nil && node.Executor.Name == executor.ExecutorNameUserTypeResolver {
			return true
		}
	}
	return false
}

// createUserTypeResolverNode creates a TASK_EXECUTION node with UserTypeResolverExecutor
func (s *flowInferenceService) createUserTypeResolverNode(promptNodeID string, includeLayout bool) NodeDefinition {
	node := NodeDefinition{
		ID:   userTypeResolverNodeID,
		Type: string(common.NodeTypeTaskExecution),
		Executor: &ExecutorDefinition{
			Name: executor.ExecutorNameUserTypeResolver,
		},
		OnIncomplete: promptNodeID,
	}

	if includeLayout {
		s.addDefaultLayout(&node)
	}

	return node
}

// createUserTypePromptNode creates a PROMPT node for collecting user type selection
func (s *flowInferenceService) createUserTypePromptNode(nextNodeID string, includeLayout bool) NodeDefinition {
	node := NodeDefinition{
		ID:   userTypePromptNodeID,
		Type: string(common.NodeTypePrompt),
		Meta: map[string]interface{}{
			"components": []map[string]interface{}{
				{
					"type":    "TEXT",
					"id":      "heading_usertype",
					"label":   "{{ t(signup:heading) }}",
					"variant": "HEADING_2",
				},
				{
					"type": "BLOCK",
					"id":   "block_usertype",
					"components": []map[string]interface{}{
						{
							"type":        "SELECT",
							"id":          "usertype_input",
							"ref":         "userType",
							"label":       "{{ t(elements:fields.usertype.label) }}",
							"placeholder": "{{ t(elements:fields.usertype.placeholder) }}",
							"required":    true,
							"options":     []interface{}{},
						},
						{
							"type":      "ACTION",
							"id":        "action_usertype",
							"label":     "{{ t(elements:buttons.submit.text) }}",
							"variant":   "PRIMARY",
							"eventType": "SUBMIT",
						},
					},
				},
			},
		},
		Prompts: []PromptDefinition{
			{
				Inputs: []InputDefinition{
					{
						Ref:        "usertype_input",
						Identifier: "userType",
						Type:       "SELECT",
						Required:   true,
					},
				},
				Action: &ActionDefinition{
					Ref:      "action_usertype",
					NextNode: nextNodeID,
				},
			},
		},
	}

	if includeLayout {
		s.addDefaultLayout(&node)
	}

	return node
}

// createInputPromptNode creates a generic PROMPT node for collecting a specific input
func (s *flowInferenceService) createInputPromptNode(nodeID string, input common.Input,
	nextNodeID string, includeLayout bool) NodeDefinition {
	// Determine the component type based on input type
	componentType := input.Type
	if componentType == "" {
		componentType = common.InputTypeText
	}

	// Create label from identifier by capitalizing the first letter
	label := input.Identifier
	if len(label) > 0 {
		label = strings.ToUpper(label[:1]) + label[1:]
	}

	// Create placeholder by converting label to lowercase
	placeholder := "Enter your " + strings.ToLower(label)

	node := NodeDefinition{
		ID:   nodeID,
		Type: string(common.NodeTypePrompt),
		Meta: map[string]interface{}{
			"components": []map[string]interface{}{
				{
					"type":    "TEXT",
					"id":      "heading_" + input.Identifier,
					"label":   "{{ t(signup:heading) }}",
					"variant": "HEADING_2",
				},
				{
					"type": "BLOCK",
					"id":   "block_" + input.Identifier,
					"components": []map[string]interface{}{
						{
							"type":        componentType,
							"id":          input.Ref,
							"ref":         input.Identifier,
							"label":       label,
							"placeholder": placeholder,
							"required":    input.Required,
						},
						{
							"type":      "ACTION",
							"id":        "action_" + input.Identifier,
							"label":     "{{ t(elements:buttons.submit.text) }}",
							"variant":   "PRIMARY",
							"eventType": "SUBMIT",
						},
					},
				},
			},
		},
		Prompts: []PromptDefinition{
			{
				Inputs: []InputDefinition{
					{
						Ref:        input.Ref,
						Identifier: input.Identifier,
						Type:       componentType,
						Required:   input.Required,
					},
				},
				Action: &ActionDefinition{
					Ref:      "action_" + input.Identifier,
					NextNode: nextNodeID,
				},
			},
		},
	}

	if includeLayout {
		s.addDefaultLayout(&node)
	}

	return node
}

// insertPhoneInputPromptIfNeeded scans all nodes to determine if an SMS OTP send node exists
// and whether a PHONE_INPUT is already collected by any prompt node in the flow. If SMS OTP send
// is present but no PHONE_INPUT prompt exists, it inserts a phone input prompt before the SMS send node.
func (s *flowInferenceService) insertPhoneInputPromptIfNeeded(nodes *[]NodeDefinition, includeLayout bool) {
	var smsSendNodeID string
	var phoneInput *common.Input
	hasPhoneInputPrompt := false

	// Scan all the existing nodes
	for _, node := range *nodes {
		// Check for SMS OTP send node and capture phone input from executor inputs if defined
		if node.Executor != nil &&
			node.Executor.Name == executor.ExecutorNameSMSAuth &&
			node.Executor.Mode == executor.ExecutorModeSend &&
			smsSendNodeID == "" {
			smsSendNodeID = node.ID
			for _, input := range node.Executor.Inputs {
				if input.Type == common.InputTypePhone {
					phoneInput = &common.Input{
						Ref:        input.Ref,
						Identifier: input.Identifier,
						Type:       common.InputTypePhone,
						Required:   input.Required,
					}
					break
				}
			}
		}

		// Check if any prompt node collects a PHONE_INPUT
		if node.Type == string(common.NodeTypePrompt) {
			for _, prompt := range node.Prompts {
				for _, input := range prompt.Inputs {
					if input.Type == common.InputTypePhone {
						hasPhoneInputPrompt = true
						break
					}
				}
				if hasPhoneInputPrompt {
					break
				}
			}
		}
	}

	// If no SMS OTP send node found, nothing to do
	if smsSendNodeID == "" {
		return
	}

	// If phone input already collected by an existing prompt, no need to insert another prompt
	if hasPhoneInputPrompt {
		s.logger.Debug("Phone input already collected in the flow, skipping insertion")
		return
	}

	if phoneInput == nil {
		phoneInput = &common.Input{
			Ref:        "mobile_number_input",
			Identifier: common.AttributeMobileNumber,
			Type:       common.InputTypePhone,
			Required:   true,
		}
	}
	phonePromptNode := s.createInputPromptNode(
		phoneInputPromptNodeID,
		*phoneInput,
		smsSendNodeID,
		includeLayout,
	)

	err := s.insertNodeBefore(nodes, phonePromptNode, smsSendNodeID)
	if err != nil {
		s.logger.Warn("Failed to insert phone input prompt before SMS send node",
			log.String("nodeID", smsSendNodeID), log.Error(err))
		return
	}

	s.logger.Debug("Inserted phone input prompt before SMS send node",
		log.String("smsNodeID", smsSendNodeID))
}

// insertNodeBefore inserts a node before the target node by updating all nodes that point to the target
func (s *flowInferenceService) insertNodeBefore(nodes *[]NodeDefinition,
	newNode NodeDefinition, targetNodeID string) error {
	modified := false
	for i := range *nodes {
		node := &(*nodes)[i]

		// Update onSuccess if it points to target
		if node.OnSuccess != "" && node.OnSuccess == targetNodeID {
			node.OnSuccess = newNode.ID
			modified = true
		}

		// Update onFailure if it points to target
		if node.OnFailure != "" && node.OnFailure == targetNodeID {
			node.OnFailure = newNode.ID
			modified = true
		}

		// Update prompts that have actions pointing to target
		for j := range node.Prompts {
			if node.Prompts[j].Action != nil && node.Prompts[j].Action.NextNode == targetNodeID {
				node.Prompts[j].Action.NextNode = newNode.ID
				modified = true
			}
		}
	}

	if !modified {
		return fmt.Errorf("no nodes pointing to target node %s found", targetNodeID)
	}

	// Append the new node to the array
	*nodes = append(*nodes, newNode)
	return nil
}

// insertNodeAfterStart inserts a node after the START node
func (s *flowInferenceService) insertNodeAfterStart(nodes *[]NodeDefinition,
	newNode NodeDefinition, startNodeID string) error {
	// Append the new node to the array first
	*nodes = append(*nodes, newNode)

	// Find START node and get its original next node
	var originalNext string
	for i := range *nodes {
		if (*nodes)[i].ID == startNodeID {
			if (*nodes)[i].OnSuccess == "" {
				return fmt.Errorf("START node has no onSuccess defined")
			}
			originalNext = (*nodes)[i].OnSuccess
			(*nodes)[i].OnSuccess = newNode.ID
			break
		}
	}

	if originalNext == "" {
		return fmt.Errorf("START node not found")
	}

	// Find the newly appended node and update its onSuccess
	for i := range *nodes {
		if (*nodes)[i].ID == newNode.ID {
			(*nodes)[i].OnSuccess = originalNext
			return nil
		}
	}

	return fmt.Errorf("new node %s not found in array", newNode.ID)
}
