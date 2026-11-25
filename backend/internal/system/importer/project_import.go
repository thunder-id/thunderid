// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
)

// projectDocument is a declared project with the ID another system gave it.
type projectDocument struct {
	ID          string `yaml:"id"`
	Handle      string `yaml:"handle"`
	DisplayName string `yaml:"displayName"`
	Description string `yaml:"description"`
}

// importProject imports a declared project. Where projects are not served the document is skipped,
// so a Control Plane export applies to a Data Plane unchanged.
func (s *importService) importProject(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool,
) ImportItemOutcome {
	var req projectDocument
	if err := doc.Node.Decode(&req); err != nil {
		return decodeErrorOutcome(resourceTypeProject, req.ID, req.DisplayName, err)
	}

	if s.projectService == nil {
		outcome := successOutcome(resourceTypeProject, req.ID, req.DisplayName, operationSkip)
		outcome.Message = "Projects are served only by a Control Plane; the project was not imported"
		return outcome
	}

	if dryRun {
		exists, svcErr := s.projectService.ProjectExists(ctx, req.ID)
		if svcErr != nil {
			return serviceErrorOutcome(resourceTypeProject, req.ID, req.DisplayName, operationCreate, svcErr)
		}
		if exists && options.IsUpsertEnabled() {
			return successOutcome(resourceTypeProject, req.ID, req.DisplayName, operationUpdate)
		}
		return successOutcome(resourceTypeProject, req.ID, req.DisplayName, operationCreate)
	}

	created, svcErr := s.projectService.ImportProject(ctx, req.ID, req.Handle, req.DisplayName,
		req.Description, options.IsUpsertEnabled())
	operation := operationUpdate
	if created {
		operation = operationCreate
	}
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeProject, req.ID, req.DisplayName, operation, svcErr)
	}
	return successOutcome(resourceTypeProject, req.ID, req.DisplayName, operation)
}
