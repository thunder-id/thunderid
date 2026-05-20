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

package notification

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// messageNotificationSenderHandler handles HTTP requests for message notification sender management
type messageNotificationSenderHandler struct {
	mgtService NotificationSenderMgtSvcInterface
	otpService OTPServiceInterface
}

// newMessageNotificationSenderHandler creates a new instance of MessageNotificationSenderHandler
func newMessageNotificationSenderHandler(
	mgtService NotificationSenderMgtSvcInterface,
	otpService OTPServiceInterface) *messageNotificationSenderHandler {
	return &messageNotificationSenderHandler{
		mgtService: mgtService,
		otpService: otpService,
	}
}

// HandleSenderListRequest handles the request to list all message notification senders
func (h *messageNotificationSenderHandler) HandleSenderListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationHandler"))
	senders, svcErr := h.mgtService.ListSenders(ctx)
	if svcErr != nil {
		h.handleError(w, svcErr, "")
		return
	}

	senderResponses := make([]common.NotificationSenderResponse, 0, len(senders))
	for _, sender := range senders {
		senderResponse, err := getSenderResponseFromDTO(&sender)
		if err != nil {
			logger.Error("Failed to convert sender to response", log.String("sender", sender.Name), log.Error(err))
			h.handleError(w, &serviceerror.InternalServerError, "Failed to convert sender to response: "+err.Error())
			return
		}
		senderResponses = append(senderResponses, senderResponse)
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, senderResponses)
}

// HandleSenderCreateRequest handles the request to create a new message notification sender
func (h *messageNotificationSenderHandler) HandleSenderCreateRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationHandler"))
	sender, err := sysutils.DecodeJSONBody[common.NotificationSenderRequest](r)
	if err != nil {
		h.handleError(w, &ErrorInvalidRequestFormat, "Failed to parse request body: "+err.Error())
		return
	}

	senderDTO, err := getDTOFromSenderRequest(sender)
	if err != nil {
		logger.Error("Failed to process sender request", log.Error(err))
		h.handleError(w, &serviceerror.InternalServerError, "Failed to process sender request: "+err.Error())
		return
	}

	createdSender, svcErr := h.mgtService.CreateSender(ctx, *senderDTO)
	if svcErr != nil {
		if svcErr.Code == ErrorDuplicateSenderName.Code {
			errResp := apierror.ErrorResponse{
				Code:        svcErr.Code,
				Message:     svcErr.Error,
				Description: svcErr.ErrorDescription,
			}

			sysutils.WriteErrorResponse(w, http.StatusConflict, errResp)
			return
		}

		h.handleError(w, svcErr, "")
		return
	}

	senderResponse, err := getSenderResponseFromDTO(createdSender)
	if err != nil {
		logger.Error("Failed to convert sender to response", log.String("sender", createdSender.Name), log.Error(err))
		h.handleError(w, &serviceerror.InternalServerError, "Failed to convert sender to response: "+err.Error())
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusCreated, senderResponse)
}

// HandleSenderGetRequest handles the request to get a message notification sender by ID
func (h *messageNotificationSenderHandler) HandleSenderGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationHandler"))
	id := r.PathValue("id")
	if !h.validateSenderID(w, id) {
		return
	}

	sender, svcErr := h.mgtService.GetSender(ctx, id)
	if svcErr != nil {
		h.handleError(w, svcErr, "")
		return
	}
	if sender == nil {
		errResp := apierror.ErrorResponse{
			Code:        ErrorSenderNotFound.Code,
			Message:     ErrorSenderNotFound.Error,
			Description: ErrorSenderNotFound.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusNotFound, errResp)
		return
	}

	senderResponse, err := getSenderResponseFromDTO(sender)
	if err != nil {
		logger.Error("Failed to convert sender to response", log.String("sender", sender.Name), log.Error(err))
		h.handleError(w, &serviceerror.InternalServerError, "Failed to convert sender to response: "+err.Error())
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, senderResponse)
}

// HandleSenderUpdateRequest handles the request to update a message notification sender
func (h *messageNotificationSenderHandler) HandleSenderUpdateRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationHandler"))
	id := r.PathValue("id")
	if !h.validateSenderID(w, id) {
		return
	}

	sender, err := sysutils.DecodeJSONBody[common.NotificationSenderRequest](r)
	if err != nil {
		h.handleError(w, &ErrorInvalidRequestFormat, "Failed to parse request body: "+err.Error())
		return
	}

	senderDTO, err := getDTOFromSenderRequest(sender)
	if err != nil {
		logger.Error("Failed to process sender request", log.Error(err))
		h.handleError(w, &serviceerror.InternalServerError, "Failed to process sender request: "+err.Error())
		return
	}

	updatedSender, svcErr := h.mgtService.UpdateSender(ctx, id, *senderDTO)
	if svcErr != nil {
		h.handleError(w, svcErr, "")
		return
	}

	senderResponse, err := getSenderResponseFromDTO(updatedSender)
	if err != nil {
		logger.Error("Failed to convert sender to response", log.String("sender", updatedSender.Name), log.Error(err))
		h.handleError(w, &serviceerror.InternalServerError, "Failed to convert sender to response: "+err.Error())
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, senderResponse)
}

// HandleSenderDeleteRequest handles the request to delete a message notification sender
func (h *messageNotificationSenderHandler) HandleSenderDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if !h.validateSenderID(w, id) {
		return
	}

	svcErr := h.mgtService.DeleteSender(ctx, id)
	if svcErr != nil {
		h.handleError(w, svcErr, "")
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusNoContent, nil)
}

// HandleOTPSendRequest handles the request to send an OTP.
func (h *messageNotificationSenderHandler) HandleOTPSendRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	request, err := sysutils.DecodeJSONBody[common.SendOTPRequest](r)
	if err != nil {
		h.handleError(w, &ErrorInvalidRequestFormat, "Failed to parse request body: "+err.Error())
		return
	}

	otpDTO := common.SendOTPDTO(*request)
	resultDTO, svcErr := h.otpService.SendOTP(ctx, otpDTO)
	if svcErr != nil {
		h.handleError(w, svcErr, "")
		return
	}

	otpResponse := common.SendOTPResponse{
		Status:       "SUCCESS",
		SessionToken: resultDTO.SessionToken,
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, otpResponse)
}

// HandleOTPVerifyRequest handles the request to verify an OTP.
func (h *messageNotificationSenderHandler) HandleOTPVerifyRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	request, err := sysutils.DecodeJSONBody[common.VerifyOTPRequest](r)
	if err != nil {
		h.handleError(w, &ErrorInvalidRequestFormat, "Failed to parse request body: "+err.Error())
		return
	}

	verifyDTO := common.VerifyOTPDTO(*request)
	resultDTO, svcErr := h.otpService.VerifyOTP(ctx, verifyDTO)
	if svcErr != nil {
		h.handleError(w, svcErr, "")
		return
	}

	response := common.VerifyOTPResponse{
		Status: string(resultDTO.Status),
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, response)
}

// handleError handles service errors and returns appropriate HTTP responses.
func (h *messageNotificationSenderHandler) handleError(w http.ResponseWriter,
	svcErr *serviceerror.ServiceError, customErrDesc string) {
	errDesc := svcErr.ErrorDescription
	if customErrDesc != "" {
		errDesc = core.I18nMessage{
			Key:          svcErr.ErrorDescription.Key,
			DefaultValue: customErrDesc,
		}
	}
	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: errDesc,
	}

	statusCode := http.StatusInternalServerError
	if svcErr.Type == serviceerror.ClientErrorType {
		switch svcErr.Code {
		case ErrorSenderNotFound.Code:
			statusCode = http.StatusNotFound
		case ErrorDuplicateSenderName.Code:
			statusCode = http.StatusConflict
		default:
			statusCode = http.StatusBadRequest
		}
	}

	sysutils.WriteErrorResponse(w, statusCode, errResp)
}

// validateSenderID validates the sender ID and returns true if valid
func (h *messageNotificationSenderHandler) validateSenderID(w http.ResponseWriter, id string) bool {
	if strings.TrimSpace(id) == "" {
		h.handleError(w, &ErrorInvalidSenderID, "Sender ID is required")
		return false
	}
	return true
}

// getDTOFromSenderRequest sanitizes the sender request and converts it to a NotificationSenderDTO.
func getDTOFromSenderRequest(sender *common.NotificationSenderRequest) (*common.NotificationSenderDTO, error) {
	name := sysutils.SanitizeString(sender.Name)
	description := sysutils.SanitizeString(sender.Description)
	providerStr := sysutils.SanitizeString(sender.Provider)

	// Sanitize properties
	properties := make([]cmodels.Property, 0, len(sender.Properties))
	for _, propDTO := range sender.Properties {
		sanitizedDTO := cmodels.PropertyDTO{
			Name:     sysutils.SanitizeString(propDTO.Name),
			Value:    sysutils.SanitizeString(propDTO.Value),
			IsSecret: propDTO.IsSecret,
		}
		property, err := sanitizedDTO.ToProperty()
		if err != nil {
			return nil, fmt.Errorf("failed to create property %s: %w", propDTO.Name, err)
		}
		properties = append(properties, *property)
	}

	senderDTO := common.NotificationSenderDTO{
		Name:        name,
		Description: description,
		Type:        common.NotificationSenderTypeMessage,
		Provider:    common.MessageProviderType(providerStr),
		Properties:  properties,
	}
	return &senderDTO, nil
}

// getSenderResponseFromDTO converts a NotificationSenderDTO to a response object, masking secret properties.
func getSenderResponseFromDTO(sender *common.NotificationSenderDTO) (common.NotificationSenderResponse, error) {
	returnSender := common.NotificationSenderResponse{
		ID:          sender.ID,
		Name:        sender.Name,
		Description: sender.Description,
		Provider:    sender.Provider,
	}

	// Mask secret properties in the response.
	senderProperties := make([]cmodels.PropertyDTO, 0, len(sender.Properties))
	for _, property := range sender.Properties {
		if property.IsSecret() {
			maskedProperty := &cmodels.PropertyDTO{
				Name:     property.GetName(),
				Value:    "******",
				IsSecret: property.IsSecret(),
			}
			senderProperties = append(senderProperties, *maskedProperty)
		} else {
			propertyDTO, err := property.ToPropertyDTO()
			if err != nil {
				return common.NotificationSenderResponse{},
					fmt.Errorf("failed to convert property %s: %w", property.GetName(), err)
			}
			senderProperties = append(senderProperties, *propertyDTO)
		}
	}
	returnSender.Properties = senderProperties

	return returnSender, nil
}
