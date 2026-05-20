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

package executor

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/notification"
	notifcm "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/template"
)

// phoneNumberRegex matches phone numbers in various formats including optional +, digits, spaces, dashes,
// dots, and parentheses with a total length of 7 to 20 characters.
var phoneNumberRegex = regexp.MustCompile(`^\+?[0-9\s\-().]{7,20}$`)

// smsExecutor sends an SMS message using the configured sender from node properties and a template-based body.
type smsExecutor struct {
	core.ExecutorInterface
	logger          *log.Logger
	notifSenderSvc  notification.NotificationSenderServiceInterface
	templateService template.TemplateServiceInterface
}

// newSMSExecutor creates a new instance of smsExecutor.
func newSMSExecutor(flowFactory core.FlowFactoryInterface,
	notifSenderSvc notification.NotificationSenderServiceInterface,
	templateService template.TemplateServiceInterface) *smsExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SMSExecutor"))
	base := flowFactory.CreateExecutor(
		ExecutorNameSMSExecutor,
		common.ExecutorTypeUtility,
		[]common.Input{
			{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
		},
		[]common.Input{},
	)
	return &smsExecutor{
		ExecutorInterface: base,
		logger:            logger,
		notifSenderSvc:    notifSenderSvc,
		templateService:   templateService,
	}
}

// Execute resolves the recipient from user inputs or runtime data and the sender ID from node properties,
// then renders the SMS body from a template and sends it.
func (e *smsExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing SMS executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if e.notifSenderSvc == nil {
		return nil, errors.New("notification sender service is not configured")
	}

	phoneAttr := resolveInputIdentifierByType(ctx, common.InputTypePhone, common.AttributeMobileNumber)

	recipient := resolveRecipientMobile(ctx, phoneAttr)
	if recipient == "" {
		logger.Debug("SMS recipient not found in user inputs or runtime data")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "SMS recipient is required"
		return execResp, nil
	}

	if !isValidPhoneNumber(recipient) {
		logger.Debug("SMS recipient is not a valid phone number", log.String("phoneAttr", phoneAttr))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "SMS recipient is not a valid phone number"
		return execResp, nil
	}

	senderID, err := resolveStringNodeProperty(ctx, propertyKeyNotificationSenderID)
	if err != nil {
		return nil, fmt.Errorf("senderId is not configured in node properties: %w", err)
	}

	tmplProp, ok := ctx.NodeProperties[propertyKeySMSTemplate]
	if !ok {
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "SMS template is required"
		return execResp, nil
	}
	tmplStr, ok := tmplProp.(string)
	if !ok {
		return nil, fmt.Errorf("invalid type for %s: expected string, got %T with value %v",
			propertyKeySMSTemplate, tmplProp, tmplProp)
	}
	if tmplStr == "" {
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "SMS template is required"
		return execResp, nil
	}
	scenario := template.ScenarioType(tmplStr)

	templateData := template.TemplateData{
		"appName":    ctx.Application.Name,
		"inviteLink": ctx.RuntimeData[common.RuntimeKeyInviteLink],
	}

	rendered, svcErr := e.templateService.Render(ctx.Context, scenario, template.TemplateTypeSMS, templateData)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to render SMS template: %s", svcErr.Code)
	}

	notifSvcErr := e.notifSenderSvc.Send(ctx.Context, notifcm.ChannelTypeSMS, senderID,
		notifcm.NotificationData{Recipient: recipient, Body: rendered.Body})
	if notifSvcErr != nil {
		if ctx.FlowType == common.FlowTypeUserOnboarding && notifSvcErr.Type == serviceerror.ClientErrorType {
			execResp.Status = common.ExecFailure
			execResp.FailureReason = "Notification configuration is wrong or not set."
			return execResp, nil
		}
		return nil, fmt.Errorf("SMS send failed: %s", notifSvcErr.ErrorDescription)
	}

	logger.Debug("SMS sent successfully", log.MaskedString("recipient", recipient))

	execResp.AdditionalData[common.DataSMSSent] = dataValueTrue
	execResp.Status = common.ExecComplete
	return execResp, nil
}

// resolveRecipientMobile retrieves the recipient mobile number from user inputs or runtime data
// using the given attribute name as the lookup key.
func resolveRecipientMobile(ctx *core.NodeContext, phoneAttr string) string {
	if mobile, ok := ctx.UserInputs[phoneAttr]; ok && mobile != "" {
		return mobile
	}
	if mobile, ok := ctx.RuntimeData[phoneAttr]; ok && mobile != "" {
		return mobile
	}
	return ""
}

// resolveInputIdentifierByType returns the identifier of the first input in ctx.NodeInputs
// matching inputType, or fallback if none is found.
func resolveInputIdentifierByType(ctx *core.NodeContext, inputType string, fallback string) string {
	if input, ok := findInputByType(ctx.NodeInputs, inputType); ok {
		return input.Identifier
	}
	return fallback
}

// findInputByType returns the first input in the given slice whose Type matches inputType.
func findInputByType(inputs []common.Input, inputType string) (common.Input, bool) {
	for _, input := range inputs {
		if input.Type == inputType {
			return input, true
		}
	}
	return common.Input{}, false
}

// isValidPhoneNumber returns true if the given phone number matches an acceptable format.
func isValidPhoneNumber(phone string) bool {
	return phoneNumberRegex.MatchString(phone)
}

// resolveStringNodeProperty reads a string property from NodeProperties, returning an error if missing or wrong type.
func resolveStringNodeProperty(ctx *core.NodeContext, key string) (string, error) {
	val, ok := ctx.NodeProperties[key]
	if !ok {
		return "", errors.New("property not found")
	}
	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("invalid type for %s: expected string, got %T", key, val)
	}
	if str == "" {
		return "", errors.New("property is empty")
	}
	return str, nil
}
