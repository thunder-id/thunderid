// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package notificationtemplate manages the email and SMS notification templates ThunderID sends.
// A template is language-neutral: its subject and body reference translation keys resolved at render
// time. The module is channel-generic; per-channel rules live behind a channelHandler (see channel.go).
package notificationtemplate

// TemplateContent is the language-neutral content of a template, one common shape for every channel.
// Fields that do not apply to a channel are left empty. It is persisted as the CONTENT JSON column.
type TemplateContent struct {
	ContentType string `json:"contentType,omitempty"`
	Subject     string `json:"subject,omitempty"`
	Body        string `json:"body"`
}

// TemplateDesign holds a template's design references (email only). Persisted in the companion
// NOTIFICATION_TEMPLATE_DESIGN table; nil for channels without a design.
type TemplateDesign struct {
	ColorScheme string `json:"colorScheme,omitempty"`
}

// Template is the full representation of a notification template.
type Template struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Self        string          `json:"self"`
	Design      *TemplateDesign `json:"design,omitempty"`
	Content     TemplateContent `json:"content"`
}

// TemplateSummary is the list-item representation of a notification template.
type TemplateSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Self        string `json:"self"`
}

// CreateTemplateRequest is the request body for creating a template. The channel is taken from the
// path, not the body.
type CreateTemplateRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Design      *TemplateDesign `json:"design,omitempty"`
	Content     TemplateContent `json:"content"`
}

// UpdateTemplateRequest is the request body for updating a template.
type UpdateTemplateRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Design      *TemplateDesign `json:"design,omitempty"`
	Content     TemplateContent `json:"content"`
}

// TemplateListResponse is the response for listing templates of a channel.
type TemplateListResponse struct {
	Templates []TemplateSummary `json:"templates"`
}

// templateDAO is the store-level representation of a template: content is stored as a single JSON
// column, and the design (when present) lives in a companion table.
type templateDAO struct {
	ID          string
	Channel     string
	Name        string
	Description string
	Content     TemplateContent
	Design      *TemplateDesign
}
