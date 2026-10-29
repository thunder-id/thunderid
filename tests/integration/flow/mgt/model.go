// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mgt

type FlowDefinition struct {
	Name     string           `json:"name"`
	Handle   string           `json:"handle,omitempty"`
	FlowType string           `json:"flowType"`
	Nodes    []NodeDefinition `json:"nodes"`
	OUID     string           `json:"ouId,omitempty"`
}

type CompleteFlowDefinition struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Handle        string           `json:"handle"`
	FlowType      string           `json:"flowType"`
	ActiveVersion int              `json:"activeVersion"`
	Nodes         []NodeDefinition `json:"nodes"`
	OUID          string           `json:"ouId,omitempty"`
	CreatedAt     string           `json:"createdAt"`
	UpdatedAt     string           `json:"updatedAt"`
}

type BasicFlowDefinition struct {
	OUID          string `json:"ouId,omitempty"`
	ID            string `json:"id"`
	FlowType      string `json:"flowType"`
	Name          string `json:"name"`
	Handle        string `json:"handle"`
	ActiveVersion int    `json:"activeVersion"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type FlowListResponse struct {
	TotalResults int                   `json:"totalResults"`
	StartIndex   int                   `json:"startIndex"`
	Count        int                   `json:"count"`
	Flows        []BasicFlowDefinition `json:"flows"`
	Links        []Link                `json:"links"`
}

type FlowVersion struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Handle    string           `json:"handle"`
	FlowType  string           `json:"flowType"`
	Version   int              `json:"version"`
	IsActive  bool             `json:"isActive"`
	Nodes     []NodeDefinition `json:"nodes"`
	CreatedAt string           `json:"createdAt"`
}

type FlowVersionListResponse struct {
	TotalVersions int                `json:"totalVersions"`
	Versions      []BasicFlowVersion `json:"versions"`
}

type BasicFlowVersion struct {
	Version   int    `json:"version"`
	CreatedAt string `json:"createdAt"`
	IsActive  bool   `json:"isActive"`
}

type RestoreVersionRequest struct {
	Version int `json:"version"`
}

type Link struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}

type NodeLayout struct {
	Size     *NodeSize     `json:"size,omitempty"`
	Position *NodePosition `json:"position,omitempty"`
}

type NodeSize struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type NodePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type NodeDefinition struct {
	ID         string                    `json:"id"`
	Type       string                    `json:"type"`
	Layout     *NodeLayout               `json:"layout,omitempty"`
	Meta       interface{}               `json:"meta,omitempty"`
	Inputs     []InputDefinition         `json:"inputs,omitempty"`
	Actions    []ActionDefinition        `json:"actions,omitempty"`
	Properties map[string]interface{}    `json:"properties,omitempty"`
	Executor   *ExecutorDefinition       `json:"executor,omitempty"`
	Flow       *FlowReferenceDefinition  `json:"flow,omitempty"`
	OnSuccess  string                    `json:"onSuccess,omitempty"`
	OnFailure  string                    `json:"onFailure,omitempty"`
	OnSkip     string                    `json:"onSkip,omitempty"`
	Condition  *ConditionDefinition      `json:"condition,omitempty"`
}

type FlowReferenceDefinition struct {
	Ref string `json:"ref"`
}

type InputDefinition struct {
	Ref        string `json:"ref,omitempty"`
	Type       string `json:"type"`
	Identifier string `json:"identifier"`
	Required   bool   `json:"required"`
}

type ActionDefinition struct {
	Ref      string `json:"ref"`
	NextNode string `json:"nextNode"`
}

type ExecutorDefinition struct {
	Name   string            `json:"name"`
	Inputs []InputDefinition `json:"inputs,omitempty"`
}

type ConditionDefinition struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	OnSkip string `json:"onSkip"`
}

type ErrorResponse struct {
	Type             string `json:"type"`
	Code             string `json:"code"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}
