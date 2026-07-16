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

// Package notification contains the implementation of notification and otp sender services.
package notification

import (
	"context"
	"errors"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/notification/common"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// NotificationSenderMgtSvcInterface defines the interface for managing notification senders.
type NotificationSenderMgtSvcInterface interface {
	CreateSender(ctx context.Context, sender common.NotificationSenderDTO) (*common.NotificationSenderDTO,
		*tidcommon.ServiceError)
	ListSenders(ctx context.Context) ([]common.NotificationSenderDTO, *tidcommon.ServiceError)
	ListSendersByType(ctx context.Context, senderType common.NotificationSenderType) ([]common.NotificationSenderDTO,
		*tidcommon.ServiceError)
	GetSender(ctx context.Context, id string) (*common.NotificationSenderDTO, *tidcommon.ServiceError)
	GetSenderByName(ctx context.Context, name string) (*common.NotificationSenderDTO, *tidcommon.ServiceError)
	UpdateSender(ctx context.Context, id string, sender common.NotificationSenderDTO) (*common.NotificationSenderDTO,
		*tidcommon.ServiceError)
	DeleteSender(ctx context.Context, id string) *tidcommon.ServiceError
	SetDependencyRegistry(r resourcedependency.Registry)
}

// notificationSenderMgtService implements the NotificationSenderMgtSvcInterface.
type notificationSenderMgtService struct {
	notificationStore  notificationStoreInterface
	transactioner      transaction.Transactioner
	dependencyRegistry resourcedependency.Registry
	uuidGenerator      func() (string, error)
}

// newNotificationSenderMgtService returns a new instance of NotificationSenderMgtSvcInterface.
func newNotificationSenderMgtService(
	store notificationStoreInterface, tx transaction.Transactioner) NotificationSenderMgtSvcInterface {
	return &notificationSenderMgtService{
		notificationStore: store,
		transactioner:     tx,
		uuidGenerator:     sysutils.GenerateUUIDv7,
	}
}

// CreateSender creates a new notification sender.
func (s *notificationSenderMgtService) CreateSender(
	ctx context.Context, sender common.NotificationSenderDTO) (
	*common.NotificationSenderDTO, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationSenderMgtService"))
	logger.Debug(ctx, "Creating notification sender", log.String("name", sender.Name))

	if err := declarativeresource.CheckDeclarativeCreate(); err != nil {
		return nil, err
	}

	if err := validateNotificationSender(sender); err != nil {
		return nil, err
	}

	applyDefaultSenderProperties(&sender)

	if sender.ID == "" {
		id, err := s.uuidGenerator()
		if err != nil {
			logger.Error(ctx, "Failed to generate UUID", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		sender.ID = id
	}

	var svcErr *tidcommon.ServiceError
	transactErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		// Check if sender with same name already exists
		senderRetv, err := s.notificationStore.getSenderByName(txCtx, sender.Name)
		if err != nil {
			return err
		}
		if senderRetv != nil {
			logger.Debug(ctx, "Notification sender already exists", log.String("name", sender.Name),
				log.String("id", senderRetv.ID))
			svcErr = &ErrorDuplicateSenderName
			return errors.New("sender already exists")
		}

		// Create the sender
		err = s.notificationStore.createSender(txCtx, sender)
		if err != nil {
			return err
		}
		return nil
	})

	if svcErr != nil {
		return nil, svcErr
	}
	if transactErr != nil {
		logger.Error(ctx, "Failed to create notification sender",
			log.Error(transactErr), log.String("name", sender.Name))
		return nil, &tidcommon.InternalServerError
	}

	return &common.NotificationSenderDTO{
		ID:          sender.ID,
		Name:        sender.Name,
		Description: sender.Description,
		Type:        sender.Type,
		Provider:    sender.Provider,
		Properties:  sender.Properties,
	}, nil
}

// ListSenders retrieves all notification senders.
func (s *notificationSenderMgtService) ListSenders(ctx context.Context) ([]common.NotificationSenderDTO,
	*tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationSenderMgtService"))
	logger.Debug(ctx, "Listing all notification senders")

	senders, err := s.notificationStore.listSenders(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to list notification senders", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return senders, nil
}

// ListSendersByType retrieves all notification senders of the given type (e.g. only
// message/SMS senders, excluding email senders).
func (s *notificationSenderMgtService) ListSendersByType(
	ctx context.Context, senderType common.NotificationSenderType,
) ([]common.NotificationSenderDTO, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationSenderMgtService"))
	logger.Debug(ctx, "Listing notification senders by type", log.String("type", string(senderType)))

	senders, err := s.notificationStore.listSendersByType(ctx, senderType)
	if err != nil {
		logger.Error(ctx, "Failed to list notification senders by type", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return senders, nil
}

// GetSender retrieves a notification sender by ID.
func (s *notificationSenderMgtService) GetSender(ctx context.Context, id string) (*common.NotificationSenderDTO,
	*tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationSenderMgtService"))
	logger.Debug(ctx, "Retrieving notification sender", log.String("id", id))

	if id == "" {
		return nil, &ErrorInvalidSenderID
	}

	sender, err := s.notificationStore.getSenderByID(ctx, id)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve notification sender", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	if sender == nil {
		return nil, &ErrorSenderNotFound
	}

	return sender, nil
}

// GetSenderByName retrieves a notification sender by name.
func (s *notificationSenderMgtService) GetSenderByName(ctx context.Context, name string) (*common.NotificationSenderDTO,
	*tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationSenderMgtService"))
	logger.Debug(ctx, "Retrieving notification sender by name", log.String("name", name))

	if name == "" {
		return nil, &ErrorInvalidSenderName
	}

	sender, err := s.notificationStore.getSenderByName(ctx, name)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve notification sender", log.String("name", name), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	if sender == nil {
		return nil, &ErrorSenderNotFound
	}

	return sender, nil
}

// UpdateSender updates an existing notification sender
func (s *notificationSenderMgtService) UpdateSender(ctx context.Context, id string,
	sender common.NotificationSenderDTO) (*common.NotificationSenderDTO, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationSenderMgtService"))
	logger.Debug(ctx, "Updating notification sender", log.String("id", id), log.String("name", sender.Name))

	if err := declarativeresource.CheckDeclarativeUpdate(); err != nil {
		return nil, err
	}

	if id == "" {
		return nil, &ErrorInvalidSenderID
	}
	if err := validateNotificationSender(sender); err != nil {
		return nil, err
	}

	applyDefaultSenderProperties(&sender)

	var svcErr *tidcommon.ServiceError
	transactErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		// Check if sender exists
		senderRetv, err := s.notificationStore.getSenderByID(txCtx, id)
		if err != nil {
			return err
		}
		if senderRetv == nil {
			logger.Debug(ctx, "Notification sender not found", log.String("id", id))
			svcErr = &ErrorSenderNotFound
			return errors.New("sender not found")
		}

		// If the name is being updated, check for duplicates
		if sender.Name != senderRetv.Name {
			senderWithUpdatedName, err := s.notificationStore.getSenderByName(txCtx, sender.Name)
			if err != nil {
				return err
			}
			if senderWithUpdatedName != nil && senderWithUpdatedName.ID != id {
				logger.Debug(ctx, "Another sender with the same name already exists",
					log.String("name", sender.Name), log.String("existingID", senderWithUpdatedName.ID))
				svcErr = &ErrorDuplicateSenderName
				return errors.New("duplicate name")
			}
		}

		// Ensure the type is not changed
		if sender.Type != senderRetv.Type {
			logger.Debug(ctx, "Attempting to change sender type", log.String("id", id),
				log.String("originalType", string(senderRetv.Type)), log.String("newType", string(sender.Type)))
			svcErr = &ErrorSenderTypeUpdateNotAllowed
			return errors.New("cannot change type")
		}

		// Update the sender
		if err := s.notificationStore.updateSender(txCtx, id, sender); err != nil {
			return err
		}

		return nil
	})

	if svcErr != nil {
		return nil, svcErr
	}
	if transactErr != nil {
		logger.Error(ctx, "Failed to update notification sender",
			log.Error(transactErr), log.String("id", id))
		return nil, &tidcommon.InternalServerError
	}

	return &common.NotificationSenderDTO{
		ID:          id,
		Name:        sender.Name,
		Description: sender.Description,
		Type:        sender.Type,
		Provider:    sender.Provider,
		Properties:  sender.Properties,
	}, nil
}

// DeleteSender deletes a notification sender
func (s *notificationSenderMgtService) DeleteSender(ctx context.Context, id string) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationSenderMgtService"))
	logger.Debug(ctx, "Deleting notification sender", log.String("id", id))

	if err := declarativeresource.CheckDeclarativeDelete(); err != nil {
		return err
	}

	if id == "" {
		return &ErrorInvalidSenderID
	}

	// Refuse deletion while other resources block it (e.g. flows that reference the sender).
	if svcErr := s.ensureNoBlockingDependencies(ctx, id, logger); svcErr != nil {
		return svcErr
	}

	transactErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := s.notificationStore.deleteSender(txCtx, id); err != nil {
			return err
		}
		return nil
	})

	if transactErr != nil {
		logger.Error(ctx, "Failed to delete notification sender",
			log.Error(transactErr), log.String("id", id))
		return &tidcommon.InternalServerError
	}

	return nil
}

// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (s *notificationSenderMgtService) SetDependencyRegistry(r resourcedependency.Registry) {
	s.dependencyRegistry = r
}

// ensureNoBlockingDependencies refuses deletion when other resources depend on the notification
// sender in a way that forbids it (behaviorOnDelete == restrict), such as flows that reference it.
// Because deletion is destructive, it fails closed: if dependency data cannot be determined, the
// deletion is refused rather than allowed.
func (s *notificationSenderMgtService) ensureNoBlockingDependencies(
	ctx context.Context, id string, logger *log.Logger) *tidcommon.ServiceError {
	if s.dependencyRegistry == nil {
		logger.Error(ctx, "Dependency registry not set; refusing to delete notification sender",
			log.String("id", id))
		return &tidcommon.InternalServerError
	}

	deps, err := s.dependencyRegistry.GetDependencies(ctx, resourcedependency.ResourceTypeNotificationSender, id)
	if err != nil {
		logger.Error(ctx, "Failed to evaluate notification sender dependencies",
			log.Error(err), log.String("id", id))
		return &tidcommon.InternalServerError
	}
	// Fail closed: nil TotalResults means a provider failed to report, so usage is unknown.
	if deps == nil || deps.TotalResults == nil {
		logger.Error(ctx, "Notification sender dependency data unavailable; refusing to delete",
			log.String("id", id))
		return &tidcommon.InternalServerError
	}

	blocking := resourcedependency.BlockingUsages(deps)
	if len(blocking) == 0 {
		return nil
	}

	logger.Debug(ctx, "Notification sender has blocking dependencies; deletion refused",
		log.String("id", id), log.Int("blockingCount", len(blocking)))
	return tidcommon.CustomServiceError(ErrorSenderHasBlockingDependencies, tidcommon.I18nMessage{
		Key: "error.notificationservice.sender_has_blocking_dependencies_description",
		DefaultValue: fmt.Sprintf(
			"The notification sender cannot be deleted because %s depend on it. Remove or reassign them first.",
			resourcedependency.SummarizeBlockingUsages(blocking)),
	})
}
