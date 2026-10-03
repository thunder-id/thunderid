// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import "github.com/thunder-id/thunderid/internal/system/utils"

// TemplateContent is the language-neutral content of a template, one common shape for every channel.
// Subject and body are template strings that may embed zero or more placeholders resolved at render
// time: {{t(key)}} (localized text), {{ctx(var)}} (flow-context values), and {{design(token)}} (design
// tokens, body only). Fields that do not apply to a channel are left empty. Persisted as the CONTENT
// JSON column.
type TemplateContent struct {
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body"`
}

// TemplateDesign holds a template's design references (email only). Persisted in the DESIGN JSON
// column of NOTIFICATION_TEMPLATE; nil for channels without a design.
type TemplateDesign struct {
	ColorScheme string `json:"colorScheme,omitempty"`
}

// Template is the full representation of a notification template.
type Template struct {
	ID          string          `json:"id"`
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	Design      *TemplateDesign `json:"design,omitempty"`
	Content     TemplateContent `json:"content"`
}

// TemplateSummary is the list-item representation of a notification template.
type TemplateSummary struct {
	ID          string `json:"id"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
	Self        string `json:"self"`
}

// CreateTemplateRequest is the request body for creating a template. The channel is taken from the
// path, not the body. The handle is immutable and unique per channel.
type CreateTemplateRequest struct {
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description"`
	Design      *TemplateDesign `json:"design,omitempty"`
	Content     TemplateContent `json:"content"`
}

// UpdateTemplateRequest is the request body for updating a template. The handle is immutable and is
// not part of the body; the template is identified by the id in the path.
type UpdateTemplateRequest struct {
	DisplayName string          `json:"displayName"`
	Description string          `json:"description"`
	Design      *TemplateDesign `json:"design,omitempty"`
	Content     TemplateContent `json:"content"`
}

// TemplateListResponse is the paginated response for listing templates of a channel.
type TemplateListResponse struct {
	TotalResults int               `json:"totalResults"`
	StartIndex   int               `json:"startIndex"`
	Count        int               `json:"count"`
	Templates    []TemplateSummary `json:"templates"`
	Links        []utils.Link      `json:"links"`
}

// templateDAO is the store-level representation of a template: content is stored as a single JSON
// column (CONTENT), and the design (when present) as a single JSON column (DESIGN).
type templateDAO struct {
	ID          string
	Channel     ChannelType
	Handle      string
	DisplayName string
	Description string
	Content     TemplateContent
	Design      *TemplateDesign
}
