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

// Package consent provides a pluggable consent management abstraction
package consent

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// consentService is the default implementation of ConsentServiceInterface.
type consentService struct {
	enabled bool
	client  consentClientInterface
	logger  *log.Logger
}

// newConsentService creates a new instance of consentService with the given client.
func newConsentService(client consentClientInterface) ConsentServiceInterface {
	isEnabled := config.GetServerRuntime().Config.Consent.Enabled
	if !isEnabled {
		log.GetLogger().Debug("Consent service is disabled in the configuration")
	}

	return &consentService{
		enabled: isEnabled,
		client:  client,
		logger:  log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ConsentService")),
	}
}

// IsEnabled returns true if the consent service is enabled, false otherwise.
func (s *consentService) IsEnabled() bool {
	return s.enabled
}

// ----- Consent element operations -----

// CreateConsentElements creates one or more consent elements.
func (s *consentService) CreateConsentElements(ctx context.Context, ouID string,
	elements []ConsentElementInput) ([]ConsentElement, *serviceerror.ServiceError) {
	if len(elements) == 0 {
		return nil, nil
	}
	return s.client.createConsentElements(ctx, ouID, elements)
}

// ListConsentElements retrieves consent elements, optionally filtered by namespace and name
func (s *consentService) ListConsentElements(ctx context.Context, ouID string, ns Namespace,
	nameFilter string) ([]ConsentElement, *serviceerror.ServiceError) {
	return s.client.listConsentElements(ctx, ouID, ns, nameFilter)
}

// UpdateConsentElement updates an existing consent element by ID.
func (s *consentService) UpdateConsentElement(ctx context.Context, ouID string,
	elementID string, element *ConsentElementInput) (*ConsentElement, *serviceerror.ServiceError) {
	if element == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	return s.client.updateConsentElement(ctx, ouID, elementID, element)
}

// DeleteConsentElement deletes a consent element by ID.
// Returns nil if the element does not exist (idempotent).
func (s *consentService) DeleteConsentElement(ctx context.Context, ouID string,
	elementID string) *serviceerror.ServiceError {
	svcErr := s.client.deleteConsentElement(ctx, ouID, elementID)
	if svcErr != nil && svcErr.Code == ErrorConsentElementNotFound.Code {
		s.logger.Debug("Consent element not found during delete, skipping",
			log.String("elementID", elementID))
		return nil
	}
	return svcErr
}

// ValidateConsentElements validates a list of consent element names and returns the valid ones.
func (s *consentService) ValidateConsentElements(ctx context.Context, ouID string, names []string) (
	[]string, *serviceerror.ServiceError) {
	if len(names) == 0 {
		return []string{}, nil
	}
	return s.client.validateConsentElements(ctx, ouID, names)
}

// CreateConsentPurpose creates a consent purpose for a resource.
func (s *consentService) CreateConsentPurpose(ctx context.Context, ouID string, purpose *ConsentPurposeInput) (
	*ConsentPurpose, *serviceerror.ServiceError) {
	if purpose == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	return s.client.createConsentPurpose(ctx, ouID, purpose)
}

// ListConsentPurposes retrieves consent purposes for a resource.
func (s *consentService) ListConsentPurposes(ctx context.Context, ouID, groupID string) (
	[]ConsentPurpose, *serviceerror.ServiceError) {
	return s.client.listConsentPurposes(ctx, ouID, groupID)
}

// UpdateConsentPurpose updates an existing consent purpose by ID.
func (s *consentService) UpdateConsentPurpose(ctx context.Context, ouID, purposeID string,
	purpose *ConsentPurposeInput) (*ConsentPurpose, *serviceerror.ServiceError) {
	if purpose == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	return s.client.updateConsentPurpose(ctx, ouID, purposeID, purpose)
}

// DeleteConsentPurpose deletes a consent purpose by ID.
// Returns nil if the purpose does not exist (idempotent).
func (s *consentService) DeleteConsentPurpose(ctx context.Context, ouID string,
	purposeID string) *serviceerror.ServiceError {
	svcErr := s.client.deleteConsentPurpose(ctx, ouID, purposeID)
	if svcErr != nil && svcErr.Code == ErrorConsentPurposeNotFound.Code {
		s.logger.Debug("Consent purpose not found during delete, skipping",
			log.String("purposeID", purposeID))
		return nil
	}
	return svcErr
}

// CreateConsent creates a new consent record.
func (s *consentService) CreateConsent(ctx context.Context, ouID string, consent *ConsentRequest) (
	*Consent, *serviceerror.ServiceError) {
	if consent == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	return s.client.createConsent(ctx, ouID, consent)
}

// SearchConsents searches consent records matching the filter.
func (s *consentService) SearchConsents(ctx context.Context, ouID string, filter *ConsentSearchFilter) (
	[]Consent, *serviceerror.ServiceError) {
	return s.client.searchConsents(ctx, ouID, filter)
}

// ValidateConsent validates a consent by ID and returns validation details.
func (s *consentService) ValidateConsent(ctx context.Context, ouID, consentID string) (
	*ConsentValidationResult, *serviceerror.ServiceError) {
	return s.client.validateConsent(ctx, ouID, consentID)
}

// UpdateConsent updates the content of an existing consent record.
func (s *consentService) UpdateConsent(ctx context.Context, ouID string, consentID string,
	consent *ConsentRequest) (*Consent, *serviceerror.ServiceError) {
	if consent == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	return s.client.updateConsent(ctx, ouID, consentID, consent)
}

// RevokeConsent revokes an active consent record.
func (s *consentService) RevokeConsent(ctx context.Context, ouID, consentID string,
	payload *ConsentRevokeRequest) *serviceerror.ServiceError {
	if payload == nil {
		return &ErrorInvalidRequestFormat
	}
	return s.client.revokeConsent(ctx, ouID, consentID, payload)
}
