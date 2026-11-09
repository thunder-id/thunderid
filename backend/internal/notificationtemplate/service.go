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
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/projectscope"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	// projectQueryParam is the list query parameter that names a project.
	projectQueryParam = "projectId"
	// maxProjectIDLength is the length of the PROJECT_ID column. A longer project ID names no project.
	maxProjectIDLength = 36
)

// NotificationTemplateServiceInterface defines the notification template management operations.
type NotificationTemplateServiceInterface interface {
	ListTemplates(ctx context.Context, channel ChannelType, limit, offset int) (
		*TemplateListResponse, *tidcommon.ServiceError)
	ListTemplatesByOUID(ctx context.Context, channel ChannelType, ouID string, limit, offset int) (
		*TemplateListResponse, *tidcommon.ServiceError)
	// ListTemplatesByProject lists a page of the templates of a channel in a project. A project that does not
	// exist has none.
	ListTemplatesByProject(ctx context.Context, channel ChannelType, projectID string, limit, offset int) (
		*TemplateListResponse, *tidcommon.ServiceError)
	// ProjectHasTemplates reports whether a template of any channel belongs to the project.
	ProjectHasTemplates(ctx context.Context, projectID string) (bool, *tidcommon.ServiceError)
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
	GetResourceDependencies(ctx context.Context, resourceType, id string) (
		[]resourcedependency.ResourceDependency, error)
}

// notificationTemplateService is the default implementation.
type notificationTemplateService struct {
	store              notificationTemplateStoreInterface
	transactioner      providers.Transactioner
	ouService          ou.OrganizationUnitServiceInterface
	authzService       sysauthz.SystemAuthorizationServiceInterface
	dependencyRegistry resourcedependency.Registry
	// projectValidator is nil on a deployment that serves no projects.
	projectValidator projectOUValidator
	// projectRequired is set when a deployment that serves projects refuses a template created without one.
	projectRequired bool
	logger          *log.Logger
	bus             eventbus.EventBusInterface
	// managed says which templates a control plane deployed, which stay read only here. Nil where
	// nothing is deployed from one.
	managed managedresource.StoreInterface
}

// projectOUValidator checks that a resource scoped to a project is in an organization unit the project holds.
type projectOUValidator interface {
	ValidateProjectOU(ctx context.Context, projectID, ouID string) *tidcommon.ServiceError
}

// newNotificationTemplateService creates a new service with the given store, transactioner, OU service,
// authorization service and project validator. The OU service verifies the ouId at creation; a nil ouService
// skips that check. A nil projectValidator serves no projects, and a project ID is kept unchecked.
// projectRequired refuses a template created without a project.
func newNotificationTemplateService(store notificationTemplateStoreInterface,
	transactioner providers.Transactioner, ouService ou.OrganizationUnitServiceInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface, projectValidator projectOUValidator,
	projectRequired bool, bus eventbus.EventBusInterface) NotificationTemplateServiceInterface {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationTemplateService"))
	return &notificationTemplateService{
		store:            store,
		transactioner:    transactioner,
		ouService:        ouService,
		authzService:     authzService,
		projectValidator: projectValidator,
		projectRequired:  projectRequired,
		logger:           logger,
		bus:              bus,
	}
}

// ListTemplates lists a page of the templates of a channel the caller may list.
func (ts *notificationTemplateService) ListTemplates(ctx context.Context, channel ChannelType, limit, offset int) (
	*TemplateListResponse, *tidcommon.ServiceError) {
	return ts.listTemplates(ctx, channel, "", "", limit, offset)
}

// ListTemplatesByOUID lists a page of the templates of a channel an organization unit owns.
func (ts *notificationTemplateService) ListTemplatesByOUID(ctx context.Context, channel ChannelType, ouID string,
	limit, offset int) (*TemplateListResponse, *tidcommon.ServiceError) {
	if ouID == "" {
		return nil, &ErrorInvalidOU
	}
	return ts.listTemplates(ctx, channel, ouID, "", limit, offset)
}

// ListTemplatesByProject lists a page of the templates of a channel in a project. An empty project ID names
// no project, so it lists none rather than every template.
func (ts *notificationTemplateService) ListTemplatesByProject(ctx context.Context, channel ChannelType,
	projectID string, limit, offset int) (*TemplateListResponse, *tidcommon.ServiceError) {
	if projectID == "" {
		if svcErr := validateChannel(channel); svcErr != nil {
			return nil, svcErr
		}
		if svcErr := validatePagination(limit, offset); svcErr != nil {
			return nil, svcErr
		}
		return &TemplateListResponse{
			StartIndex: offset + 1,
			Templates:  []TemplateSummary{},
			Links: utils.BuildPaginationLinks(buildCollectionSelf(channel), limit, offset, 0,
				"&"+projectQueryParam+"="),
		}, nil
	}
	return ts.listTemplates(ctx, channel, "", projectID, limit, offset)
}

// ProjectHasTemplates reports whether a template of any channel belongs to the project.
func (ts *notificationTemplateService) ProjectHasTemplates(ctx context.Context, projectID string) (
	bool, *tidcommon.ServiceError) {
	for channel := range channelRulesByChannel {
		count, err := ts.store.CountTemplatesByProject(ctx, channel, projectID)
		if err != nil {
			// A project with more templates than a composite read returns still has templates.
			if errors.Is(err, errResultLimitExceededInCompositeMode) {
				return true, nil
			}
			ts.logger.Error(ctx, "Failed to count the templates of a project", log.String("projectID", projectID),
				log.Error(err))
			return false, &tidcommon.InternalServerError
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

// CreateTemplate creates a new template with a server-assigned id. The handle-uniqueness check and the
// insert run in one transaction so a concurrent create cannot slip a duplicate past the check.
func (ts *notificationTemplateService) CreateTemplate(ctx context.Context, channel ChannelType,
	request CreateTemplateRequest) (*Template, *tidcommon.ServiceError) {
	created, svcErr := ts.createTemplate(ctx, channel, request)
	var templateID string
	if svcErr == nil {
		templateID = created.ID
	}
	ts.publishTemplateEvent(ctx, audit.EventNotificationTemplateCreate, audit.ActionCreate, templateID, svcErr)
	return created, svcErr
}

// createTemplate creates a new template without publishing its audit event.
func (ts *notificationTemplateService) createTemplate(ctx context.Context, channel ChannelType,
	request CreateTemplateRequest) (*Template, *tidcommon.ServiceError) {
	// In declarative-only mode the store is read-only: no handle can be created through the API.
	if isDeclarativeModeEnabled() {
		return nil, &ErrorDeclarativeModeCreateNotAllowed
	}
	if svcErr := validateChannel(channel); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := validateHandle(request.Handle); svcErr != nil {
		return nil, svcErr
	}
	ouID, svcErr := ts.resolveCreateOU(ctx, request.ProjectID, request.OUID)
	if svcErr != nil {
		return nil, svcErr
	}

	dao, svcErr := buildValidatedDAO(channel, "", request.DisplayName, request.Description,
		request.Content, request.Design)
	if svcErr != nil {
		return nil, svcErr
	}
	dao.Handle = request.Handle
	dao.OUID = ouID
	dao.ProjectID = request.ProjectID

	id := request.ID
	if id == "" {
		var err error
		id, err = utils.GenerateUUIDv7()
		if err != nil {
			ts.logger.Error(ctx, "Failed to generate template id", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
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
	if !dao.IsReadOnly {
		deployed, err := managedresource.IsDeployed(ctx, ts.managed, id, managedresource.TypeNotificationTemplate)
		if err != nil {
			// Every write to such a template is refused whatever the read says, so it is served without the flag.
			ts.logger.Error(ctx, "Failed to read whether a control plane deployed the template",
				log.String("id", id), log.Error(err))
		}
		dao.IsReadOnly = deployed
	}

	if svcErr := ts.checkTemplateAccess(ctx, security.ActionReadNotificationTemplate, dao.OUID); svcErr != nil {
		return nil, svcErr
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

	if svcErr := ts.checkTemplateAccess(ctx, security.ActionReadNotificationTemplate, dao.OUID); svcErr != nil {
		return nil, svcErr
	}

	return daoToTemplate(dao), nil
}

// ValidateTemplate runs the same channel, handle, OU, and content validation CreateTemplate performs,
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
	if _, svcErr := ts.resolveCreateOU(ctx, request.ProjectID, request.OUID); svcErr != nil {
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
	updated, svcErr := ts.updateTemplate(ctx, channel, id, request)
	ts.publishTemplateEvent(ctx, audit.EventNotificationTemplateUpdate, audit.ActionUpdate, id, svcErr)
	return updated, svcErr
}

// updateTemplate updates an existing template without publishing its audit event.
func (ts *notificationTemplateService) updateTemplate(ctx context.Context, channel ChannelType, id string,
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
	var projectErr *tidcommon.ServiceError
	var accessErr *tidcommon.ServiceError
	var managedErr *tidcommon.ServiceError
	txErr := ts.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existing, err := ts.store.GetTemplate(txCtx, channel, id)
		if err != nil {
			if errors.Is(err, errTemplateNotFound) {
				notFound = true
			}
			return err
		}

		if accessErr = ts.checkTemplateAccess(txCtx, security.ActionUpdateNotificationTemplate,
			existing.OUID); accessErr != nil {
			return errTemplateAccessDenied
		}

		// A file-declared template is read-only: refuse before the store write. Mapped to
		// ErrorTemplateReadOnly below.
		if existing.IsReadOnly {
			return errDeclarativeTemplate
		}
		if svcErr := ts.refuseManaged(txCtx, id); svcErr != nil {
			managedErr = svcErr
			return errManagedTemplate
		}

		// The handle and ouId are immutable; carry the stored values so the response reflects them.
		dao.Handle = existing.Handle
		dao.OUID = existing.OUID

		// An update that names no project, or the current one, keeps the template in its project without
		// checking it again. A different project has to hold the template's organization unit.
		dao.ProjectID = request.ProjectID
		if dao.ProjectID == "" {
			dao.ProjectID = existing.ProjectID
		}
		if dao.ProjectID != existing.ProjectID {
			if projectErr = ts.validateProjectOU(txCtx, dao.ProjectID, dao.OUID); projectErr != nil {
				return errProjectRefused
			}
		}
		return ts.store.UpdateTemplate(txCtx, dao)
	})
	if txErr != nil {
		if notFound {
			return nil, &ErrorTemplateNotFound
		}
		if accessErr != nil {
			return nil, accessErr
		}
		if projectErr != nil {
			return nil, projectErr
		}
		if errors.Is(txErr, errDeclarativeTemplate) {
			return nil, &ErrorTemplateReadOnly
		}
		if managedErr != nil {
			return nil, managedErr
		}
		ts.logger.Error(ctx, "Failed to update notification template", log.String("id", id), log.Error(txErr))
		return nil, &tidcommon.InternalServerError
	}

	ts.logger.Debug(ctx, "Successfully updated notification template", log.String("id", id))
	return daoToTemplate(dao), nil
}

// DeleteTemplate deletes a template, rejecting the delete with a conflict if it is in use.
func (ts *notificationTemplateService) DeleteTemplate(
	ctx context.Context, channel ChannelType, id string,
) *tidcommon.ServiceError {
	svcErr := ts.deleteTemplate(ctx, channel, id)
	// A delete that found nothing to remove succeeds without an event, so no caller can record a
	// deletion of an ID it made up.
	if svcErr == errTemplateNotFoundNoop {
		return nil
	}
	ts.publishTemplateEvent(ctx, audit.EventNotificationTemplateDelete, audit.ActionDelete, id, svcErr)
	return svcErr
}

// deleteTemplate deletes a template without publishing its audit event.
func (ts *notificationTemplateService) deleteTemplate(
	ctx context.Context, channel ChannelType, id string,
) *tidcommon.ServiceError {
	if svcErr := validateChannel(channel); svcErr != nil {
		return svcErr
	}
	if id == "" {
		return &ErrorInvalidTemplateID
	}

	existing, err := ts.store.GetTemplate(ctx, channel, id)
	if err != nil {
		if errors.Is(err, errTemplateNotFound) {
			// Idempotent delete: an absent template is treated as already deleted (204).
			return errTemplateNotFoundNoop
		}
		ts.logger.Error(ctx, "Failed to check template existence", log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}

	if svcErr := ts.checkTemplateAccess(ctx, security.ActionDeleteNotificationTemplate,
		existing.OUID); svcErr != nil {
		return svcErr
	}

	// A file-declared template is read-only: refuse before the dependency check so a declared template
	// that a flow references returns read-only (403) rather than in-use (409).
	if existing.IsReadOnly {
		return &ErrorTemplateReadOnly
	}
	if svcErr := ts.refuseManaged(ctx, id); svcErr != nil {
		return svcErr
	}

	// A template reported as a blocking dependency by the registry cannot be deleted.
	if ts.dependencyRegistry != nil {
		resp, depErr := ts.dependencyRegistry.GetDependencies(
			ctx, resourcedependency.ResourceTypeNotificationTemplate, id)
		if depErr != nil {
			ts.logger.Error(ctx, "Failed to resolve template dependencies", log.String("id", id), log.Error(depErr))
			return &tidcommon.InternalServerError
		}
		// Fail closed: unknown usage (nil response or TotalResults) refuses the delete.
		if resp == nil || resp.TotalResults == nil {
			ts.logger.Error(ctx, "Incomplete dependency response for template", log.String("id", id))
			return &tidcommon.InternalServerError
		}
		if blocking := resourcedependency.BlockingUsages(resp); len(blocking) > 0 {
			usages := resourcedependency.SummarizeBlockingUsages(blocking)
			// Carry the usage summary as a param so a localized client key can interpolate it; the
			// default value stays a complete sentence for non-UI consumers and logs.
			return tidcommon.CustomServiceError(ErrorTemplateInUse, tidcommon.I18nMessage{
				Key: "error.notificationtemplateservice.template_in_use_description",
				DefaultValue: fmt.Sprintf(
					"The template cannot be deleted because %s depend on it. Remove or reassign them first.", usages),
				Params: map[string]string{"usages": usages},
			})
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

// publishTemplateEvent publishes the audit event of an attempt on the template with id, after the change
// committed or was refused. No part of the template, its handle, name or content, is recorded.
func (ts *notificationTemplateService) publishTemplateEvent(ctx context.Context, name string,
	action audit.Action, id string, svcErr *tidcommon.ServiceError) {
	// A failed attempt keeps the ID the caller asked for, even one that matches no template, as forensic
	// evidence of what was probed; the payload caps it and the sink escapes it.
	target := audit.Ref{Type: audit.TargetTypeNotificationTemplate, ID: id}
	eventbus.TryPublish(ctx, ts.bus,
		eventbus.NewEvent(ctx, name).With(audit.ChangePayload(ctx, target, action, svcErr)))
}

// SetDependencyRegistry injects the dependency registry. Called by the service manager after the
// provider services are initialized to avoid a cyclic import.
func (ts *notificationTemplateService) SetDependencyRegistry(r resourcedependency.Registry) {
	ts.dependencyRegistry = r
}

// GetResourceDependencies implements resourcedependency.Provider: it blocks deletion of a template
// required by a built-in endpoint. It lives on the template service, not the consumer, because
// template deletion is served on both planes while a consumer runs on one.
func (ts *notificationTemplateService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	usages := make([]resourcedependency.ResourceDependency, 0)
	if resourceType != resourcedependency.ResourceTypeNotificationTemplate {
		return usages, nil
	}

	for _, r := range systemReservedTemplates {
		dao, err := ts.store.GetTemplateByHandle(ctx, r.channel, r.handle)
		if err != nil {
			if errors.Is(err, errTemplateNotFound) {
				// Not provisioned; nothing to protect.
				continue
			}
			// The registry logs provider errors from GetDependencies; don't double-log here.
			return nil, err
		}
		if dao.ID == id {
			usages = append(usages, resourcedependency.ResourceDependency{
				ResourceType:     systemEndpointReferrer,
				ID:               r.consumerID,
				DisplayName:      r.displayName,
				BehaviorOnDelete: resourcedependency.BehaviorRestrict,
			})
		}
	}

	return usages, nil
}

// listTemplates lists a page of templates of a channel, only the ones an organization unit owns when
// an organization unit is given, or only the ones in a project when a project is given.
func (ts *notificationTemplateService) listTemplates(ctx context.Context, channel ChannelType, ouID, projectID string,
	limit, offset int) (*TemplateListResponse, *tidcommon.ServiceError) {
	if svcErr := validateChannel(channel); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := validatePagination(limit, offset); svcErr != nil {
		return nil, svcErr
	}

	extraQuery := ""
	var ouIDs []string
	all := false
	if projectID != "" {
		extraQuery = "&" + projectQueryParam + "=" + url.QueryEscape(projectID)
	} else {
		if ouID != "" {
			extraQuery = utils.FilterQueryParam(map[string]any{filter.AttributeOUID: ouID})
		}
		var svcErr *tidcommon.ServiceError
		ouIDs, all, svcErr = ts.resolveListOUs(ctx, ouID)
		if svcErr != nil {
			return nil, svcErr
		}
		if !all && len(ouIDs) == 0 {
			return &TemplateListResponse{
				StartIndex: offset + 1,
				Templates:  []TemplateSummary{},
				Links:      utils.BuildPaginationLinks(buildCollectionSelf(channel), limit, offset, 0, extraQuery),
			}, nil
		}
	}

	var total int
	var err error
	switch {
	case projectID != "":
		total, err = ts.store.CountTemplatesByProject(ctx, channel, projectID)
	case all:
		total, err = ts.store.CountTemplates(ctx, channel)
	default:
		total, err = ts.store.CountTemplatesByOUIDs(ctx, channel, ouIDs)
	}
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrorResultLimitExceededInCompositeMode
		}
		ts.logger.Error(ctx, "Failed to count notification templates", log.String("ouID", ouID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	var templates []templateDAO
	switch {
	case projectID != "":
		templates, err = ts.store.ListTemplatesByProject(ctx, channel, projectID, limit, offset)
	case all:
		templates, err = ts.store.ListTemplates(ctx, channel, limit, offset)
	default:
		templates, err = ts.store.ListTemplatesByOUIDs(ctx, channel, ouIDs, limit, offset)
	}
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrorResultLimitExceededInCompositeMode
		}
		ts.logger.Error(ctx, "Failed to list notification templates", log.String("ouID", ouID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	summaries := make([]TemplateSummary, 0, len(templates))
	for _, t := range templates {
		summaries = append(summaries, TemplateSummary{
			ID:          t.ID,
			Handle:      t.Handle,
			DisplayName: t.DisplayName,
			Description: t.Description,
			ProjectID:   t.ProjectID,
			Self:        buildSelf(channel, t.ID),
		})
	}

	return &TemplateListResponse{
		TotalResults: total,
		StartIndex:   offset + 1,
		Count:        len(summaries),
		Templates:    summaries,
		Links:        utils.BuildPaginationLinks(buildCollectionSelf(channel), limit, offset, total, extraQuery),
	}, nil
}

// resolveListOUs returns the organization units a template list serves for the caller, given the one
// the request filters by, and true when it serves every organization unit. A caller confined to
// organization units lists the templates of its own and of the ones above it, which its flows can use.
func (ts *notificationTemplateService) resolveListOUs(ctx context.Context, ouID string) (
	[]string, bool, *tidcommon.ServiceError) {
	accessible, svcErr := ts.authzService.GetAccessibleResources(
		ctx, security.ActionListNotificationTemplates, security.ResourceTypeOU)
	if svcErr != nil {
		ts.logger.Error(ctx, "Failed to resolve the organization units the caller may list",
			log.Any("error", svcErr))
		return nil, false, &tidcommon.InternalServerError
	}
	switch {
	case accessible.AllAllowed && ouID == "":
		return nil, true, nil
	case accessible.AllAllowed || slices.Contains(accessible.IDs, ouID):
		return []string{ouID}, false, nil
	case ouID != "":
		return []string{}, false, nil
	default:
		return accessible.IDs, false, nil
	}
}

// checkTemplateAccess checks the caller may perform the action on a template of the organization
// unit.
func (ts *notificationTemplateService) checkTemplateAccess(ctx context.Context, action security.Action,
	ouID string) *tidcommon.ServiceError {
	allowed, svcErr := ts.authzService.IsActionAllowed(ctx, action,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeNotificationTemplate, OUID: ouID})
	if svcErr != nil {
		ts.logger.Error(ctx, "Failed to check authorization for action",
			log.String("action", string(action)), log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}
	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
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
		OUID:        dao.OUID,
		ProjectID:   dao.ProjectID,
		Handle:      dao.Handle,
		DisplayName: dao.DisplayName,
		Description: dao.Description,
		Design:      dao.Design,
		Content:     dao.Content,
		IsReadOnly:  dao.IsReadOnly,
	}
}

// resolveOU verifies that ouID is present and references an existing organization unit. A nil ouService
// skips the check (e.g. in tests or a bootstrap context that has no OU service wired).
func (ts *notificationTemplateService) resolveOU(ctx context.Context, ouID string) *tidcommon.ServiceError {
	if ts.ouService == nil {
		return nil
	}
	if strings.TrimSpace(ouID) == "" {
		return &ErrorInvalidOU
	}
	exists, svcErr := ts.ouService.IsOrganizationUnitExists(ctx, ouID)
	if svcErr != nil {
		ts.logger.Error(ctx, "Failed to verify organization unit", log.String("ouID", ouID),
			log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}
	if !exists {
		return &ErrorInvalidOU
	}
	return nil
}

// resolveCreateOU returns the organization unit a new template is placed in, once the caller may create
// templates there. A template in a project may leave its organization unit out, so the caller is then
// checked against the one it is placed in.
func (ts *notificationTemplateService) resolveCreateOU(ctx context.Context, projectID, ouID string) (
	string, *tidcommon.ServiceError) {
	if ouID != "" {
		if svcErr := ts.checkTemplateAccess(ctx, security.ActionCreateNotificationTemplate, ouID); svcErr != nil {
			return "", svcErr
		}
	}
	resolved, svcErr := ts.resolveTemplateOU(ctx, projectID, ouID)
	if svcErr != nil {
		return "", svcErr
	}
	if ouID == "" {
		if svcErr := ts.checkTemplateAccess(ctx, security.ActionCreateNotificationTemplate,
			resolved); svcErr != nil {
			return "", svcErr
		}
	}
	return resolved, nil
}

// resolveTemplateOU returns the organization unit a new template is placed in. A deployment that requires a
// project refuses a template without one. A template in a project may leave the organization unit out to be
// placed in the system organization unit, and one it names has to belong to the project.
func (ts *notificationTemplateService) resolveTemplateOU(ctx context.Context, projectID, ouID string) (
	string, *tidcommon.ServiceError) {
	if projectID == "" {
		if ts.projectRequired {
			return "", &ErrorProjectRequired
		}
		return ouID, ts.resolveOU(ctx, ouID)
	}

	if strings.TrimSpace(ouID) == "" && ts.ouService != nil {
		systemOUID, svcErr := ts.ouService.GetSystemOUID(ctx)
		if svcErr != nil {
			ts.logger.Error(ctx, "Failed to get the system organization unit", log.String("code", svcErr.Code))
			return "", &tidcommon.InternalServerError
		}
		ouID = systemOUID
	} else if svcErr := ts.resolveOU(ctx, ouID); svcErr != nil {
		return "", svcErr
	}
	return ouID, ts.validateProjectOU(ctx, projectID, ouID)
}

// validateProjectOU checks that a template's organization unit belongs to its project. It returns
// ErrorInvalidProject when no such project exists, and ErrorOUNotInProject when the organization unit is
// outside the project.
//
// A deployment that serves no projects, such as a Data Plane, cannot check the project. It keeps the project
// ID as given so that imported configuration retains it.
func (ts *notificationTemplateService) validateProjectOU(ctx context.Context, projectID,
	ouID string) *tidcommon.ServiceError {
	// The column holds 36 characters, so a longer ID is refused on every deployment, checked or not.
	if utf8.RuneCountInString(projectID) > maxProjectIDLength {
		return &ErrorInvalidProject
	}
	if ts.projectValidator == nil {
		return nil
	}
	svcErr := ts.projectValidator.ValidateProjectOU(ctx, projectID, ouID)
	switch {
	case svcErr == nil:
		return nil
	case svcErr.Code == projectscope.ProjectNotFoundErrorCode:
		return &ErrorInvalidProject
	case svcErr.Code == projectscope.OUNotInProjectErrorCode:
		return &ErrorOUNotInProject
	default:
		ts.logger.Error(ctx, "Failed to check the project of a template",
			log.String("projectID", projectID), log.String("code", svcErr.Code))
		return &tidcommon.InternalServerError
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

// errManagedTemplate rolls back an update refused because a control plane deployed the template.
var errManagedTemplate = errors.New("the template was deployed by a control plane")

// refuseManaged refuses a change to a template a control plane deployed, unless the change is the managed
// import's: only a deploy from the control plane changes or removes it.
func (ts *notificationTemplateService) refuseManaged(ctx context.Context, id string) *tidcommon.ServiceError {
	managed, err := managedresource.IsReadOnly(ctx, ts.managed, id, managedresource.TypeNotificationTemplate)
	if err != nil {
		ts.logger.Error(ctx, "Failed to read whether a control plane deployed the template",
			log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if managed {
		return &ErrorManagedTemplate
	}
	return nil
}
