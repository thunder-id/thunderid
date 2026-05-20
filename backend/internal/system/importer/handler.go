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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package importer

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

type importHandler struct {
	service ImportServiceInterface
	logger  *log.Logger
}

func newImportHandler(service ImportServiceInterface) *importHandler {
	return &importHandler{
		service: service,
		logger:  log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ImportHandler")),
	}
}

func (ih *importHandler) HandleImportRequest(w http.ResponseWriter, r *http.Request) {
	importRequest, err := sysutils.DecodeJSONBody[ImportRequest](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidImportRequest.Code,
			Message:     ErrorInvalidImportRequest.Error,
			Description: ErrorInvalidImportRequest.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	importResponse, svcErr := ih.service.ImportResources(r.Context(), importRequest)
	if svcErr != nil {
		ih.handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, importResponse)
}

func (ih *importHandler) HandleDeleteImportRequest(w http.ResponseWriter, r *http.Request) {
	deleteRequest, err := sysutils.DecodeJSONBody[DeleteResourceRequest](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidImportRequest.Code,
			Message:     ErrorInvalidImportRequest.Error,
			Description: ErrorInvalidImportRequest.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	deleteResponse, svcErr := ih.service.DeleteResource(r.Context(), deleteRequest)
	if svcErr != nil {
		ih.handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, deleteResponse)
}

func (ih *importHandler) handleError(w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == serviceerror.ClientErrorType {
		statusCode = http.StatusBadRequest
	}

	if statusCode == http.StatusInternalServerError {
		ih.logger.Error(
			"Import request failed with server error",
			log.String("code", svcErr.Code),
			log.String("error", svcErr.Error.DefaultValue),
			log.String("description", svcErr.ErrorDescription.DefaultValue),
		)
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	sysutils.WriteErrorResponse(w, statusCode, errResp)
}
