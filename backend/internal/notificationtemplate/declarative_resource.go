// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// declaredNotificationTemplate is a file-declared template (camelCase, see backend/AGENTS.md). The id
// is taken verbatim from the file (any non-empty string; no UUID enforcement).
type declaredNotificationTemplate struct {
	ID          string          `yaml:"id"`
	Channel     string          `yaml:"channel"`
	Handle      string          `yaml:"handle"`
	DisplayName string          `yaml:"displayName"`
	Description string          `yaml:"description"`
	Content     TemplateContent `yaml:"content"`
	Design      *TemplateDesign `yaml:"design"`
}

// parseDeclaredTemplate reads one declared template, resolving environment references first.
func parseDeclaredTemplate(data []byte) (interface{}, error) {
	resolved, err := utils.SubstituteEnvironmentVariables(data)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve the template's environment references: %w", err)
	}
	var declared declaredNotificationTemplate
	if err := yaml.Unmarshal(resolved, &declared); err != nil {
		return nil, fmt.Errorf("failed to parse the notification template: %w", err)
	}
	return &declared, nil
}

// declaredToDAO validates a declared template and builds the DAO to store.
func declaredToDAO(declared *declaredNotificationTemplate) (templateDAO, error) {
	channel := ChannelType(declared.Channel)
	if strings.TrimSpace(declared.ID) == "" {
		return templateDAO{}, fmt.Errorf("notification template declaration is missing the required id")
	}
	if svcErr := validateHandle(declared.Handle); svcErr != nil {
		return templateDAO{}, fmt.Errorf("invalid notification template handle %q: %s", declared.Handle,
			svcErr.Error.DefaultValue)
	}
	dao, svcErr := buildValidatedDAO(channel, declared.ID,
		declared.DisplayName, declared.Description, declared.Content, declared.Design)
	if svcErr != nil {
		return templateDAO{}, fmt.Errorf("invalid notification template %q: %s", declared.Handle,
			svcErr.Error.DefaultValue)
	}
	dao.Handle = declared.Handle
	return dao, nil
}

// validateDeclaredTemplate refuses a declaration that could not be applied.
func validateDeclaredTemplate(data interface{}) error {
	declared, ok := data.(*declaredNotificationTemplate)
	if !ok {
		return fmt.Errorf("invalid data type: expected *declaredNotificationTemplate")
	}
	_, err := declaredToDAO(declared)
	return err
}

// declaredTemplateStore is the loader's sink: it converts a declaration to a DAO and stores it.
// dbStore is set only in composite mode so a declared handle can be checked against the database.
type declaredTemplateStore struct {
	fileStore *templateFileBasedStore
	dbStore   notificationTemplateStoreInterface
	logger    *log.Logger
}

// Create stores one declared template.
func (s *declaredTemplateStore) Create(_ string, data interface{}) error {
	declared, ok := data.(*declaredNotificationTemplate)
	if !ok {
		return fmt.Errorf("invalid data type: expected *declaredNotificationTemplate")
	}
	dao, err := declaredToDAO(declared)
	if err != nil {
		return err
	}
	// Reject a declaration that collides with one already loaded: a duplicate id, or a duplicate
	// (channel, handle) pair (the composite store dedupes on handle, so a second one would silently
	// shadow the first).
	loaded, err := s.fileStore.listAll()
	if err != nil {
		return fmt.Errorf("failed to check for duplicate declared notification templates: %w", err)
	}
	for _, t := range loaded {
		if t.ID == dao.ID {
			return fmt.Errorf("duplicate declared notification template id %q", dao.ID)
		}
		if t.Channel == dao.Channel && t.Handle == dao.Handle {
			return fmt.Errorf("duplicate declared notification template for channel %q and handle %q",
				dao.Channel, dao.Handle)
		}
	}
	// In composite mode a declared template cannot reuse a handle already taken by a database
	// template; the composite store dedupes on handle, so this would otherwise shadow the DB row.
	if s.dbStore != nil {
		_, err := s.dbStore.GetTemplateByHandle(context.Background(), dao.Channel, dao.Handle)
		if err == nil {
			return fmt.Errorf("notification template handle %q (channel %q) is already used by a "+
				"database template", dao.Handle, dao.Channel)
		}
		if !errors.Is(err, errTemplateNotFound) {
			return fmt.Errorf("failed to check the database for notification template handle %q: %w",
				dao.Handle, err)
		}
	}
	if err := s.fileStore.put(dao); err != nil {
		return fmt.Errorf("failed to load notification template %q: %w", dao.Handle, err)
	}
	// Read at startup, outside any request.
	s.logger.Info(context.Background(), "Loaded a declared notification template",
		log.String("channel", string(dao.Channel)), log.String("handle", dao.Handle))
	return nil
}

// loadDeclarativeTemplates reads every declared template into the in-memory store. dbStore is passed
// only in composite mode so a declared handle already used by a database template fails the load.
func loadDeclarativeTemplates(fileStore *templateFileBasedStore, dbStore notificationTemplateStoreInterface) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationTemplateDeclarative"))

	loader := declarativeresource.NewResourceLoader(declarativeresource.ResourceConfig{
		ResourceType:  resourceTypeNotificationTemplate,
		DirectoryName: "notification_templates",
		Parser:        parseDeclaredTemplate,
		Validator:     validateDeclaredTemplate,
		IDExtractor: func(data interface{}) string {
			if declared, ok := data.(*declaredNotificationTemplate); ok {
				return declared.ID
			}
			return ""
		},
	}, &declaredTemplateStore{fileStore: fileStore, dbStore: dbStore, logger: logger})

	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load notification template resources: %w", err)
	}
	return nil
}

// notificationTemplateExporter implements declarative_resource.ResourceExporter for one channel.
// Templates are addressed by (channel, id), so one exporter is created per channel (mirroring the
// per-category entity-type exporters).
type notificationTemplateExporter struct {
	service NotificationTemplateServiceInterface
	channel ChannelType
}

// newNotificationTemplateExporter creates an exporter for the given channel.
func newNotificationTemplateExporter(service NotificationTemplateServiceInterface,
	channel ChannelType) *notificationTemplateExporter {
	return &notificationTemplateExporter{service: service, channel: channel}
}

// GetResourceType returns the channel-scoped resource type (e.g. "email_notification_template").
func (e *notificationTemplateExporter) GetResourceType() string {
	return string(e.channel) + "_notification_template"
}

// GetParameterizerType returns the parameterizer type name.
func (e *notificationTemplateExporter) GetParameterizerType() string {
	return "NotificationTemplate"
}

// GetAllResourceIDs lists every template id in the exporter's channel.
func (e *notificationTemplateExporter) GetAllResourceIDs(ctx context.Context) (
	[]string, *tidcommon.ServiceError) {
	offset := 0
	limit := serverconst.MaxPageSize
	var ids []string

	for {
		response, svcErr := e.service.ListTemplates(ctx, e.channel, limit, offset)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, t := range response.Templates {
			ids = append(ids, t.ID)
		}
		if len(response.Templates) == 0 {
			break
		}
		offset += len(response.Templates)
	}

	return ids, nil
}

// GetResourceByID fetches a template and maps it to the file-declaration shape so the export
// round-trips with the declarative loader (the YAML carries the channel the path would otherwise
// supply).
func (e *notificationTemplateExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *tidcommon.ServiceError) {
	tmpl, svcErr := e.service.GetTemplate(ctx, e.channel, id)
	if svcErr != nil {
		return nil, "", svcErr
	}
	declared := &declaredNotificationTemplate{
		ID:          tmpl.ID,
		Channel:     string(e.channel),
		Handle:      tmpl.Handle,
		DisplayName: tmpl.DisplayName,
		Description: tmpl.Description,
		Content:     tmpl.Content,
		Design:      tmpl.Design,
	}
	return declared, tmpl.DisplayName, nil
}

// ValidateResource validates an exported template and extracts its name.
func (e *notificationTemplateExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger) (string, *declarativeresource.ExportError) {
	declared, ok := resource.(*declaredNotificationTemplate)
	if !ok {
		return "", declarativeresource.CreateTypeError(e.GetResourceType(), id)
	}
	if err := declarativeresource.ValidateResourceName(ctx,
		declared.DisplayName, e.GetResourceType(), id, "TEMPLATE_VALIDATION_ERROR", logger); err != nil {
		return "", err
	}
	return declared.DisplayName, nil
}

// GetResourceRules returns the parameterization rules for notification templates.
func (e *notificationTemplateExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{}
}
