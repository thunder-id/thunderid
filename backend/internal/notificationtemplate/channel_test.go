// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandlerFor(t *testing.T) {
	t.Run("email", func(t *testing.T) {
		h, err := handlerFor(ChannelEmail)
		require.Nil(t, err)
		require.IsType(t, emailHandler{}, h)
	})
	t.Run("sms", func(t *testing.T) {
		h, err := handlerFor(ChannelSMS)
		require.Nil(t, err)
		require.IsType(t, smsHandler{}, h)
	})
	t.Run("unknown", func(t *testing.T) {
		h, err := handlerFor("push")
		require.Nil(t, h)
		require.NotNil(t, err)
		require.Equal(t, ErrorInvalidChannel.Code, err.Code)
	})
}

func TestValidateChannel(t *testing.T) {
	require.Nil(t, validateChannel(ChannelEmail))
	require.Nil(t, validateChannel(ChannelSMS))
	require.NotNil(t, validateChannel(""))
	require.Equal(t, ErrorInvalidChannel.Code, validateChannel("carrier-pigeon").Code)
}

func TestEmailHandlerValidate(t *testing.T) {
	h := emailHandler{}

	require.Nil(t, h.validate(TemplateContent{Body: "b"}, nil))
	require.Nil(t, h.validate(TemplateContent{Body: "b"}, &TemplateDesign{ColorScheme: ColorSchemeDark}))

	// Color scheme is the only design value validated.
	require.Equal(t, ErrorInvalidColorScheme.Code,
		h.validate(TemplateContent{Body: "b"}, &TemplateDesign{ColorScheme: "teal"}).Code)
}

func TestEmailHandlerNormalize(t *testing.T) {
	h := emailHandler{}

	// Content type is always forced to HTML, even if the request said otherwise.
	content, design := h.normalize(TemplateContent{ContentType: ContentTypePlain, Body: "b"}, nil)
	require.Equal(t, ContentTypeHTML, content.ContentType)
	require.Nil(t, design)

	// A design with no color scheme is treated as absent (default applied later at resolution).
	_, design = h.normalize(TemplateContent{Body: "b"}, &TemplateDesign{})
	require.Nil(t, design)

	// A real color scheme is preserved and the subject is kept for email.
	content, design = h.normalize(
		TemplateContent{Subject: "s", Body: "b"}, &TemplateDesign{ColorScheme: ColorSchemeLight})
	require.Equal(t, ContentTypeHTML, content.ContentType)
	require.Equal(t, "s", content.Subject)
	require.NotNil(t, design)
	require.Equal(t, ColorSchemeLight, design.ColorScheme)
}

func TestSMSHandlerStrict(t *testing.T) {
	h := smsHandler{}

	// A plain-text body only is valid.
	require.Nil(t, h.validate(TemplateContent{Body: "b"}, nil))

	// A subject or a design is rejected, not silently dropped.
	require.Equal(t, ErrorSubjectNotAllowed.Code,
		h.validate(TemplateContent{Subject: "s", Body: "b"}, nil).Code)
	require.Equal(t, ErrorDesignNotAllowed.Code,
		h.validate(TemplateContent{Body: "b"}, &TemplateDesign{ColorScheme: ColorSchemeDark}).Code)

	// Normalize forces plain text.
	content, design := h.normalize(TemplateContent{Body: "b"}, nil)
	require.Equal(t, ContentTypePlain, content.ContentType)
	require.Empty(t, content.Subject)
	require.Equal(t, "b", content.Body)
	require.Nil(t, design)
}
