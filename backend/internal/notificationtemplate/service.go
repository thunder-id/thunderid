// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const loggerComponentName = "NotificationTemplateService"

// errNameConflict is an internal sentinel used to roll back a transaction when a name already exists;
// the caller surfaces ErrorTemplateNameConflict.
var errNameConflict = errors.New("template name already exists")

// NotificationTemplateServiceInterface defines the notification template management operations.
type NotificationTemplateServiceInterface interface {
	ListTemplates(ctx context.Context, channel string) (*TemplateListResponse, *tidcommon.ServiceError)
	CreateTemplate(ctx context.Context, channel string, request CreateTemplateRequest) (
		*Template, *tidcommon.ServiceError)
	GetTemplate(ctx context.Context, channel, id string) (*Template, *tidcommon.ServiceError)
	UpdateTemplate(ctx context.Context, channel, id string, request UpdateTemplateRequest) (
		*Template, *tidcommon.ServiceError)
	DeleteTemplate(ctx context.Context, channel, id string) *tidcommon.ServiceError
	SetDependencyRegistry(r resourcedependency.Registry)
}

// notificationTemplateService is the default implementation.
type notificationTemplateService struct {
	store              notificationTemplateStoreInterface
	transactioner      providers.Transactioner
	dependencyRegistry resourcedependency.Registry
	logger             *log.Logger
}

// newNotificationTemplateService creates a new service with the given store and transactioner.
func newNotificationTemplateService(store notificationTemplateStoreInterface,
	transactioner providers.Transactioner) NotificationTemplateServiceInterface {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	return &notificationTemplateService{
		store:         store,
		transactioner: transactioner,
		logger:        logger,
	}
}

// ListTemplates lists all templates of a channel.
func (ts *notificationTemplateService) ListTemplates(ctx context.Context, channel string) (
	*TemplateListResponse, *tidcommon.ServiceError) {
	if svcErr := validateChannel(channel); svcErr != nil {
		return nil, svcErr
	}

	templates, err := ts.store.ListTemplates(ctx, channel)
	if err != nil {
		ts.logger.Error(ctx, "Failed to list notification templates", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	summaries := make([]TemplateSummary, 0, len(templates))
	for _, t := range templates {
		summaries = append(summaries, TemplateSummary{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			Self:        buildSelf(channel, t.ID),
		})
	}

	return &TemplateListResponse{Templates: summaries}, nil
}

// CreateTemplate creates a new template with a server-assigned id. The name-uniqueness check and the
// insert run in one transaction so a concurrent create cannot slip a duplicate past the check.
func (ts *notificationTemplateService) CreateTemplate(ctx context.Context, channel string,
	request CreateTemplateRequest) (*Template, *tidcommon.ServiceError) {
	dao, svcErr := ts.toValidatedDAO(channel, "", request.Name, request.Description,
		request.Content, request.Design)
	if svcErr != nil {
		return nil, svcErr
	}

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		ts.logger.Error(ctx, "Failed to generate template id", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	dao.ID = id

	if svcErr := ts.persistUnique(ctx, channel, dao.Name, "", func(txCtx context.Context) error {
		return ts.store.CreateTemplate(txCtx, dao)
	}); svcErr != nil {
		return nil, svcErr
	}

	ts.logger.Debug(ctx, "Successfully created notification template", log.String("id", id))
	return daoToTemplate(channel, dao), nil
}

// GetTemplate retrieves a template by channel and id.
func (ts *notificationTemplateService) GetTemplate(ctx context.Context, channel, id string) (
	*Template, *tidcommon.ServiceError) {
	if svcErr := validateChannel(channel); svcErr != nil {
		return nil, svcErr
	}
	if id == "" {
		return nil, &ErrorInvalidTemplateID
	}

	dao, err := ts.store.GetTemplate(ctx, channel, id)
	if err != nil {
		if errors.Is(err, errTemplateNotFound) {
			return nil, &ErrorTemplateNotFound
		}
		ts.logger.Error(ctx, "Failed to retrieve notification template", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return daoToTemplate(channel, dao), nil
}

// UpdateTemplate updates an existing template. Existence, name-uniqueness, and the write run in one
// transaction.
func (ts *notificationTemplateService) UpdateTemplate(ctx context.Context, channel, id string,
	request UpdateTemplateRequest) (*Template, *tidcommon.ServiceError) {
	if svcErr := validateChannel(channel); svcErr != nil {
		return nil, svcErr
	}
	if id == "" {
		return nil, &ErrorInvalidTemplateID
	}

	dao, svcErr := ts.toValidatedDAO(channel, id, request.Name, request.Description,
		request.Content, request.Design)
	if svcErr != nil {
		return nil, svcErr
	}

	var notFound bool
	if svcErr := ts.persistUnique(ctx, channel, dao.Name, id, func(txCtx context.Context) error {
		if _, err := ts.store.GetTemplate(txCtx, channel, id); err != nil {
			if errors.Is(err, errTemplateNotFound) {
				notFound = true
			}
			return err
		}
		return ts.store.UpdateTemplate(txCtx, dao)
	}); svcErr != nil {
		if notFound {
			return nil, &ErrorTemplateNotFound
		}
		return nil, svcErr
	}

	ts.logger.Debug(ctx, "Successfully updated notification template", log.String("id", id))
	return daoToTemplate(channel, dao), nil
}

// DeleteTemplate deletes a template, rejecting the delete with a conflict if a flow references it.
func (ts *notificationTemplateService) DeleteTemplate(ctx context.Context, channel, id string) *tidcommon.ServiceError {
	if svcErr := validateChannel(channel); svcErr != nil {
		return svcErr
	}
	if id == "" {
		return &ErrorInvalidTemplateID
	}

	if _, err := ts.store.GetTemplate(ctx, channel, id); err != nil {
		if errors.Is(err, errTemplateNotFound) {
			// Idempotent delete: absent template is treated as already deleted.
			return nil
		}
		ts.logger.Error(ctx, "Failed to check template existence", log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}

	// A template referenced by a flow cannot be deleted (see the notification templates design).
	if ts.dependencyRegistry != nil {
		resp, depErr := ts.dependencyRegistry.GetDependencies(
			ctx, resourcedependency.ResourceTypeNotificationTemplate, id)
		if depErr != nil {
			ts.logger.Error(ctx, "Failed to resolve template dependencies", log.String("id", id), log.Error(depErr))
			return &tidcommon.InternalServerError
		}
		if len(resourcedependency.BlockingUsages(resp)) > 0 {
			return &ErrorTemplateInUse
		}
	}

	if err := ts.store.DeleteTemplate(ctx, channel, id); err != nil {
		ts.logger.Error(ctx, "Failed to delete notification template", log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}

	ts.logger.Debug(ctx, "Successfully deleted notification template", log.String("id", id))
	return nil
}

// SetDependencyRegistry injects the dependency registry. Called by the service manager after the
// provider services are initialized to avoid a cyclic import.
func (ts *notificationTemplateService) SetDependencyRegistry(r resourcedependency.Registry) {
	ts.dependencyRegistry = r
}

// persistUnique runs write inside a transaction after checking that name is free in the channel
// (excluding excludeID). It maps a name collision to ErrorTemplateNameConflict and any other failure
// to an internal error; write may return errTemplateNotFound, which the caller distinguishes.
func (ts *notificationTemplateService) persistUnique(ctx context.Context, channel, name, excludeID string,
	write func(txCtx context.Context) error) *tidcommon.ServiceError {
	var nameConflict bool
	txErr := ts.transactioner.Transact(ctx, func(txCtx context.Context) error {
		taken, err := ts.store.IsNameExists(txCtx, channel, name, excludeID)
		if err != nil {
			return err
		}
		if taken {
			nameConflict = true
			return errNameConflict
		}
		return write(txCtx)
	})
	if txErr != nil {
		if nameConflict {
			return &ErrorTemplateNameConflict
		}
		// Race backstop: two concurrent creates can both pass the read pre-check; the loser's INSERT
		// then trips the UNIQUE (DEPLOYMENT_ID, CHANNEL, NAME) constraint. Map that to the documented
		// conflict instead of a 500.
		if isUniqueViolation(txErr) {
			return &ErrorTemplateNameConflict
		}
		if errors.Is(txErr, errTemplateNotFound) {
			// Surfaced to the caller, which translates it to a 404.
			return &tidcommon.InternalServerError
		}
		ts.logger.Error(ctx, "Failed to persist notification template", log.Error(txErr))
		return &tidcommon.InternalServerError
	}
	return nil
}

// toValidatedDAO validates the request through the channel handler and builds a templateDAO. Name and
// body are required for every channel; per-channel rules and canonicalization are delegated to the
// handler.
func (ts *notificationTemplateService) toValidatedDAO(channel, id, name, description string,
	content TemplateContent, design *TemplateDesign) (templateDAO, *tidcommon.ServiceError) {
	handler, svcErr := handlerFor(channel)
	if svcErr != nil {
		return templateDAO{}, svcErr
	}
	if name == "" {
		return templateDAO{}, &ErrorMissingName
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		return templateDAO{}, &ErrorNameTooLong
	}
	if utf8.RuneCountInString(description) > maxDescriptionLength {
		return templateDAO{}, &ErrorDescriptionTooLong
	}
	if content.Body == "" {
		return templateDAO{}, &ErrorMissingBodyKey
	}
	if svcErr := handler.validate(content, design); svcErr != nil {
		return templateDAO{}, svcErr
	}

	normContent, normDesign := handler.normalize(content, design)
	return templateDAO{
		ID:          id,
		Channel:     channel,
		Name:        name,
		Description: description,
		Content:     normContent,
		Design:      normDesign,
	}, nil
}

// daoToTemplate maps a store DAO to the API representation, including the self link. The design is
// surfaced exactly as stored; the channel handler already dropped it for channels without a design.
func daoToTemplate(channel string, dao templateDAO) *Template {
	return &Template{
		ID:          dao.ID,
		Name:        dao.Name,
		Description: dao.Description,
		Self:        buildSelf(channel, dao.ID),
		Design:      dao.Design,
		Content:     dao.Content,
	}
}

// buildSelf builds the relative URL of a template resource.
func buildSelf(channel, id string) string {
	return fmt.Sprintf("/notification-templates/%s/templates/%s", channel, id)
}
