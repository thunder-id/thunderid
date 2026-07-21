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

package interceptor

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// captchaInterceptor validates a captcha token submitted with a node's user inputs on PRE_NODE,
// delegating verification to a CaptchaServiceInterface.
type captchaInterceptor struct {
	core.InterceptorInterface
	captchaService providers.CaptchaValidationProvider
	logger         *log.Logger
}

var _ core.InterceptorInterface = (*captchaInterceptor)(nil)

// captchaInterceptorInputs declares the inputs consumed by the captcha interceptor. The captcha
// token is one-time-use: the captcha provider invalidates it on first verification, so the engine
// clears it from the flow context once it has been consumed.
var captchaInterceptorInputs = []providers.Input{
	{
		Identifier: captchaTokenFieldKey,
		Type:       providers.InputTypeHidden,
		OneTimeUse: true,
	},
}

// newCaptchaInterceptor creates a new captcha interceptor.
func newCaptchaInterceptor(flowFactory core.FlowFactoryInterface,
	captchaService providers.CaptchaValidationProvider) *captchaInterceptor {
	base := flowFactory.CreateInterceptor(CaptchaInterceptor, false, BasePriorityConfigurable)

	return &captchaInterceptor{
		InterceptorInterface: base,
		captchaService:       captchaService,
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, CaptchaInterceptor)),
	}
}

// GetInputs returns the inputs declared by the captcha interceptor.
func (c *captchaInterceptor) GetInputs() []providers.Input {
	return captchaInterceptorInputs
}

// Execute delegates to the appropriate handler based on the interceptor mode.
func (c *captchaInterceptor) Execute(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
	switch ctx.Mode {
	case providers.InterceptorModePreNode:
		return c.validateCaptchaToken(ctx)
	default:
		return &common.InterceptorResponse{
			Status: common.InterceptorStatusFailure,
		}, nil
	}
}

// validateCaptchaToken validates the captcha token carried in the node's user inputs. An invalid
// token fails with a client error; an operational verification failure is surfaced as an execution
// error so the engine rejects the request with a server error.
func (c *captchaInterceptor) validateCaptchaToken(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
	token, _ := ctx.ConsumeInput(captchaTokenFieldKey)
	if token == "" {
		return &common.InterceptorResponse{
			Status: common.InterceptorStatusFailure,
			Error:  &ErrorCaptchaInvalid,
		}, nil
	}
	result, svcErr := c.captchaService.Verify(ctx.Context, token)
	if svcErr != nil {
		return nil, fmt.Errorf("captcha verification failed: %s", svcErr.Code)
	}
	if result == nil || !result.Success {
		c.logger.Debug(ctx.Context, "Captcha token verification returned a negative verdict",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
		return &common.InterceptorResponse{
			Status: common.InterceptorStatusFailure,
			Error:  &ErrorCaptchaInvalid,
		}, nil
	}

	return &common.InterceptorResponse{
		Status: common.InterceptorStatusComplete,
	}, nil
}
