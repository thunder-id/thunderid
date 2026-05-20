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

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/email"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/template"
)

// emailExecutor sends emails based on the configured email template and runtime context data.
// When email is not configured (emailClient is nil), it returns a failure status.
type emailExecutor struct {
	core.ExecutorInterface
	logger          *log.Logger
	emailClient     email.EmailClientInterface
	templateService template.TemplateServiceInterface
	entityProvider  entityprovider.EntityProviderInterface
}

// defaultEmailInput is the default input definition for email collection.
var defaultEmailInput = common.Input{
	Ref:        "email_input",
	Identifier: userAttributeEmail,
	Type:       common.InputTypeEmail,
	Required:   true,
}

// newEmailExecutor creates a new instance of the email executor.
func newEmailExecutor(flowFactory core.FlowFactoryInterface, emailClient email.EmailClientInterface,
	templateService template.TemplateServiceInterface,
	entityProvider entityprovider.EntityProviderInterface) *emailExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EmailExecutor"))
	base := flowFactory.CreateExecutor(
		ExecutorNameEmailExecutor,
		common.ExecutorTypeUtility,
		[]common.Input{},
		[]common.Input{
			defaultEmailInput,
		},
	)
	return &emailExecutor{
		ExecutorInterface: base,
		logger:            logger,
		emailClient:       emailClient,
		templateService:   templateService,
		entityProvider:    entityProvider,
	}
}

// Execute sends an email using the data from the runtime context.
func (e *emailExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	switch ctx.ExecutorMode {
	case ExecutorModeSend:
		return e.executeSend(ctx)
	default:
		return nil, fmt.Errorf("invalid executor mode for EmailExecutor: %s", ctx.ExecutorMode)
	}
}

// executeSend resolves the email template, constructs the email, and sends it.
// If the email client is not configured, it returns a failure status.
func (e *emailExecutor) executeSend(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing email executor in send mode")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if ctx.RuntimeData[common.RuntimeKeySkipDelivery] == dataValueTrue {
		logger.Debug("Delivery marked as skipped, completing without sending email")
		execResp.Status = common.ExecComplete
		return execResp, nil
	}

	if e.emailClient == nil {
		execResp.AdditionalData[common.DataEmailSent] = dataValueFalse
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Email service is not configured"
		logger.Debug("Email client not configured")
		return execResp, nil
	}

	if e.templateService == nil {
		return nil, errors.New("template service is not configured")
	}

	// Resolve recipient email from user inputs or runtime data.
	recipient, err := e.resolveRecipientEmail(ctx, logger)
	if err != nil {
		return nil, err
	}
	if recipient == "" {
		logger.Debug("Email recipient not found in user inputs or runtime data")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Email recipient is required"
		return execResp, nil
	}

	var scenario template.ScenarioType
	if tmplProp, ok := ctx.NodeProperties[propertyKeyEmailTemplate]; ok {
		tmplStr, ok := tmplProp.(string)
		if !ok {
			return nil, fmt.Errorf("invalid type for %s: expected string, got %T with value %v",
				propertyKeyEmailTemplate, tmplProp, tmplProp)
		}
		if tmplStr == "" {
			scenario = template.ScenarioUserInvite
		} else {
			scenario = template.ScenarioType(tmplStr)
		}
	} else {
		scenario = template.ScenarioUserInvite
	}

	templateData := e.resolveTemplateData(ctx)
	rendered, svcErr := e.templateService.Render(ctx.Context, scenario, template.TemplateTypeEmail, templateData)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to render email template: %s", svcErr.Code)
	}

	emailData := email.EmailData{
		To:      []string{recipient},
		Subject: rendered.Subject,
		Body:    rendered.Body,
		IsHTML:  rendered.IsHTML,
	}

	if err := e.emailClient.Send(emailData); err != nil {
		if isEmailError(err) {
			logger.Error("Error sending mail : ", log.Error(err))
			execResp.Status = common.ExecFailure
			execResp.FailureReason = "Failed to send email"
			return execResp, nil
		}
		return nil, fmt.Errorf("email send failed: %w", err)
	}

	logger.Debug("Email sent successfully",
		log.MaskedString("recipient", recipient))

	execResp.AdditionalData[common.DataEmailSent] = dataValueTrue
	execResp.Status = common.ExecComplete
	return execResp, nil
}

// resolveRecipientEmail retrieves the recipient email from user inputs, runtime data, or forwarded data.
func (e *emailExecutor) resolveRecipientEmail(ctx *core.NodeContext, logger *log.Logger) (string, error) {
	emailAttr := e.resolveEmailInput(ctx).Identifier

	if recipientEmail, ok := ctx.ForwardedData[emailAttr]; ok {
		if emailStr, isString := recipientEmail.(string); isString && emailStr != "" {
			return emailStr, nil
		}
	}
	if recipientEmail, ok := ctx.RuntimeData[emailAttr]; ok && recipientEmail != "" {
		return recipientEmail, nil
	}
	if recipientEmail, ok := ctx.UserInputs[emailAttr]; ok && recipientEmail != "" {
		return recipientEmail, nil
	}

	if userID, ok := ctx.RuntimeData[userAttributeUserID]; ok && userID != "" {
		if e.entityProvider == nil {
			return "", errors.New("entity provider is not configured for email resolution")
		}
		user, providerErr := e.entityProvider.GetEntity(userID)
		if providerErr != nil {
			if providerErr.Code == entityprovider.ErrorCodeEntityNotFound {
				return "", nil
			}
			return "", fmt.Errorf("failed to fetch user from entity provider: %w", providerErr)
		}

		if recipientEmail, err := GetUserAttribute(user, emailAttr); err == nil {
			return recipientEmail, nil
		}
		logger.Debug("Email attribute not found in user entity", log.String("attribute", emailAttr))
	}

	return "", nil
}

// resolveTemplateData extracts template data from forwarded data or initializes an empty map if not present.
func (e *emailExecutor) resolveTemplateData(ctx *core.NodeContext) template.TemplateData {
	if ctx.ForwardedData != nil {
		if forwardedTemplateData, ok := ctx.ForwardedData[common.ForwardedDataKeyTemplateData]; ok {
			if interfaceData, isInterfaceMap := forwardedTemplateData.(map[string]interface{}); isInterfaceMap {
				templateData := template.TemplateData{}
				for k, v := range interfaceData {
					templateData[k] = fmt.Sprintf("%v", v)
				}
				return templateData
			}
		}
	}

	return template.TemplateData{}
}

// isEmailError returns true if the error originated from the email subsystem,
// covering both client-side validation errors and server-side SMTP transport errors.
func isEmailError(err error) bool {
	return errors.Is(err, email.ErrorInvalidRecipient) ||
		errors.Is(err, email.ErrorInvalidSender) ||
		errors.Is(err, email.ErrorInvalidSubject) ||
		errors.Is(err, email.ErrorInvalidHost) ||
		errors.Is(err, email.ErrorInvalidPort) ||
		errors.Is(err, email.ErrorInvalidCredentials) ||
		errors.Is(err, email.ErrorSMTPConnection) ||
		errors.Is(err, email.ErrorSMTPAuth) ||
		errors.Is(err, email.ErrorEmailSendFailed)
}

// resolveEmailInput returns the EMAIL_INPUT definition from the node context inputs,
// falling back to the default if none is found.
func (e *emailExecutor) resolveEmailInput(ctx *core.NodeContext) common.Input {
	for _, input := range ctx.NodeInputs {
		if input.Type == common.InputTypeEmail {
			return input
		}
	}
	return defaultEmailInput
}
