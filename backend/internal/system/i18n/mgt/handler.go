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

package mgt

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const handlerLoggerComponentName = "I18nHandler"

// i18nHandler is the handler for i18n management operations.
type i18nHandler struct {
	i18nService I18nServiceInterface
}

// newI18nHandler creates a new instance of i18nHandler.
func newI18nHandler(i18nService I18nServiceInterface) *i18nHandler {
	return &i18nHandler{
		i18nService: i18nService,
	}
}

// HandleListLanguages handles GET /i18n/languages
func (h *i18nHandler) HandleListLanguages(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	localeCodes, svcErr := h.i18nService.ListLanguages()
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	resp := LanguageListResponse{
		Languages: localeCodes,
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, resp)
	logger.Debug("Successfully retrieved languages", log.Int("count", len(localeCodes)))
}

// HandleResolveTranslationsByLanguage handles GET /i18n/languages/{language}/translations/resolve
func (h *i18nHandler) HandleResolveTranslationsByLanguage(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	language := r.PathValue("language")
	namespace := r.URL.Query().Get("namespace")

	sanitizedLanguage := sysutils.SanitizeString(language)
	sanitizedNamespace := sysutils.SanitizeString(namespace)

	resp, svcErr := h.i18nService.ResolveTranslations(sanitizedLanguage, sanitizedNamespace)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, resp)
	logger.Debug("Successfully resolved translations",
		log.String("language", sanitizedLanguage),
		log.String("namespace", sanitizedNamespace),
		log.Int("totalResults", resp.TotalResults))
}

// HandleSetOverrideTranslationsByLanguage handles POST /i18n/languages/{language}/translations
func (h *i18nHandler) HandleSetOverrideTranslationsByLanguage(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	language := r.PathValue("language")
	sanitizedLanguage := sysutils.SanitizeString(language)

	req, err := sysutils.DecodeJSONBody[SetTranslationsRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	resp, svcErr := h.i18nService.SetTranslationOverrides(sanitizedLanguage, req.Translations)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, resp)
	logger.Debug("Successfully set override translations",
		log.String("language", sanitizedLanguage),
		log.Int("totalResults", resp.TotalResults))
}

// HandleClearOverrideTranslationsByLanguage handles DELETE /i18n/languages/{language}/translations
func (h *i18nHandler) HandleClearOverrideTranslationsByLanguage(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	language := r.PathValue("language")
	sanitizedLanguage := sysutils.SanitizeString(language)

	svcErr := h.i18nService.ClearTranslationOverrides(sanitizedLanguage)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	logger.Debug("Successfully cleared override translations", log.String("language", sanitizedLanguage))
}

// HandleResolveTranslation handles GET /i18n/languages/{language}/translations/ns/{namespace}/keys/{key}/resolve
func (h *i18nHandler) HandleResolveTranslation(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	language := r.PathValue("language")
	namespace := r.PathValue("namespace")
	key := r.PathValue("key")

	sanitizedLanguage := sysutils.SanitizeString(language)
	sanitizedNamespace := sysutils.SanitizeString(namespace)
	sanitizedKey := sysutils.SanitizeString(key)

	resp, svcErr := h.i18nService.ResolveTranslationsForKey(sanitizedLanguage, sanitizedNamespace, sanitizedKey)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, resp)
	logger.Debug("Successfully resolved translation",
		log.String("language", sanitizedLanguage),
		log.String("namespace", sanitizedNamespace),
		log.String("key", sanitizedKey))
}

// HandleSetOverrideTranslation handles POST /i18n/languages/{language}/translations/ns/{namespace}/keys/{key}
func (h *i18nHandler) HandleSetOverrideTranslation(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	language := r.PathValue("language")
	namespace := r.PathValue("namespace")
	key := r.PathValue("key")

	sanitizedLanguage := sysutils.SanitizeString(language)
	sanitizedNamespace := sysutils.SanitizeString(namespace)
	sanitizedKey := sysutils.SanitizeString(key)

	req, err := sysutils.DecodeJSONBody[SetTranslationRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitizedValue := sysutils.SanitizeString(req.Value)

	resp, svcErr := h.i18nService.SetTranslationOverrideForKey(
		sanitizedLanguage, sanitizedNamespace, sanitizedKey, sanitizedValue)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, resp)
	logger.Debug("Successfully set override translation",
		log.String("language", sanitizedLanguage),
		log.String("namespace", sanitizedNamespace),
		log.String("key", sanitizedKey))
}

// HandleClearOverrideTranslation handles
// DELETE /i18n/languages/{language}/translations/ns/{namespace}/keys/{key}
func (h *i18nHandler) HandleClearOverrideTranslation(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	language := r.PathValue("language")
	namespace := r.PathValue("namespace")
	key := r.PathValue("key")

	sanitizedLanguage := sysutils.SanitizeString(language)
	sanitizedNamespace := sysutils.SanitizeString(namespace)
	sanitizedKey := sysutils.SanitizeString(key)

	svcErr := h.i18nService.ClearTranslationOverrideForKey(sanitizedLanguage, sanitizedNamespace, sanitizedKey)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	logger.Debug("Successfully cleared override translation",
		log.String("language", sanitizedLanguage),
		log.String("namespace", sanitizedNamespace),
		log.String("key", sanitizedKey))
}

// handleError handles service errors and returns appropriate HTTP responses.
func handleError(w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == serviceerror.ClientErrorType {
		statusCode = http.StatusBadRequest
		// Use 404 for not found errors
		if svcErr.Code == "I18N-1006" {
			statusCode = http.StatusNotFound
		}
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	sysutils.WriteErrorResponse(w, statusCode, errResp)
}
