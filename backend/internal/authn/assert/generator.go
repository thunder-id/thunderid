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

// Package assert provides functionality to generate and verify authentication assertions with support
// for authentication assurance levels (AAL, IAL).
package assert

import (
	"context"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// TODO: Refactor this to be a centralized auth assertion generator with appropriate token generation logics.

const loggerComponentName = "AuthAssertGenerator"

// AuthAssertGeneratorInterface defines the interface for generating auth assertion claims.
type AuthAssertGeneratorInterface interface {
	GenerateAssertion(ctx context.Context, authenticators []authncm.AuthenticatorReference) (*AssertionResult,
		*serviceerror.ServiceError)
	UpdateAssertion(ctx context.Context, context *AssuranceContext, authenticator authncm.AuthenticatorReference) (
		*AssertionResult, *serviceerror.ServiceError)
	VerifyAssurance(ctx context.Context,
		context *AssuranceContext, requiredAAL AssuranceLevel, requiredIAL AssuranceLevel) (
		bool, *serviceerror.ServiceError)
}

// authAssertGenerator implements the AuthAssertGeneratorInterface.
type authAssertGenerator struct{}

// newAuthAssertGenerator creates a new instance of AuthAssertGeneratorInterface.
func newAuthAssertGenerator() AuthAssertGeneratorInterface {
	return &authAssertGenerator{}
}

// GenerateAssertion generates authenticator assertion based on the provided authenticators.
func (ag *authAssertGenerator) GenerateAssertion(ctx context.Context,
	authenticators []authncm.AuthenticatorReference) (*AssertionResult, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Generating authentication assertion")

	if len(authenticators) == 0 {
		logger.Debug(ctx, "No authenticators provided for assertion generation")
		return nil, &ErrorNoAuthenticators
	}

	_, factorSet := ag.extractUniqueAuthenticators(ctx, authenticators, logger)
	aal := ag.calculateAAL(ctx, factorSet, logger)
	ial := ag.calculateIAL()

	return &AssertionResult{
		Context: &AssuranceContext{
			AAL:            aal,
			IAL:            ial,
			Authenticators: authenticators,
		},
	}, nil
}

// UpdateAssertion updates existing assurance context with the provided authenticator.
func (ag *authAssertGenerator) UpdateAssertion(ctx context.Context, context *AssuranceContext,
	authenticator authncm.AuthenticatorReference) (*AssertionResult, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Updating authentication assertion with new authenticator")

	if context == nil {
		logger.Debug(ctx, "No existing assurance context found, generating new assertion")
		return ag.GenerateAssertion(ctx, []authncm.AuthenticatorReference{authenticator})
	}

	// Validate authenticator name is present
	if authenticator.Authenticator == "" {
		logger.Debug(ctx, "Invalid authenticator: missing authenticator name")
		return nil, &ErrorInvalidAuthenticator
	}

	// Merge authenticators
	allAuthenticators := make([]authncm.AuthenticatorReference, 0, len(context.Authenticators)+1)
	allAuthenticators = append(allAuthenticators, context.Authenticators...)
	allAuthenticators = append(allAuthenticators, authenticator)

	// Regenerate claims with all authenticators
	return ag.GenerateAssertion(ctx, allAuthenticators)
}

// VerifyAssurance verifies if actual assurance meets the required assurance level.
func (ag *authAssertGenerator) VerifyAssurance(
	ctx context.Context, context *AssuranceContext, requiredAAL AssuranceLevel,
	requiredIAL AssuranceLevel) (bool, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Verifying assurance levels")

	if context == nil {
		logger.Debug(ctx, "Nil assurance context provided")
		return false, &ErrorNilAssuranceContext
	}
	if requiredAAL == "" && requiredIAL == "" {
		logger.Debug(ctx, "No assurance levels specified for verification")
		return false, &ErrorNoAssuranceRequirements
	}

	// Check AAL level
	if requiredAAL != "" && !ag.meetsAssuranceLevel(context.AAL, requiredAAL) {
		logger.Debug(ctx, "Actual AAL does not meet required AAL",
			log.String("actualAAL", string(context.AAL)),
			log.String("requiredAAL", string(requiredAAL)))
		return false, nil
	}

	// Check IAL level
	if requiredIAL != "" && !ag.meetsAssuranceLevel(context.IAL, requiredIAL) {
		logger.Debug(ctx, "Actual IAL does not meet required IAL",
			log.String("actualIAL", string(context.IAL)),
			log.String("requiredIAL", string(requiredIAL)))
		return false, nil
	}

	return true, nil
}

// extractUniqueAuthenticators extracts unique authenticators and factors from authenticator references.
// Returns slices of unique authenticator names and authentication factors.
func (ag *authAssertGenerator) extractUniqueAuthenticators(
	ctx context.Context, authenticators []authncm.AuthenticatorReference,
	logger *log.Logger) ([]string, []authncm.AuthenticationFactor) {
	authenticatorsMap := make(map[string]bool)
	factorSet := make(map[authncm.AuthenticationFactor]bool)

	for _, auth := range authenticators {
		authenticatorsMap[auth.Authenticator] = true

		factors := authncm.GetAuthenticatorFactors(auth.Authenticator)
		if len(factors) == 0 {
			logger.Debug(ctx, "No factors found for authenticator. Skipping",
				log.String("authenticator", auth.Authenticator))
			continue
		}

		for _, factor := range factors {
			factorSet[factor] = true
		}
	}

	// Convert maps to slices
	uniqueAuthenticators := make([]string, 0, len(authenticatorsMap))
	for authName := range authenticatorsMap {
		uniqueAuthenticators = append(uniqueAuthenticators, authName)
	}

	uniqueFactors := make([]authncm.AuthenticationFactor, 0, len(factorSet))
	for factor := range factorSet {
		uniqueFactors = append(uniqueFactors, factor)
	}

	return uniqueAuthenticators, uniqueFactors
}

// calculateAAL calculates the AAL based on the authentication factors.
// - UNKNOWN: No valid authentication factors found
// - AAL1: Single-factor authentication (any one factor)
// - AAL2: Two-factor authentication (two different factors)
// - AAL3: Multi-factor authentication with hardware-based cryptographic authenticator
func (ag *authAssertGenerator) calculateAAL(ctx context.Context, factorSet []authncm.AuthenticationFactor,
	logger *log.Logger) AssuranceLevel {
	var aal AssuranceLevel
	factorCount := len(factorSet)

	switch factorCount {
	case 0:
		aal = AALUnknown
	case 1:
		aal = AALLevel1
	case 2:
		aal = AALLevel2
	default:
		aal = AALLevel3
	}

	logger.Debug(ctx, "Calculated AAL from authentication factors", log.Any("factors", factorSet),
		log.String("aal", string(aal)))

	return aal
}

// calculateIAL calculates the IAL based on authenticators.
// For now, returns default IAL1. Can be enhanced based on user verification status.
func (ag *authAssertGenerator) calculateIAL() AssuranceLevel {
	// Default implementation - can be enhanced to check user verification status
	// For example, check if email/phone is verified, document verification, etc.
	return IALLevel1
}

// meetsAssuranceLevel checks if actual assurance level meets or exceeds the required level.
func (ag *authAssertGenerator) meetsAssuranceLevel(actual, required AssuranceLevel) bool {
	return actual.Level() >= required.Level()
}
