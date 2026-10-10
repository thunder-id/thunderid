// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"errors"
	"html"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	translationmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// translationResolver is the narrow slice of the translation service the renderer needs.
type translationResolver interface {
	ResolveTranslationsForKey(ctx context.Context, language, namespace, key string) (
		*translationmgt.TranslationResponse, *tidcommon.ServiceError)
}

// designResolver substitutes {{design(<token>)}} placeholders against a theme's color scheme.
// Satisfied by the design resolve service (internal/design/resolve).
type designResolver interface {
	ResolveDesignContent(ctx context.Context, themeID, colorScheme, content string) (
		string, *tidcommon.ServiceError)
}

// TemplateRendererInterface is the runtime surface consumed by flow executors: it returns fully
// resolved, ready-to-send content and exposes no CRUD.
//
// Escaping: only {{ctx(...)}} values are HTML-escaped (HTML body only), as untrusted caller input;
// translation and design values are inserted verbatim. Context is substituted last.
type TemplateRendererInterface interface {
	Resolve(
		ctx context.Context, channel ChannelType, handle string, in RenderInput,
	) (*ResolvedContent, *tidcommon.ServiceError)
	// SetDesignResolver injects the design resolve service, which is built after this renderer.
	SetDesignResolver(resolver designResolver)
}

// templateRenderer is the default implementation.
type templateRenderer struct {
	store       notificationTemplateStoreInterface
	translation translationResolver
	design      designResolver
	logger      *log.Logger
}

// newTemplateRenderer creates a renderer; the design resolver is injected later via SetDesignResolver.
func newTemplateRenderer(store notificationTemplateStoreInterface,
	translation translationResolver) *templateRenderer {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationTemplateRenderer"))
	return &templateRenderer{store: store, translation: translation, logger: logger}
}

// SetDesignResolver injects the design resolve service once it is available.
func (r *templateRenderer) SetDesignResolver(resolver designResolver) {
	r.design = resolver
}

// Resolve loads the template by handle and produces fully resolved, ready-to-send content for the
// locale, failing closed if any required placeholder cannot be resolved.
func (r *templateRenderer) Resolve(ctx context.Context, channel ChannelType, handle string, in RenderInput) (
	*ResolvedContent, *tidcommon.ServiceError) {
	if svcErr := validateChannel(channel); svcErr != nil {
		return nil, svcErr
	}
	if handle == "" {
		return nil, &ErrorInvalidHandle
	}

	dao, err := r.store.GetTemplateByHandle(ctx, channel, handle)
	if err != nil {
		if errors.Is(err, errTemplateNotFound) {
			return nil, &ErrorTemplateNotFound
		}
		r.logger.Error(ctx, "Failed to load template for rendering", log.String("handle", handle), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return r.resolveContent(ctx, channel, dao, in)
}

// resolveContent renders the subject and body. An email body is HTML and may carry design tokens; the
// subject is plain text; SMS has neither a subject nor design.
func (r *templateRenderer) resolveContent(ctx context.Context, channel ChannelType, dao templateDAO,
	in RenderInput) (*ResolvedContent, *tidcommon.ServiceError) {
	htmlBody := channel == ChannelTypeEmail

	// Shared across the subject and body so a key used in both is resolved only once.
	translations := map[string]string{}

	// The subject is plain text: translations and context only, no design.
	subject, svcErr := r.renderField(ctx, in.Locale, dao.Content.Subject,
		renderOptions{data: in.Data, translations: translations})
	if svcErr != nil {
		return nil, svcErr
	}

	// An HTML body may carry design tokens; an empty scheme lets the design service use the theme's
	// default. A plain-text body (SMS) resolves neither.
	bodyOpts := renderOptions{escapeHTML: htmlBody, data: in.Data, translations: translations}
	if htmlBody {
		var colorScheme string
		if dao.Design != nil {
			colorScheme = dao.Design.ColorScheme
		}
		bodyOpts.design = &designContext{themeID: in.ThemeID, colorScheme: colorScheme}
	}
	body, svcErr := r.renderField(ctx, in.Locale, dao.Content.Body, bodyOpts)
	if svcErr != nil {
		return nil, svcErr
	}

	return &ResolvedContent{Subject: subject, Body: body}, nil
}

// renderField resolves one field's placeholders in order: translations, then design tokens, then
// context values last (so a context value is never reinterpreted). Each step fails closed.
func (r *templateRenderer) renderField(ctx context.Context, locale, template string, opts renderOptions) (
	string, *tidcommon.ServiceError) {
	text, svcErr := r.applyTranslations(ctx, locale, template, opts.translations)
	if svcErr != nil {
		return "", svcErr
	}
	if opts.design != nil {
		text, svcErr = r.applyDesignTokens(ctx, text, opts.design.themeID, opts.design.colorScheme)
		if svcErr != nil {
			return "", svcErr
		}
	}
	// Before substituting context, every {{t}}/{{design}} must be resolved and well-formed; a leftover
	// fails closed. Checking here (not after) avoids misreading a context value's placeholder-like text.
	if ph := unresolvedBeforeContext(text); ph != "" {
		r.logger.Error(ctx, "Unresolved or malformed placeholder before context substitution",
			log.String("placeholder", ph))
		return "", &tidcommon.InternalServerError
	}
	return r.applyContextValues(ctx, text, opts.data, opts.escapeHTML)
}

// applyTranslations substitutes every {{t(key)}} with its localized value. A key with no value fails
// closed; resolved values are memoized across the render.
func (r *templateRenderer) applyTranslations(ctx context.Context, locale, text string,
	cache map[string]string) (string, *tidcommon.ServiceError) {
	if !translationPlaceholderRegex.MatchString(text) {
		return text, nil
	}
	// Defensive: a nil translation resolver fails closed here rather than nil-panicking.
	if r.translation == nil {
		r.logger.Error(ctx, "Translation resolver not configured")
		return "", &tidcommon.InternalServerError
	}
	// Normalize the locale to canonical BCP 47, falling back to the system language when it is empty or
	// invalid, so a non-canonical locale never fails the render.
	if canonical, ok := translationmgt.NormaliseBCP47Tag(locale); ok {
		locale = canonical
	} else {
		locale = translationmgt.SystemLanguage
	}

	var svcErr *tidcommon.ServiceError
	out := translationPlaceholderRegex.ReplaceAllStringFunc(text, func(match string) string {
		if svcErr != nil {
			return match
		}
		key := translationPlaceholderRegex.FindStringSubmatch(match)[1]
		if cached, ok := cache[key]; ok {
			return cached
		}
		resp, tErr := r.translation.ResolveTranslationsForKey(ctx, locale, translationNamespace, key)
		if tErr != nil {
			if tErr.Code == translationmgt.ErrorTranslationNotFound.Code {
				r.logger.Error(ctx, "Template translation key has no value for locale",
					log.String("key", key), log.String("locale", locale))
			} else {
				r.logger.Error(ctx, "Failed to resolve translation key", log.String("key", key),
					log.String("locale", locale), log.String("errorCode", tErr.Code))
			}
			svcErr = &tidcommon.InternalServerError
			return match
		}
		// Per spec, a translated value may embed only {{ctx(...)}}; a {{t}}/{{design}} or malformed
		// placeholder inside it is invalid and fails closed consistently across every field.
		if bad := unresolvedBeforeContext(resp.Value); bad != "" {
			r.logger.Error(ctx, "Translation value embeds a disallowed placeholder (only {{ctx(...)}} is allowed)",
				log.String("key", key), log.String("placeholder", bad))
			svcErr = &tidcommon.InternalServerError
			return match
		}
		cache[key] = resp.Value
		return resp.Value
	})
	if svcErr != nil {
		return "", svcErr
	}
	return out, nil
}

// applyDesignTokens substitutes {{design(<token>)}} via the design resolve service. No-op when the
// field has no design token; fails closed when a token cannot be resolved.
func (r *templateRenderer) applyDesignTokens(ctx context.Context, text, themeID, colorScheme string) (
	string, *tidcommon.ServiceError) {
	if !strings.Contains(text, designPlaceholderPrefix) {
		return text, nil
	}
	if r.design == nil {
		r.logger.Error(ctx, "Cannot resolve design tokens: design resolver not configured")
		return "", &tidcommon.InternalServerError
	}
	if themeID == "" {
		r.logger.Error(ctx, "Cannot resolve design tokens: no theme supplied for the render")
		return "", &tidcommon.InternalServerError
	}

	resolved, svcErr := r.design.ResolveDesignContent(ctx, themeID, colorScheme, text)
	if svcErr != nil {
		r.logger.Error(ctx, "Failed to resolve design tokens", log.String("errorCode", svcErr.Code))
		return "", &tidcommon.InternalServerError
	}
	// The design service leaves an unknown token in place; fail closed rather than ship it.
	if strings.Contains(resolved, designPlaceholderPrefix) {
		r.logger.Error(ctx, "Unresolved design token after substitution")
		return "", &tidcommon.InternalServerError
	}
	return resolved, nil
}

// applyContextValues substitutes {{ctx(<key>)}} with flow-context values. A placeholder with no value
// fails closed.
func (r *templateRenderer) applyContextValues(ctx context.Context, text string, data map[string]string,
	escapeHTML bool) (string, *tidcommon.ServiceError) {
	var missingKey string
	out := contextPlaceholderRegex.ReplaceAllStringFunc(text, func(match string) string {
		key := contextPlaceholderRegex.FindStringSubmatch(match)[1]
		value, ok := data[key]
		if !ok {
			if missingKey == "" {
				missingKey = key
			}
			return match
		}
		if escapeHTML {
			return html.EscapeString(value)
		}
		return value
	})
	if missingKey != "" {
		r.logger.Error(ctx, "Unresolved context placeholder after substitution", log.String("key", missingKey))
		return "", &tidcommon.InternalServerError
	}
	return out, nil
}
