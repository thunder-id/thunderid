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

package layoutmgt

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

const handlerLoggerComponentName = "LayoutMgtHandler"

// layoutMgtHandler is the handler for layout management operations.
type layoutMgtHandler struct {
	layoutMgtService LayoutMgtServiceInterface
	logger           *log.Logger
}

// newLayoutMgtHandler creates a new instance of layoutMgtHandler
func newLayoutMgtHandler(layoutMgtService LayoutMgtServiceInterface) *layoutMgtHandler {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))
	return &layoutMgtHandler{
		layoutMgtService: layoutMgtService,
		logger:           logger,
	}
}

// HandleLayoutListRequest handles the list layout configurations request.
func (lh *layoutMgtHandler) HandleLayoutListRequest(w http.ResponseWriter, r *http.Request) {
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	layoutList, svcErr := lh.layoutMgtService.GetLayoutList(limit, offset)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	layouts := make([]LayoutListItem, 0, len(layoutList.Layouts))
	for _, layout := range layoutList.Layouts {
		layouts = append(layouts, LayoutListItem{
			ID:          layout.ID,
			Handle:      layout.Handle,
			DisplayName: layout.DisplayName,
			Description: layout.Description,
			CreatedAt:   layout.CreatedAt,
			UpdatedAt:   layout.UpdatedAt,
			IsReadOnly:  layout.IsReadOnly,
		})
	}

	layoutListResponse := &LayoutListResponse{
		TotalResults: layoutList.TotalResults,
		StartIndex:   layoutList.StartIndex,
		Count:        layoutList.Count,
		Layouts:      layouts,
		Links:        toHTTPLinks(layoutList.Links),
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, layoutListResponse)

	lh.logger.Debug("Successfully listed layout configurations with pagination",
		log.Int("limit", limit), log.Int("offset", offset),
		log.Int("totalResults", layoutListResponse.TotalResults),
		log.Int("count", layoutListResponse.Count))
}

// HandleLayoutPostRequest handles the create layout configuration request.
func (lh *layoutMgtHandler) HandleLayoutPostRequest(w http.ResponseWriter, r *http.Request) {
	createRequest, err := sysutils.DecodeJSONBody[CreateLayoutRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidLayoutData)
		return
	}

	createdLayout, svcErr := lh.layoutMgtService.CreateLayout(*createRequest)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	layoutResponse := Layout{
		ID:          createdLayout.ID,
		Handle:      createdLayout.Handle,
		DisplayName: createdLayout.DisplayName,
		Description: createdLayout.Description,
		Layout:      createdLayout.Layout,
		CreatedAt:   createdLayout.CreatedAt,
		UpdatedAt:   createdLayout.UpdatedAt,
		IsReadOnly:  createdLayout.IsReadOnly,
	}

	sysutils.WriteSuccessResponse(w, http.StatusCreated, layoutResponse)

	lh.logger.Debug("Successfully created layout configuration", log.String("id", createdLayout.ID))
}

// HandleLayoutGetRequest handles the get layout configuration request.
func (lh *layoutMgtHandler) HandleLayoutGetRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	layout, svcErr := lh.layoutMgtService.GetLayout(id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	layoutResponse := Layout{
		ID:          layout.ID,
		Handle:      layout.Handle,
		DisplayName: layout.DisplayName,
		Description: layout.Description,
		Layout:      layout.Layout,
		CreatedAt:   layout.CreatedAt,
		UpdatedAt:   layout.UpdatedAt,
		IsReadOnly:  layout.IsReadOnly,
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, layoutResponse)

	lh.logger.Debug("Successfully retrieved layout configuration", log.String("id", id))
}

// HandleLayoutPutRequest handles the update layout configuration request.
func (lh *layoutMgtHandler) HandleLayoutPutRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	updateRequest, err := sysutils.DecodeJSONBody[UpdateLayoutRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidLayoutData)
		return
	}

	updatedLayout, svcErr := lh.layoutMgtService.UpdateLayout(id, *updateRequest)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	layoutResponse := Layout{
		ID:          updatedLayout.ID,
		Handle:      updatedLayout.Handle,
		DisplayName: updatedLayout.DisplayName,
		Description: updatedLayout.Description,
		Layout:      updatedLayout.Layout,
		CreatedAt:   updatedLayout.CreatedAt,
		UpdatedAt:   updatedLayout.UpdatedAt,
		IsReadOnly:  updatedLayout.IsReadOnly,
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, layoutResponse)

	lh.logger.Debug("Successfully updated layout configuration", log.String("id", id))
}

// HandleLayoutDeleteRequest handles the delete layout configuration request.
func (lh *layoutMgtHandler) HandleLayoutDeleteRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svcErr := lh.layoutMgtService.DeleteLayout(id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusNoContent, nil)
	lh.logger.Debug("Successfully deleted layout configuration", log.String("id", id))
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
	if svcErr.Type == serviceerror.ClientErrorType {
		switch svcErr.Code {
		case ErrorLayoutNotFound.Code:
			statusCode = http.StatusNotFound
		case ErrorLayoutAlreadyExists.Code:
			statusCode = http.StatusConflict
		case ErrorInvalidLayoutID.Code, ErrorInvalidLayoutData.Code:
			statusCode = http.StatusBadRequest
		default:
			statusCode = http.StatusBadRequest
		}
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	sysutils.WriteErrorResponse(w, statusCode, errResp)
}
