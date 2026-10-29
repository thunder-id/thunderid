// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// localeInterceptor updates the flow's effective UI language on PRE_REQUEST from an incoming locale:
// a flow-collected locale (e.g. an invite form's field) wins over the client's UI language. A valid,
// supported value updates it in shared runtime data; otherwise the stored value is left unchanged.
type localeInterceptor struct {
	core.InterceptorInterface
	i18nService providers.I18nProvider
	logger      *log.Logger
}

var _ core.InterceptorInterface = (*localeInterceptor)(nil)

// localeInterceptorInputs declares the reserved current-locale input so the engine clears it once
// consumed. Routed in by the handler; not a client field.
var localeInterceptorInputs = []providers.Input{
	{
		Identifier: common.InputKeyCurrentLocale,
		Type:       providers.InputTypeHidden,
		OneTimeUse: true,
	},
}

// newLocaleInterceptor creates a new locale interceptor.
func newLocaleInterceptor(flowFactory core.FlowFactoryInterface,
	i18nService providers.I18nProvider) *localeInterceptor {
	base := flowFactory.CreateInterceptor(LocaleInterceptor, true, PriorityDefault)

	return &localeInterceptor{
		InterceptorInterface: base,
		i18nService:          i18nService,
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, LocaleInterceptor)),
	}
}

// GetInputs returns the inputs consumed by the locale interceptor.
func (l *localeInterceptor) GetInputs() []providers.Input {
	return localeInterceptorInputs
}

// Execute updates the locale on PRE_REQUEST and leaves other modes untouched.
func (l *localeInterceptor) Execute(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
	if ctx.Mode != providers.InterceptorModePreRequest {
		return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil
	}

	// Sources in priority order: a flow-collected locale (e.g. an invite form's recipient language)
	// over the client's UI language. A valid, supported value updates the stored locale; anything
	// else leaves it unchanged, so a step that carries no fresh locale keeps what was stored.
	form, _ := ctx.ConsumeInput(common.InputKeyFormLocale)
	current, _ := ctx.ConsumeInput(common.InputKeyCurrentLocale)
	resolved := l.firstSupported(ctx, []string{form, current})
	if resolved == "" {
		return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil
	}

	return &common.InterceptorResponse{
		Status:               common.InterceptorStatusComplete,
		SharedRuntimeOutputs: map[string]string{common.RuntimeKeyEffectiveLocale: resolved},
	}, nil
}

// firstSupported returns the first candidate that canonicalises to a supported BCP 47 tag, else "".
func (l *localeInterceptor) firstSupported(ctx *core.InterceptorContext, candidates []string) string {
	supported, svcErr := l.i18nService.ListLanguages(ctx.Context)
	if svcErr != nil {
		l.logger.Debug(ctx.Context, "Failed to list supported languages; leaving locale unresolved",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
		return ""
	}

	supportedSet := make(map[string]struct{}, len(supported))
	for _, s := range supported {
		supportedSet[s] = struct{}{}
	}

	for _, tag := range candidates {
		if tag == "" {
			continue
		}
		canonical, ok := i18nmgt.NormaliseBCP47Tag(tag)
		if !ok {
			continue
		}
		if _, isSupported := supportedSet[canonical]; isSupported {
			return canonical
		}
	}
	return ""
}
