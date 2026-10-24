// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"net/http"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
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
	// An import writes every resource it carries, which can take longer than the server's write
	// timeout allows any request.
	if err := sysutils.ExtendWriteDeadline(w, config.GetServerRuntime().Config.Import.Timeout()); err != nil {
		ih.logger.Warn(r.Context(),
			"Could not extend the write deadline of an import; the server's write timeout applies", log.Error(err))
	}
	importRequest, err := sysutils.DecodeJSONBody[ImportRequest](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidImportRequest.Code,
			Message:     ErrorInvalidImportRequest.Error,
			Description: ErrorInvalidImportRequest.ErrorDescription,
		}
		sysutils.WriteErrorResponse(r.Context(), w, http.StatusBadRequest, errResp)
		return
	}

	importResponse, svcErr := ih.service.ImportResources(r.Context(), importRequest)
	if svcErr != nil {
		ih.handleError(r.Context(), w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, importResponse)
}

func (ih *importHandler) HandleDeleteImportRequest(w http.ResponseWriter, r *http.Request) {
	deleteRequest, err := sysutils.DecodeJSONBody[DeleteResourceRequest](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidImportRequest.Code,
			Message:     ErrorInvalidImportRequest.Error,
			Description: ErrorInvalidImportRequest.ErrorDescription,
		}
		sysutils.WriteErrorResponse(r.Context(), w, http.StatusBadRequest, errResp)
		return
	}

	deleteResponse, svcErr := ih.service.DeleteResource(r.Context(), deleteRequest)
	if svcErr != nil {
		ih.handleError(r.Context(), w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, deleteResponse)
}

func (ih *importHandler) handleError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		statusCode = http.StatusBadRequest
	}

	if statusCode == http.StatusInternalServerError {
		ih.logger.Error(ctx,
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

	sysutils.WriteErrorResponse(ctx, w, statusCode, errResp)
}
