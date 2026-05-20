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
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeNotificationSender = "notification_sender"
	paramTypNotificationSender     = "NotificationSender"
)

// notificationSenderExporter implements declarativeresource.ResourceExporter for notification senders.
type notificationSenderExporter struct {
	service NotificationSenderMgtSvcInterface
}

// newNotificationSenderExporter creates a new notification sender exporter.
func newNotificationSenderExporter(service NotificationSenderMgtSvcInterface) *notificationSenderExporter {
	return &notificationSenderExporter{service: service}
}

// NewNotificationSenderExporterForTest creates a new notification sender exporter for testing purposes.
func NewNotificationSenderExporterForTest(service NotificationSenderMgtSvcInterface) *notificationSenderExporter {
	if !testing.Testing() {
		panic("only for tests!")
	}
	return newNotificationSenderExporter(service)
}

// GetResourceType returns the resource type for notification senders.
func (e *notificationSenderExporter) GetResourceType() string {
	return resourceTypeNotificationSender
}

// GetParameterizerType returns the parameterizer type for notification senders.
func (e *notificationSenderExporter) GetParameterizerType() string {
	return paramTypNotificationSender
}

// GetAllResourceIDs retrieves all notification sender IDs.
func (e *notificationSenderExporter) GetAllResourceIDs(ctx context.Context) ([]string, *serviceerror.ServiceError) {
	senders, err := e.service.ListSenders(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(senders))
	for _, sender := range senders {
		ids = append(ids, sender.ID)
	}
	return ids, nil
}

// GetResourceByID retrieves a notification sender by its ID.
func (e *notificationSenderExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *serviceerror.ServiceError,
) {
	sender, err := e.service.GetSender(ctx, id)
	if err != nil {
		return nil, "", err
	}
	return sender, sender.Name, nil
}

// ValidateResource validates a notification sender resource.
func (e *notificationSenderExporter) ValidateResource(
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	sender, ok := resource.(*common.NotificationSenderDTO)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeNotificationSender, id)
	}

	err := declarativeresource.ValidateResourceName(
		sender.Name, resourceTypeNotificationSender, id, "SENDER_VALIDATION_ERROR", logger,
	)
	if err != nil {
		return "", err
	}

	if len(sender.Properties) == 0 {
		logger.Warn("Notification sender has no properties",
			log.String("senderID", id), log.String("name", sender.Name))
	}

	return sender.Name, nil
}

// GetResourceRules returns the parameterization rules for notification senders.
func (e *notificationSenderExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{
		DynamicPropertyFields: []string{"Properties"},
	}
}

// loadDeclarativeResources loads declarative notification sender resources from files.
func loadDeclarativeResources(notificationStore notificationStoreInterface) error {
	// Type assert to access Storer interface for resource loading
	fileBasedStore, ok := notificationStore.(*notificationFileBasedStore)
	if !ok {
		return fmt.Errorf("failed to assert notificationStore to *notificationFileBasedStore")
	}

	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "NotificationSender",
		DirectoryName: "notification_senders",
		Parser:        parseToNotificationSenderDTOWrapper,
		Validator:     validateNotificationSenderWrapper,
		IDExtractor: func(data interface{}) string {
			return data.(*common.NotificationSenderDTO).ID
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, fileBasedStore)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load notification sender resources: %w", err)
	}

	return nil
}

// parseToNotificationSenderDTOWrapper wraps parseToNotificationSenderDTO to match ResourceConfig.Parser signature.
func parseToNotificationSenderDTOWrapper(data []byte) (interface{}, error) {
	return parseToNotificationSenderDTO(data)
}

func parseToNotificationSenderDTO(data []byte) (*common.NotificationSenderDTO, error) {
	var senderRequest common.NotificationSenderRequestWithID
	err := yaml.Unmarshal(data, &senderRequest)
	if err != nil {
		return nil, err
	}

	senderDTO := &common.NotificationSenderDTO{
		ID:          senderRequest.ID,
		Name:        senderRequest.Name,
		Description: senderRequest.Description,
		Type:        common.NotificationSenderTypeMessage,
	}

	// Parse provider type
	provider, err := parseProviderType(senderRequest.Provider)
	if err != nil {
		return nil, err
	}
	senderDTO.Provider = provider

	// Convert PropertyDTO to Property
	if len(senderRequest.Properties) > 0 {
		properties := make([]cmodels.Property, 0, len(senderRequest.Properties))
		for _, propDTO := range senderRequest.Properties {
			prop, err := cmodels.NewProperty(propDTO.Name, propDTO.Value, propDTO.IsSecret)
			if err != nil {
				return nil, err
			}
			properties = append(properties, *prop)
		}
		senderDTO.Properties = properties
	}

	return senderDTO, nil
}

func parseProviderType(providerStr string) (common.MessageProviderType, error) {
	// Convert string to lowercase for case-insensitive matching
	providerStrLower := common.MessageProviderType(strings.ToLower(providerStr))

	// Check if it's a valid provider
	supportedProviders := []common.MessageProviderType{
		common.MessageProviderTypeVonage,
		common.MessageProviderTypeTwilio,
		common.MessageProviderTypeCustom,
	}

	for _, supportedProvider := range supportedProviders {
		if supportedProvider == providerStrLower {
			return supportedProvider, nil
		}
	}

	return "", fmt.Errorf("unsupported provider type: %s", providerStr)
}

// validateNotificationSenderWrapper wraps validateNotificationSender to match ResourceConfig.Validator signature.
func validateNotificationSenderWrapper(dto interface{}) error {
	senderDTO, ok := dto.(*common.NotificationSenderDTO)
	if !ok {
		return fmt.Errorf("invalid type: expected *common.NotificationSenderDTO")
	}
	return validateNotificationSenderForDeclarativeResource(senderDTO)
}

func validateNotificationSenderForDeclarativeResource(senderDTO *common.NotificationSenderDTO) error {
	if strings.TrimSpace(senderDTO.Name) == "" {
		return fmt.Errorf("notification sender name is required")
	}

	if strings.TrimSpace(senderDTO.ID) == "" {
		return fmt.Errorf("notification sender ID is required")
	}

	if senderDTO.Type == "" {
		return fmt.Errorf("notification sender type is required for '%s'", senderDTO.Name)
	}

	return nil
}
