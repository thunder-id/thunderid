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
	Subject string `json:"subject,omitempty" yaml:"subject,omitempty"`
	Body    string `json:"body" yaml:"body"`
}

// TemplateDesign holds a template's design references (email only). Persisted in the DESIGN JSON
// column of NOTIFICATION_TEMPLATE; nil for channels without a design.
type TemplateDesign struct {
	ColorScheme string `json:"colorScheme,omitempty" yaml:"colorScheme,omitempty"`
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

// RenderInput carries the per-send inputs the renderer needs to turn a stored template into
// ready-to-send content: the recipient locale for translation, the runtime values substituted
// into {{ctx(...)}} placeholders, and the theme the {{design(...)}} tokens resolve against.
type RenderInput struct {
	// Locale is the recipient's language tag (e.g. "en-US"); empty falls back to the system language.
	Locale string
	// Data holds the runtime values substituted into {{ctx(<key>)}} placeholders.
	Data map[string]string
	// ThemeID is the theme whose color scheme resolves {{design(<token>)}} placeholders (email body
	// only). Empty means no theme is available, so a template that embeds a design token fails closed.
	ThemeID string
}

// ResolvedContent is the fully-resolved, ready-to-send output of a render: every placeholder is
// substituted. Subject is empty for channels that have none (e.g. SMS).
type ResolvedContent struct {
	Subject string
	Body    string
}

// renderOptions carries what renderField needs for one field: whether to HTML-escape context values,
// the optional design context (nil resolves no design), the context values (data), and the per-render
// translation memo (translations), shared across the subject and body so a repeated key resolves once.
type renderOptions struct {
	escapeHTML   bool
	design       *designContext
	data         map[string]string
	translations map[string]string
}

// designContext is the theme and color scheme {{design(...)}} tokens resolve against. It is set only
// for a field that may carry design tokens (an email body); a nil designContext resolves no design.
type designContext struct {
	themeID     string
	colorScheme string
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
