// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// AppliedConfiguration is the configuration a gateway runs: every resource of the version recorded as
// applied to it, each in the shape the resource's own read returns. An apply is recorded even when the
// gateway refused some of its resources, which the next apply sends again, so a refused resource is
// listed all the same; the apply's own result names it.
type AppliedConfiguration struct {
	GatewayID string `json:"gatewayId"`
	// Version is the hash of the version the gateway applied, empty when nothing was applied.
	Version   string            `json:"version,omitempty"`
	AppliedAt *time.Time        `json:"appliedAt,omitempty"`
	Resources []AppliedResource `json:"resources"`
	// Skipped are the version's documents that cannot be shown as their resource.
	Skipped []SkippedResource `json:"skipped,omitempty"`
}

// AppliedResource is one resource of an applied configuration.
type AppliedResource struct {
	ResourceType string `json:"resourceType"`
	// ID is the resource's id, or its name for a resource exported without one.
	ID string `json:"id"`
	// Resource is what the resource's own read returns.
	Resource interface{} `json:"resource"`
	// Parts are the collections the resource's own API serves apart from it, such as a group's
	// members, keyed by the path below the resource they are served at.
	Parts map[string]interface{} `json:"parts,omitempty"`
}

// SkippedResource is a document of an applied version that cannot be shown as its resource.
type SkippedResource struct {
	ResourceType string `json:"resourceType"`
	ID           string `json:"id,omitempty"`
	Code         string `json:"code"`
	Reason       string `json:"reason"`
}

// AppliedConfigurationServiceInterface shows the configuration a gateway runs.
type AppliedConfigurationServiceInterface interface {
	// GetAppliedConfiguration returns every resource of the version a gateway last applied, as each
	// resource's own read returns it.
	GetAppliedConfiguration(ctx context.Context, gatewayID string) (*AppliedConfiguration, *tidcommon.ServiceError)
}

// appliedConfigurationService reads a gateway's applied version through the exporter of each
// resource type, which is the one that wrote it.
//
// A gateway runs the version it applied, which this plane's live configuration may have moved on
// from, so this is how a console shows the configuration as a gateway runs it.
type appliedConfigurationService struct {
	versions *versionService
	viewers  map[string]declarativeresource.ResourceViewer
	logger   *log.Logger
}

// newAppliedConfigurationService reads with the exporters that can show what they export. A
// resource type whose exporter cannot is reported as skipped rather than shown some other way.
func newAppliedConfigurationService(versions *versionService,
	exporters []declarativeresource.ResourceExporter) AppliedConfigurationServiceInterface {
	viewers := make(map[string]declarativeresource.ResourceViewer, len(exporters))
	for _, exporter := range exporters {
		if viewer, ok := exporter.(declarativeresource.ResourceViewer); ok {
			viewers[exporter.GetResourceType()] = viewer
		}
	}
	return &appliedConfigurationService{
		versions: versions,
		viewers:  viewers,
		logger:   log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GatewayAppliedConfigurationService")),
	}
}

func (s *appliedConfigurationService) GetAppliedConfiguration(ctx context.Context,
	gatewayID string) (*AppliedConfiguration, *tidcommon.ServiceError) {
	applied, svcErr := s.versions.GetApplied(ctx, gatewayID)
	if svcErr != nil {
		return nil, svcErr
	}
	configuration := &AppliedConfiguration{GatewayID: gatewayID, Resources: []AppliedResource{}}
	if applied.AppliedVersion == 0 {
		return configuration, nil
	}
	version, svcErr := s.versions.bySeq(ctx, applied.AppliedVersion)
	if svcErr != nil {
		return nil, svcErr
	}
	configuration.Version = version.Hash
	configuration.AppliedAt = applied.AppliedAt

	for _, resource := range parseBundle(version.Resources) {
		id := resource.ID
		if id == "" {
			id = resource.Name
		}
		shown, skipped := s.show(ctx, resource, id)
		if skipped != nil {
			configuration.Skipped = append(configuration.Skipped, *skipped)
			continue
		}
		configuration.Resources = append(configuration.Resources, *shown)
	}
	return configuration, nil
}

// show reads one document as its resource and the parts it has, or says why it cannot.
func (s *appliedConfigurationService) show(ctx context.Context, resource bundleResource,
	id string) (*AppliedResource, *SkippedResource) {
	skip := func(svcErr tidcommon.ServiceError) *SkippedResource {
		return &SkippedResource{ResourceType: resource.Type, ID: id, Code: svcErr.Code,
			Reason: svcErr.Error.DefaultValue}
	}
	viewer, ok := s.viewers[resource.Type]
	if !ok {
		return nil, skip(ErrorResourceNotViewable)
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(quotePlaceholders(resource.Content)), &root); err != nil ||
		len(root.Content) == 0 {
		return nil, skip(ErrorResourceNotReadable)
	}
	document := root.Content[0]

	view, err := viewer.ViewResource(ctx, document)
	shown := &AppliedResource{ResourceType: resource.Type, ID: id, Resource: view}
	if partViewer, ok := viewer.(declarativeresource.ResourcePartViewer); ok && err == nil {
		shown.Parts, err = partViewer.ViewResourceParts(ctx, document)
	}
	if err != nil {
		s.logger.Error(ctx, "Failed to show an applied document as its resource",
			log.String("resourceType", resource.Type), log.Error(err))
		return nil, skip(ErrorResourceNotReadable)
	}
	return shown, nil
}

// scalarPlaceholder is a template placeholder standing as a whole value, as the export writes a
// parameterized scalar. Unquoted, YAML reads it as a mapping rather than as the value it stands for.
var scalarPlaceholder = regexp.MustCompile(`(?m)(:[ \t]+|^[ \t]*-[ \t]+)(\{\{\.[A-Za-z_][A-Za-z0-9_]*\}\})[ \t]*$`)

// quotePlaceholders makes each placeholder that stands as a whole value a string, so a document that
// is otherwise YAML reads as one and shows its placeholders as they are written. A template range is
// left as it is, since no quoting makes it YAML.
func quotePlaceholders(document string) string {
	return scalarPlaceholder.ReplaceAllString(document, `$1"$2"`)
}
