// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const (
	// resourceTypeNotificationTemplate is the export/file resource type: the singular of the loader's
	// DirectoryName ("notification_templates"), so an exported file round-trips back through
	// loadDeclarativeTemplates.
	resourceTypeNotificationTemplate = "notification_template"

	// paramTypeNotificationTemplate is the parameterizer type and the label used in load logs.
	paramTypeNotificationTemplate = "NotificationTemplate"
)

// declaredNotificationTemplate is a file-declared template (camelCase, see backend/AGENTS.md). The id
// is taken verbatim from the file (any non-empty string; no UUID enforcement).
type declaredNotificationTemplate struct {
	ID          string          `yaml:"id"`
	Channel     string          `yaml:"channel"`
	Handle      string          `yaml:"handle"`
	OUID        string          `yaml:"ouId"`
	DisplayName string          `yaml:"displayName"`
	Description string          `yaml:"description"`
	Content     TemplateContent `yaml:"content"`
	Design      *TemplateDesign `yaml:"design"`
}

// parseDeclaredTemplate unmarshals one declared template. The loader resolves environment references
// before the parser runs.
func parseDeclaredTemplate(data []byte) (interface{}, error) {
	var declared declaredNotificationTemplate
	if err := yaml.Unmarshal(data, &declared); err != nil {
		return nil, fmt.Errorf("failed to parse the notification template: %w", err)
	}
	return &declared, nil
}

// svcErrReason returns the most specific human-readable reason from a service error, preferring the
// detailed description (e.g. "must be lowercase kebab-case") over the terse title ("Invalid handle").
func svcErrReason(svcErr *tidcommon.ServiceError) string {
	if svcErr.ErrorDescription.DefaultValue != "" {
		return svcErr.ErrorDescription.DefaultValue
	}
	return svcErr.Error.DefaultValue
}

// declaredToDAO validates a declared template and builds the DAO to store.
func declaredToDAO(declared *declaredNotificationTemplate) (templateDAO, error) {
	channel := ChannelType(declared.Channel)
	if strings.TrimSpace(declared.ID) == "" {
		return templateDAO{}, fmt.Errorf("notification template declaration is missing the required id")
	}
	if strings.TrimSpace(declared.OUID) == "" {
		return templateDAO{}, fmt.Errorf("notification template declaration is missing the required ouId")
	}
	if svcErr := validateHandle(declared.Handle); svcErr != nil {
		return templateDAO{}, fmt.Errorf("invalid notification template handle %q: %s", declared.Handle,
			svcErrReason(svcErr))
	}
	dao, svcErr := buildValidatedDAO(channel, declared.ID,
		declared.DisplayName, declared.Description, declared.Content, declared.Design)
	if svcErr != nil {
		return templateDAO{}, fmt.Errorf("invalid notification template %q: %s", declared.Handle,
			svcErrReason(svcErr))
	}
	dao.Handle = declared.Handle
	dao.OUID = declared.OUID
	return dao, nil
}

// newDeclaredTemplateValidator returns the loader's Validator. It rejects duplicate ids and
// (channel, handle) pairs within the load, tracking what it has seen so each check is O(1). dbStore is
// set only in composite mode, where a declared id or handle must not collide with a database template.
func newDeclaredTemplateValidator(dbStore notificationTemplateStoreInterface) func(interface{}) error {
	seenIDs := make(map[string]bool)
	seenHandles := make(map[string]bool)
	return func(data interface{}) error {
		declared, ok := data.(*declaredNotificationTemplate)
		if !ok {
			return fmt.Errorf("invalid data type: expected *declaredNotificationTemplate")
		}
		dao, err := declaredToDAO(declared)
		if err != nil {
			return err
		}

		handleKey := string(dao.Channel) + "\x00" + dao.Handle
		if seenIDs[dao.ID] {
			return fmt.Errorf("duplicate declared notification template id %q", dao.ID)
		}
		if seenHandles[handleKey] {
			return fmt.Errorf("duplicate declared notification template for channel %q and handle %q",
				dao.Channel, dao.Handle)
		}

		if dbStore != nil {
			if err := checkDeclaredTemplateAgainstDB(dbStore, dao); err != nil {
				return err
			}
		}

		seenIDs[dao.ID] = true
		seenHandles[handleKey] = true
		return nil
	}
}

// checkDeclaredTemplateAgainstDB rejects a declared template whose id or (channel, handle) is already
// taken by a database template.
func checkDeclaredTemplateAgainstDB(dbStore notificationTemplateStoreInterface, dao templateDAO) error {
	// context.Background(): a load-time check, outside any request.
	dbCtx := context.Background()

	exists, err := dbStore.IsHandleExists(dbCtx, dao.Channel, dao.Handle)
	if err != nil {
		return fmt.Errorf("failed to check the database for notification template handle %q: %w",
			dao.Handle, err)
	}
	if exists {
		return fmt.Errorf("notification template handle %q (channel %q) is already used by a "+
			"database template", dao.Handle, dao.Channel)
	}

	if _, err := dbStore.GetTemplate(dbCtx, dao.Channel, dao.ID); err == nil {
		return fmt.Errorf("notification template id %q (channel %q) is already used by a "+
			"database template", dao.ID, dao.Channel)
	} else if !errors.Is(err, errTemplateNotFound) {
		return fmt.Errorf("failed to check the database for notification template id %q: %w", dao.ID, err)
	}
	return nil
}

// loadDeclarativeTemplates reads every declared template into the in-memory store. dbStore is passed
// only in composite mode so a declared handle already used by a database template fails the load. The
// file store is handed to the loader directly as its sink.
func loadDeclarativeTemplates(fileStore *templateFileBasedStore, dbStore notificationTemplateStoreInterface) error {
	loader := declarativeresource.NewResourceLoader(declarativeresource.ResourceConfig{
		ResourceType:  paramTypeNotificationTemplate,
		DirectoryName: "notification_templates",
		Parser:        parseDeclaredTemplate,
		Validator:     newDeclaredTemplateValidator(dbStore),
		IDExtractor: func(data interface{}) string {
			if declared, ok := data.(*declaredNotificationTemplate); ok {
				return declared.ID
			}
			return ""
		},
	}, fileStore)

	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load notification template resources: %w", err)
	}
	return nil
}

// sortedChannels returns the registered channels in a fixed (name-sorted) order, so exports that sweep
// channels are deterministic. Derived from channelRulesByChannel, so a new channel is picked up here.
func sortedChannels() []ChannelType {
	channels := make([]ChannelType, 0, len(channelRulesByChannel))
	for channel := range channelRulesByChannel {
		channels = append(channels, channel)
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i] < channels[j] })
	return channels
}

// exportName builds the export file name for a template. Handles are unique only within a channel
// (e.g. "user-invite" exists for both email and SMS), so the channel is included to keep names unique.
func exportName(channel ChannelType, handle string) string {
	return fmt.Sprintf("%s-%s", channel, handle)
}

// notificationTemplateExporter implements declarativeresource.ResourceExporter for notification
// templates. One exported file holds one template, matching the declared-template file shape. It reads
// through the service (like the role, theme, and group exporters) rather than the store directly.
type notificationTemplateExporter struct {
	service NotificationTemplateServiceInterface
}

// newNotificationTemplateExporter creates the notification template exporter over the service.
func newNotificationTemplateExporter(service NotificationTemplateServiceInterface) *notificationTemplateExporter {
	return &notificationTemplateExporter{service: service}
}

// GetResourceType returns the resource type for notification templates.
func (e *notificationTemplateExporter) GetResourceType() string {
	return resourceTypeNotificationTemplate
}

// GetParameterizerType returns the parameterizer type for notification templates.
func (e *notificationTemplateExporter) GetParameterizerType() string {
	return paramTypeNotificationTemplate
}

// GetAllResourceIDs returns every mutable template id across all channels, for wildcard export.
func (e *notificationTemplateExporter) GetAllResourceIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	ids := []string{}
	limit := serverconst.MaxPageSize
	// Iterate channels in a fixed order: map iteration is random, and gateway Capture hashes the
	// exported documents in this order, so a stable order keeps an unchanged config from re-versioning.
	for _, channel := range sortedChannels() {
		offset := 0
		for {
			list, svcErr := e.service.ListTemplates(ctx, channel, limit, offset)
			if svcErr != nil {
				return nil, svcErr
			}
			for _, summary := range list.Templates {
				// The list summary omits isReadOnly, so fetch each template to skip the declarative
				// (file-based) ones; export captures mutable DB state only.
				full, svcErr := e.service.GetTemplate(ctx, channel, summary.ID)
				if svcErr != nil {
					return nil, svcErr
				}
				if full.IsReadOnly {
					continue
				}
				ids = append(ids, summary.ID)
			}
			if len(list.Templates) < limit {
				break
			}
			offset += limit
		}
	}
	return ids, nil
}

// GetResourceByID returns the template with the given id. The id is unique across channels, so the
// channels are swept until it is found.
func (e *notificationTemplateExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *tidcommon.ServiceError) {
	for _, channel := range sortedChannels() {
		tmpl, svcErr := e.service.GetTemplate(ctx, channel, id)
		if svcErr != nil {
			if svcErr.Code == ErrorTemplateNotFound.Code {
				continue
			}
			return nil, "", svcErr
		}
		return templateToDeclared(channel, tmpl), exportName(channel, tmpl.Handle), nil
	}
	notFound := ErrorTemplateNotFound
	return nil, "", &notFound
}

// ValidateResource checks the template carries the fields the loader requires and returns its export
// name (channel-handle).
func (e *notificationTemplateExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger) (string, *declarativeresource.ExportError) {
	declared, ok := resource.(*declaredNotificationTemplate)
	if !ok {
		return "", declarativeresource.CreateTypeError(e.GetResourceType(), id)
	}
	// The channel is a separate concern from the name: validate it against the channel registry, which
	// rejects a missing or unknown channel.
	if _, svcErr := rulesFor(ChannelType(declared.Channel)); svcErr != nil {
		return "", &declarativeresource.ExportError{
			ResourceType: e.GetResourceType(),
			ResourceID:   id,
			Code:         "INVALID_NOTIFICATION_TEMPLATE",
			Error:        fmt.Sprintf("invalid or missing channel %q", declared.Channel),
		}
	}
	// The handle is the resource name: validate it with the shared helper.
	if err := declarativeresource.ValidateResourceName(ctx, declared.Handle, e.GetResourceType(), id,
		"INVALID_NOTIFICATION_TEMPLATE", logger); err != nil {
		return "", err
	}
	// Handles are unique only within a channel, so the file name is composed to stay unique.
	return exportName(ChannelType(declared.Channel), declared.Handle), nil
}

// GetResourceRules returns nil: templates carry no parameterized fields. A nil ruleset keeps scalars
// quoted; a non-nil one makes the parameterizer write them unquoted, corrupting a multi-line body or
// one containing ": ".
func (e *notificationTemplateExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return nil
}

// templateToDeclared converts a service Template (plus its channel, which the Template does not carry)
// to the file-declared shape so it round-trips through the loader when re-imported.
func templateToDeclared(channel ChannelType, t *Template) *declaredNotificationTemplate {
	return &declaredNotificationTemplate{
		ID:          t.ID,
		Channel:     string(channel),
		Handle:      t.Handle,
		OUID:        t.OUID,
		DisplayName: t.DisplayName,
		Description: t.Description,
		Content:     t.Content,
		Design:      t.Design,
	}
}
