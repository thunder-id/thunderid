// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const (
	resourceTypeProject = "project"
	paramTypeProject    = "Project"
)

// projectDocument is a project as it is exported and imported. The root organization unit is
// exported as a document of its own, which carries the rest of its settings.
type projectDocument struct {
	ID          string `json:"id" yaml:"id"`
	RootOUID    string `json:"rootOuId" yaml:"rootOuId"`
	Handle      string `json:"handle" yaml:"handle"`
	DisplayName string `json:"displayName" yaml:"displayName"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// projectExporter implements declarativeresource.ResourceExporter for projects.
type projectExporter struct {
	service ProjectServiceInterface
}

// newProjectExporter creates a project exporter.
func newProjectExporter(service ProjectServiceInterface) *projectExporter {
	return &projectExporter{service: service}
}

// GetResourceType returns the resource type for projects.
func (e *projectExporter) GetResourceType() string {
	return resourceTypeProject
}

// GetParameterizerType returns the parameterizer type for projects.
func (e *projectExporter) GetParameterizerType() string {
	return paramTypeProject
}

// GetAllResourceIDs returns the IDs of every project, oldest first.
func (e *projectExporter) GetAllResourceIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	var ids []string
	for offset := 0; ; {
		page, svcErr := e.service.GetProjectList(ctx, serverconst.MaxPageSize, offset)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, project := range page.Projects {
			ids = append(ids, project.ID)
		}
		offset += len(page.Projects)
		if len(page.Projects) == 0 || offset >= page.TotalResults {
			return ids, nil
		}
	}
}

// GetResourceByID returns the project with the given ID as it is exported.
func (e *projectExporter) GetResourceByID(
	ctx context.Context, id string) (interface{}, string, *tidcommon.ServiceError) {
	project, svcErr := e.service.GetProject(ctx, id)
	if svcErr != nil {
		return nil, "", svcErr
	}
	return &projectDocument{
		ID:          project.ID,
		RootOUID:    project.RootOUID,
		Handle:      project.Handle,
		DisplayName: project.DisplayName,
		Description: project.Description,
	}, project.DisplayName, nil
}

// ValidateResource validates an exported project and returns its name.
func (e *projectExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	project, ok := resource.(*projectDocument)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeProject, id)
	}
	if err := declarativeresource.ValidateResourceName(ctx,
		project.DisplayName, resourceTypeProject, id, "PROJECT_VALIDATION_ERROR", logger); err != nil {
		return "", err
	}
	return project.DisplayName, nil
}

// GetResourceRules returns the parameterization rules for projects. A project has no
// environment-specific values.
func (e *projectExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{
		Variables:      []string{},
		ArrayVariables: []string{},
	}
}
