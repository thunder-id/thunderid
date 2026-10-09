// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package notificationtemplate manages the notification templates ThunderID sends.
// A template is language-neutral: its subject and body are template strings that embed placeholders
// resolved at render time — {{t(key)}} for localized text, {{ctx(var)}} for flow-context values, and
// {{design(token)}} for design tokens. The module is channel-generic; per-channel rules live behind
// channelRules (see channel.go).
package notificationtemplate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// NotificationTemplateServiceInterface defines the notification template management operations.
type NotificationTemplateServiceInterface interface {
	ListTemplates(ctx context.Context, channel ChannelType, limit, offset int) (
		*TemplateListResponse, *tidcommon.ServiceError)
	CreateTemplate(ctx context.Context, channel ChannelType, request CreateTemplateRequest) (
		*Template, *tidcommon.ServiceError)
	GetTemplate(ctx context.Context, channel ChannelType, id string) (*Template, *tidcommon.ServiceError)
	GetTemplateByHandle(ctx context.Context, channel ChannelType, handle string) (
		*Template, *tidcommon.ServiceError)
	ValidateTemplate(ctx context.Context, channel ChannelType, request CreateTemplateRequest) *tidcommon.ServiceError
	UpdateTemplate(ctx context.Context, channel ChannelType, id string, request UpdateTemplateRequest) (
		*Template, *tidcommon.ServiceError)
	DeleteTemplate(ctx context.Context, channel ChannelType, id string) *tidcommon.ServiceError
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
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationTemplateService"))
	return &notificationTemplateService{
		store:         store,
		transactioner: transactioner,
		logger:        logger,
	}
}

// ListTemplates lists a page of templates of a channel.
func (ts *notificationTemplateService) ListTemplates(ctx context.Context, channel ChannelType, limit, offset int) (
	*TemplateListResponse, *tidcommon.ServiceError) {
	if svcErr := validateChannel(channel); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := validatePagination(limit, offset); svcErr != nil {
		return nil, svcErr
	}

	total, err := ts.store.CountTemplates(ctx, channel)
	if err != nil {
		ts.logger.Error(ctx, "Failed to count notification templates", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	templates, err := ts.store.ListTemplates(ctx, channel, limit, offset)
	if err != nil {
		ts.logger.Error(ctx, "Failed to list notification templates", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	summaries := make([]TemplateSummary, 0, len(templates))
	for _, t := range templates {
		summaries = append(summaries, TemplateSummary{
			ID:          t.ID,
			Handle:      t.Handle,
			DisplayName: t.DisplayName,
			Description: t.Description,
			Self:        buildSelf(channel, t.ID),
		})
	}

	return &TemplateListResponse{
		TotalResults: total,
		StartIndex:   offset + 1,
		Count:        len(summaries),
		Templates:    summaries,
		Links:        utils.BuildPaginationLinks(buildCollectionSelf(channel), limit, offset, total, ""),
	}, nil
}

// CreateTemplate creates a new template with a server-assigned id. The handle-uniqueness check and the
// insert run in one transaction so a concurrent create cannot slip a duplicate past the check.
func (ts *notificationTemplateService) CreateTemplate(ctx context.Context, channel ChannelType,
	request CreateTemplateRequest) (*Template, *tidcommon.ServiceError) {
	if svcErr := validateChannel(channel); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := validateHandle(request.Handle); svcErr != nil {
		return nil, svcErr
	}

	dao, svcErr := buildValidatedDAO(channel, "", request.DisplayName, request.Description,
		request.Content, request.Design)
	if svcErr != nil {
		return nil, svcErr
	}
	dao.Handle = request.Handle

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		ts.logger.Error(ctx, "Failed to generate template id", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	dao.ID = id

	if svcErr := ts.persistUniqueHandle(ctx, channel, dao.Handle, func(txCtx context.Context) error {
		return ts.store.CreateTemplate(txCtx, dao)
	}); svcErr != nil {
		return nil, svcErr
	}

	ts.logger.Debug(ctx, "Successfully created notification template", log.String("id", id))
	return daoToTemplate(dao), nil
}

// GetTemplate retrieves a template by channel and id.
func (ts *notificationTemplateService) GetTemplate(ctx context.Context, channel ChannelType, id string) (
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

	return daoToTemplate(dao), nil
}

// GetTemplateByHandle retrieves a template by channel and handle. Handles are immutable and unique per
// channel, so this is the stable lookup key for callers that only know a template by its handle.
func (ts *notificationTemplateService) GetTemplateByHandle(ctx context.Context, channel ChannelType,
	handle string) (*Template, *tidcommon.ServiceError) {
	if svcErr := validateChannel(channel); svcErr != nil {
		return nil, svcErr
	}
	if handle == "" {
		return nil, &ErrorInvalidHandle
	}

	dao, err := ts.store.GetTemplateByHandle(ctx, channel, handle)
	if err != nil {
		if errors.Is(err, errTemplateNotFound) {
			return nil, &ErrorTemplateNotFound
		}
		ts.logger.Error(ctx, "Failed to retrieve notification template", log.String("handle", handle), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return daoToTemplate(dao), nil
}

// ValidateTemplate runs the same channel, handle, and content validation CreateTemplate performs,
// without persisting. The importer uses it so a dry run predicts the validation failures a real apply
// would hit, reusing the per-channel rules instead of duplicating them.
func (ts *notificationTemplateService) ValidateTemplate(ctx context.Context, channel ChannelType,
	request CreateTemplateRequest) *tidcommon.ServiceError {
	if svcErr := validateChannel(channel); svcErr != nil {
		return svcErr
	}
	if svcErr := validateHandle(request.Handle); svcErr != nil {
		return svcErr
	}
	_, svcErr := buildValidatedDAO(channel, "", request.DisplayName, request.Description,
		request.Content, request.Design)
	return svcErr
}

// UpdateTemplate updates an existing template. The handle is immutable, so it is not part of the
// request; existence and the write run in one transaction, and the stored handle is preserved and
// echoed on the response.
func (ts *notificationTemplateService) UpdateTemplate(ctx context.Context, channel ChannelType, id string,
	request UpdateTemplateRequest) (*Template, *tidcommon.ServiceError) {
	if svcErr := validateChannel(channel); svcErr != nil {
		return nil, svcErr
	}
	if id == "" {
		return nil, &ErrorInvalidTemplateID
	}

	dao, svcErr := buildValidatedDAO(channel, id, request.DisplayName, request.Description,
		request.Content, request.Design)
	if svcErr != nil {
		return nil, svcErr
	}

	var notFound bool
	txErr := ts.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existing, err := ts.store.GetTemplate(txCtx, channel, id)
		if err != nil {
			if errors.Is(err, errTemplateNotFound) {
				notFound = true
			}
			return err
		}
		// The handle is immutable; carry the stored value so the response reflects it.
		dao.Handle = existing.Handle
		return ts.store.UpdateTemplate(txCtx, dao)
	})
	if txErr != nil {
		if notFound {
			return nil, &ErrorTemplateNotFound
		}
		if errors.Is(txErr, errDeclarativeTemplate) {
			return nil, &ErrorTemplateReadOnly
		}
		ts.logger.Error(ctx, "Failed to update notification template", log.String("id", id), log.Error(txErr))
		return nil, &tidcommon.InternalServerError
	}

	ts.logger.Debug(ctx, "Successfully updated notification template", log.String("id", id))
	return daoToTemplate(dao), nil
}

// DeleteTemplate deletes a template, rejecting the delete with a conflict if a flow references it.
func (ts *notificationTemplateService) DeleteTemplate(
	ctx context.Context, channel ChannelType, id string,
) *tidcommon.ServiceError {
	if svcErr := validateChannel(channel); svcErr != nil {
		return svcErr
	}
	if id == "" {
		return &ErrorInvalidTemplateID
	}

	if _, err := ts.store.GetTemplate(ctx, channel, id); err != nil {
		if errors.Is(err, errTemplateNotFound) {
			// Idempotent delete: an absent template is treated as already deleted (204).
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
		// Fail closed: if usage could not be determined (nil response or nil TotalResults),
		// BlockingUsages reports none, so refuse the delete rather than risk removing a referenced
		// template.
		if resp == nil || resp.TotalResults == nil {
			ts.logger.Error(ctx, "Incomplete dependency response for template", log.String("id", id))
			return &tidcommon.InternalServerError
		}
		if len(resourcedependency.BlockingUsages(resp)) > 0 {
			return &ErrorTemplateInUse
		}
	}

	// Delete the template row (its design is a column on the same row).
	if err := ts.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return ts.store.DeleteTemplate(txCtx, channel, id)
	}); err != nil {
		if errors.Is(err, errDeclarativeTemplate) {
			return &ErrorTemplateReadOnly
		}
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

// persistUniqueHandle runs write inside a transaction after checking that handle is free in the
// channel. It maps a handle collision to ErrorTemplateHandleConflict and any other failure to an
// internal error.
func (ts *notificationTemplateService) persistUniqueHandle(ctx context.Context, channel ChannelType, handle string,
	write func(txCtx context.Context) error) *tidcommon.ServiceError {
	var handleConflict bool
	txErr := ts.transactioner.Transact(ctx, func(txCtx context.Context) error {
		taken, err := ts.store.IsHandleExists(txCtx, channel, handle)
		if err != nil {
			return err
		}
		if taken {
			handleConflict = true
			return errHandleConflict
		}
		return write(txCtx)
	})
	if txErr != nil {
		if handleConflict {
			return &ErrorTemplateHandleConflict
		}
		// The handle is free but it belongs to a file-declared template; such templates are read-only.
		if errors.Is(txErr, errDeclarativeTemplate) {
			return &ErrorTemplateReadOnly
		}
		// Race backstop: two concurrent creates can both pass the read pre-check; the loser's INSERT
		// then trips the UNIQUE (DEPLOYMENT_ID, CHANNEL, HANDLE) constraint. Map that to the documented
		// conflict instead of a 500.
		if isUniqueViolation(txErr) {
			return &ErrorTemplateHandleConflict
		}
		ts.logger.Error(ctx, "Failed to persist notification template", log.Error(txErr))
		return &tidcommon.InternalServerError
	}
	return nil
}

// buildValidatedDAO validates and normalizes a template. Shared by the service and declarative loader;
// the handle is validated and set separately by the caller.
func buildValidatedDAO(channel ChannelType, id, displayName, description string,
	content TemplateContent, design *TemplateDesign) (templateDAO, *tidcommon.ServiceError) {
	rules, svcErr := rulesFor(channel)
	if svcErr != nil {
		return templateDAO{}, svcErr
	}
	if strings.TrimSpace(displayName) == "" {
		return templateDAO{}, &ErrorMissingDisplayName
	}
	if utf8.RuneCountInString(displayName) > maxDisplayNameLength {
		return templateDAO{}, &ErrorDisplayNameTooLong
	}
	if utf8.RuneCountInString(description) > maxDescriptionLength {
		return templateDAO{}, &ErrorDescriptionTooLong
	}
	if strings.TrimSpace(content.Body) == "" {
		return templateDAO{}, &ErrorMissingBody
	}

	dao := templateDAO{
		ID:          id,
		Channel:     channel,
		DisplayName: displayName,
		Description: description,
		Content:     content,
		Design:      design,
	}
	if svcErr := rules.validate(dao); svcErr != nil {
		return templateDAO{}, svcErr
	}
	return rules.normalize(dao), nil
}

// validateHandle checks that a handle is present, within the length limit, and a well-formed
// kebab-case identifier.
func validateHandle(handle string) *tidcommon.ServiceError {
	if handle == "" {
		return &ErrorMissingHandle
	}
	if utf8.RuneCountInString(handle) > maxHandleLength {
		return &ErrorHandleTooLong
	}
	if !handleFormatRegex.MatchString(handle) {
		return &ErrorInvalidHandle
	}
	return nil
}

// validatePagination checks the limit/offset bounds, matching the shared server page-size limits.
func validatePagination(limit, offset int) *tidcommon.ServiceError {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return &ErrorInvalidLimit
	}
	if offset < 0 {
		return &ErrorInvalidOffset
	}
	return nil
}

// daoToTemplate maps a store DAO to the API representation. The design is surfaced exactly as stored;
// the channel handler already dropped it for channels without a design. A self link is only included
// on list items, not on the single-template representation.
func daoToTemplate(dao templateDAO) *Template {
	return &Template{
		ID:          dao.ID,
		Handle:      dao.Handle,
		DisplayName: dao.DisplayName,
		Description: dao.Description,
		Design:      dao.Design,
		Content:     dao.Content,
	}
}

// buildSelf builds the relative URL of a template resource.
func buildSelf(channel ChannelType, id string) string {
	return fmt.Sprintf("/notification-templates/%s/templates/%s", channel, id)
}

// buildCollectionSelf builds the relative URL of a channel's template collection, used as the base for
// pagination links.
func buildCollectionSelf(channel ChannelType) string {
	return fmt.Sprintf("/notification-templates/%s/templates", channel)
}
