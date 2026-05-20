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

package thememgt

import (
	"net/http"
	"net/url"
	"strconv"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const handlerLoggerComponentName = "ThemeMgtHandler"

// themeMgtHandler is the handler for theme management operations.
type themeMgtHandler struct {
	themeMgtService ThemeMgtServiceInterface
	logger          *log.Logger
}

// newThemeMgtHandler creates a new instance of themeMgtHandler
func newThemeMgtHandler(themeMgtService ThemeMgtServiceInterface) *themeMgtHandler {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))
	return &themeMgtHandler{
		themeMgtService: themeMgtService,
		logger:          logger,
	}
}

// HandleThemeListRequest handles the list theme configurations request.
func (th *themeMgtHandler) HandleThemeListRequest(w http.ResponseWriter, r *http.Request) {
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	themeList, svcErr := th.themeMgtService.GetThemeList(limit, offset)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	themes := make([]ThemeListItem, 0, len(themeList.Themes))
	for _, theme := range themeList.Themes {
		defaultColorScheme, primaryColor := extractThemeColorInfo(theme.Theme)
		themes = append(themes, ThemeListItem{
			ID:                 theme.ID,
			Handle:             theme.Handle,
			DisplayName:        theme.DisplayName,
			Description:        theme.Description,
			DefaultColorScheme: defaultColorScheme,
			PrimaryColor:       primaryColor,
			CreatedAt:          theme.CreatedAt,
			UpdatedAt:          theme.UpdatedAt,
			IsReadOnly:         theme.IsReadOnly,
		})
	}

	themeListResponse := &ThemeListResponse{
		TotalResults: themeList.TotalResults,
		StartIndex:   themeList.StartIndex,
		Count:        themeList.Count,
		Themes:       themes,
		Links:        toHTTPLinks(themeList.Links),
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, themeListResponse)

	th.logger.Debug("Successfully listed theme configurations with pagination",
		log.Int("limit", limit), log.Int("offset", offset),
		log.Int("totalResults", themeListResponse.TotalResults),
		log.Int("count", themeListResponse.Count))
}

// HandleThemePostRequest handles the create theme configuration request.
func (th *themeMgtHandler) HandleThemePostRequest(w http.ResponseWriter, r *http.Request) {
	createRequest, err := sysutils.DecodeJSONBody[CreateThemeRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidThemeData)
		return
	}

	createdTheme, svcErr := th.themeMgtService.CreateTheme(CreateThemeRequestWithID{
		Handle:      createRequest.Handle,
		DisplayName: createRequest.DisplayName,
		Description: createRequest.Description,
		Theme:       createRequest.Theme,
	})
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	themeResponse := Theme{
		ID:          createdTheme.ID,
		Handle:      createdTheme.Handle,
		DisplayName: createdTheme.DisplayName,
		Description: createdTheme.Description,
		Theme:       createdTheme.Theme,
		CreatedAt:   createdTheme.CreatedAt,
		UpdatedAt:   createdTheme.UpdatedAt,
	}

	sysutils.WriteSuccessResponse(w, http.StatusCreated, themeResponse)

	th.logger.Debug("Successfully created theme configuration", log.String("id", createdTheme.ID))
}

// HandleThemeGetRequest handles the get theme configuration request.
func (th *themeMgtHandler) HandleThemeGetRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	theme, svcErr := th.themeMgtService.GetTheme(id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	themeResponse := Theme{
		ID:          theme.ID,
		Handle:      theme.Handle,
		DisplayName: theme.DisplayName,
		Description: theme.Description,
		Theme:       theme.Theme,
		CreatedAt:   theme.CreatedAt,
		UpdatedAt:   theme.UpdatedAt,
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, themeResponse)

	th.logger.Debug("Successfully retrieved theme configuration", log.String("id", id))
}

// HandleThemePutRequest handles the update theme configuration request.
func (th *themeMgtHandler) HandleThemePutRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	updateRequest, err := sysutils.DecodeJSONBody[UpdateThemeRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidThemeData)
		return
	}

	updatedTheme, svcErr := th.themeMgtService.UpdateTheme(id, *updateRequest)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	themeResponse := Theme{
		ID:          updatedTheme.ID,
		Handle:      updatedTheme.Handle,
		DisplayName: updatedTheme.DisplayName,
		Description: updatedTheme.Description,
		Theme:       updatedTheme.Theme,
		CreatedAt:   updatedTheme.CreatedAt,
		UpdatedAt:   updatedTheme.UpdatedAt,
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, themeResponse)

	th.logger.Debug("Successfully updated theme configuration", log.String("id", id))
}

// HandleThemeDeleteRequest handles the delete theme configuration request.
func (th *themeMgtHandler) HandleThemeDeleteRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svcErr := th.themeMgtService.DeleteTheme(id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusNoContent, nil)
	th.logger.Debug("Successfully deleted theme configuration", log.String("id", id))
}

// parsePaginationParams parses limit and offset query parameters from the request.
func parsePaginationParams(query url.Values) (int, int, *serviceerror.ServiceError) {
	limit := 0
	offset := 0

	if limitStr := query.Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err != nil {
			return 0, 0, &ErrorInvalidLimitParam
		} else {
			limit = parsedLimit
		}
	}

	if offsetStr := query.Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err != nil {
			return 0, 0, &ErrorInvalidOffsetParam
		} else {
			offset = parsedOffset
		}
	}

	if limit == 0 {
		limit = serverconst.DefaultPageSize
	}

	return limit, offset, nil
}

// toHTTPLinks converts service layer Links to HTTP LinkResponses.
func toHTTPLinks(links []Link) []LinkResponse {
	httpLinks := make([]LinkResponse, len(links))
	for i, link := range links {
		httpLinks[i] = LinkResponse(link)
	}
	return httpLinks
}

// handleError handles service errors and returns appropriate HTTP responses.
func handleError(w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	statusCode := http.StatusInternalServerError
	switch {
	case svcErr == &ErrorThemeNotFound:
		statusCode = http.StatusNotFound
	case svcErr == &ErrorThemeInUse:
		statusCode = http.StatusConflict
	case svcErr.Type == serviceerror.ClientErrorType:
		statusCode = http.StatusBadRequest
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	sysutils.WriteErrorResponse(w, statusCode, errResp)
}
