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

package enginebridge

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type i18nBridge struct {
	provider thunderidengine.I18nProvider
}

func newI18nBridge(provider thunderidengine.I18nProvider) *i18nBridge {
	return &i18nBridge{provider: provider}
}

func (b *i18nBridge) ListLanguages() ([]string, *serviceerror.ServiceError) {
	if b.provider == nil {
		return nil, &serviceerror.InternalServerError
	}
	langs, err := b.provider.ListLanguages(context.Background())
	if err != nil {
		return nil, &serviceerror.InternalServerError
	}
	return langs, nil
}

func (b *i18nBridge) ResolveTranslations(
	language string, namespace string,
) (*i18nmgt.LanguageTranslationsResponse, *serviceerror.ServiceError) {
	if b.provider == nil {
		return nil, &serviceerror.InternalServerError
	}
	resp, err := b.provider.ResolveTranslations(context.Background(), language, namespace)
	if err != nil {
		return nil, &serviceerror.InternalServerError
	}
	if resp == nil {
		return &i18nmgt.LanguageTranslationsResponse{
			Language:     language,
			Translations: map[string]map[string]string{},
		}, nil
	}
	return &i18nmgt.LanguageTranslationsResponse{
		Language:     resp.Language,
		TotalResults: resp.TotalResults,
		Translations: resp.Translations,
	}, nil
}

func (b *i18nBridge) SetTranslationOverrides(
	_ string, _ map[string]map[string]string,
) (*i18nmgt.LanguageTranslationsResponse, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *i18nBridge) ClearTranslationOverrides(_ string) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

func (b *i18nBridge) ResolveTranslationsForKey(
	_, _, _ string,
) (*i18nmgt.TranslationResponse, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *i18nBridge) SetTranslationOverrideForKey(
	_, _, _, _ string,
) (*i18nmgt.TranslationResponse, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *i18nBridge) SetTranslationOverridesForNamespace(
	_ context.Context, _ string, _ map[string]map[string]string,
) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

func (b *i18nBridge) ClearTranslationOverrideForKey(_, _, _ string) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

func (b *i18nBridge) DeleteTranslationsByNamespace(_ context.Context, _ string) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

func (b *i18nBridge) DeleteTranslationsByKey(_ context.Context, _, _ string) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

func (b *i18nBridge) GetTranslationsByNamespace(
	_ string,
) (map[string]map[string]string, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

var _ i18nmgt.I18nServiceInterface = (*i18nBridge)(nil)
