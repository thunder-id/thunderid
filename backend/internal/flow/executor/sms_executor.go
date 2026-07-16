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

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/notification"
	notifcm "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/template"
)

// phoneNumberRegex matches phone numbers in various formats including optional +, digits, spaces, dashes,
// dots, and parentheses with a total length of 7 to 20 characters.
var phoneNumberRegex = regexp.MustCompile(`^\+?[0-9\s\-().]{7,20}$`)

// smsExecutor sends an SMS message using the configured sender from node properties and a template-based body.
type smsExecutor struct {
	providers.Executor
	logger          *log.Logger
	notifSenderSvc  notification.NotificationSenderServiceInterface
	templateService template.TemplateServiceInterface
	entityProvider  entityprovider.EntityProviderInterface
}

// newSMSExecutor creates a new instance of smsExecutor.
func newSMSExecutor(flowFactory core.FlowFactoryInterface,
	notifSenderSvc notification.NotificationSenderServiceInterface,
	templateService template.TemplateServiceInterface,
	entityProvider entityprovider.EntityProviderInterface) *smsExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SMSExecutor"))
	base := flowFactory.CreateExecutor(
		ExecutorNameSMSExecutor,
		providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
		},
		[]providers.Input{},
		&providers.ExecutorMeta{
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyNotificationSenderID, IsRequired: true},
				{Property: propertyKeySMSTemplate, IsRequired: true},
			},
		},
	)
	return &smsExecutor{
		Executor:        base,
		logger:          logger,
		notifSenderSvc:  notifSenderSvc,
		templateService: templateService,
		entityProvider:  entityProvider,
	}
}

// Execute resolves the recipient from user inputs or runtime data and the sender ID from node properties,
// then renders the SMS body from a template and sends it.
func (e *smsExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing SMS executor")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if e.notifSenderSvc == nil {
		return nil, errors.New("notification sender service is not configured")
	}

	phoneAttr := resolveInputIdentifierByType(ctx, providers.InputTypePhone, common.AttributeMobileNumber)

	recipient := e.resolveRecipientMobile(ctx, phoneAttr)
	if recipient == "" {
		logger.Debug(ctx.Context, "SMS recipient not found in user inputs or runtime data")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrSMSRecipientMissing
		return execResp, nil
	}

	if !isValidPhoneNumber(recipient) {
		logger.Debug(ctx.Context, "SMS recipient is not a valid phone number",
			log.String("phoneAttr", phoneAttr))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrSMSInvalidPhone
		return execResp, nil
	}

	senderID, err := resolveStringNodeProperty(ctx, propertyKeyNotificationSenderID)
	if err != nil {
		return nil, fmt.Errorf("senderId is not configured in node properties: %w", err)
	}

	tmplProp, ok := ctx.NodeProperties[propertyKeySMSTemplate]
	if !ok {
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrSMSTemplateMissing
		return execResp, nil
	}
	tmplStr, ok := tmplProp.(string)
	if !ok {
		return nil, fmt.Errorf("invalid type for %s: expected string, got %T with value %v",
			propertyKeySMSTemplate, tmplProp, tmplProp)
	}
	if tmplStr == "" {
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrSMSTemplateMissing
		return execResp, nil
	}
	scenario := template.ScenarioType(tmplStr)

	templateData := template.TemplateData{"appName": ctx.Application.Name}
	if ctx.ForwardedData != nil {
		if fwdTemplateData, ok := ctx.ForwardedData[common.ForwardedDataKeyTemplateData]; ok {
			switch data := fwdTemplateData.(type) {
			case map[string]interface{}:
				for k, v := range data {
					templateData[k] = fmt.Sprintf("%v", v)
				}
			case map[string]string:
				for k, v := range data {
					templateData[k] = v
				}
			}
		}
	}

	rendered, svcErr := e.templateService.Render(ctx.Context, scenario, template.TemplateTypeSMS, templateData)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to render SMS template: %s", svcErr.Code)
	}

	notifSvcErr := e.notifSenderSvc.Send(ctx.Context, notifcm.ChannelTypeSMS, senderID,
		notifcm.NotificationData{Recipient: recipient, Body: rendered.Body})
	if notifSvcErr != nil {
		if ctx.FlowType == providers.FlowTypeUserOnboarding && notifSvcErr.Type == tidcommon.ClientErrorType {
			execResp.Status = providers.ExecFailure
			execResp.Error = &ErrSMSProviderNotConfigured
			return execResp, nil
		}
		return nil, fmt.Errorf("SMS send failed: %s", notifSvcErr.ErrorDescription)
	}

	logger.Debug(ctx.Context, "SMS sent successfully", log.MaskedString("recipient", recipient))

	execResp.AdditionalData[common.DataSMSSent] = dataValueTrue
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// resolveRecipientMobile retrieves the recipient mobile number from user inputs, runtime data,
// or the entity provider (via RuntimeData["userID"]), in that order.
func (e *smsExecutor) resolveRecipientMobile(ctx *providers.NodeContext, phoneAttr string) string {
	if mobile, ok := ctx.UserInputs[phoneAttr]; ok && mobile != "" {
		return mobile
	}
	if mobile, ok := ctx.RuntimeData[phoneAttr]; ok && mobile != "" {
		return mobile
	}
	if userID, ok := ctx.RuntimeData[userAttributeUserID]; ok && userID != "" && e.entityProvider != nil {
		user, err := e.entityProvider.GetEntity(userID)
		if err == nil {
			if mobile, attrErr := GetUserAttribute(user, phoneAttr); attrErr == nil {
				return mobile
			}
		}
	}
	return ""
}

// isValidPhoneNumber returns true if the given phone number matches an acceptable format.
func isValidPhoneNumber(phone string) bool {
	return phoneNumberRegex.MatchString(phone)
}

// resolveStringNodeProperty reads a string property from NodeProperties, returning an error if missing or wrong type.
func resolveStringNodeProperty(ctx *providers.NodeContext, key string) (string, error) {
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
