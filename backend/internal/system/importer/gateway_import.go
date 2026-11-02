// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"

	"github.com/thunder-id/thunderid/internal/gateway"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// gatewayAdapter is the subset of the gateway service an import uses.
//
// A gateway is addressed by name here rather than by id, because a bootstrap file is written by a
// person and the id is generated. Finding the existing one therefore means listing.
type gatewayAdapter interface {
	List(ctx context.Context) ([]gateway.Gateway, *common.ServiceError)
	Register(ctx context.Context, req gateway.RegisterRequest) (*gateway.Registration, *common.ServiceError)
	Update(ctx context.Context, id string, req gateway.UpdateRequest) (*gateway.Gateway, *common.ServiceError)
}

// gatewayDeclarativeYAML is a gateway document as a bootstrap or import file carries it. It is the
// same shape a declared gateway file uses, so the two are interchangeable to whoever writes them.
type gatewayDeclarativeYAML struct {
	Name          string `yaml:"name"`
	BaseURL       string `yaml:"baseUrl"`
	Key           string `yaml:"key,omitempty"`
	CACertificate string `yaml:"caCertificate,omitempty"`
}

// importGateway registers a gateway, or updates the one already registered under that name.
//
// This is the one-time path: it writes to the database and what it writes outlives the file. That
// is the difference from a gateway declared in config/resources/gateways, which is held in memory
// and reconciled from its file on every start.
func (s *importService) importGateway(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool) ImportItemOutcome {
	if s.gatewayService == nil {
		return unsupportedAdapterOutcome(resourceTypeGateway, "gateway")
	}

	var req gatewayDeclarativeYAML
	if err := doc.Node.Decode(&req); err != nil {
		return decodeErrorOutcome(resourceTypeGateway, "", req.Name, err)
	}
	if req.Name == "" || req.BaseURL == "" {
		return ImportItemOutcome{
			ResourceType: resourceTypeGateway,
			ResourceName: req.Name,
			Status:       statusFailed,
			Code:         ErrorInvalidYAMLContent.Code,
			Message:      "a gateway needs a name and a baseUrl",
		}
	}

	existing, svcErr := s.findGatewayByName(ctx, req.Name)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeGateway, "", req.Name, operationCreate, svcErr)
	}

	if dryRun {
		if existing != nil && options.IsUpsertEnabled() {
			return successOutcome(resourceTypeGateway, existing.ID, req.Name, operationUpdate)
		}
		return successOutcome(resourceTypeGateway, "", req.Name, operationCreate)
	}

	if existing != nil {
		if !options.IsUpsertEnabled() {
			return ImportItemOutcome{
				ResourceType: resourceTypeGateway,
				ResourceID:   existing.ID,
				ResourceName: req.Name,
				Status:       statusFailed,
				Code:         ErrorInvalidYAMLContent.Code,
				Message:      "a gateway of that name is already registered",
			}
		}
		return s.updateImportedGateway(ctx, existing.ID, req)
	}

	// The key is optional. Supplying one is how a gateway configured first and this control plane
	// come to agree; omitting it has the control plane issue one, which the registration response
	// carries and nothing reads back afterwards.
	registration, svcErr := s.gatewayService.Register(ctx, gateway.RegisterRequest{
		Name:          req.Name,
		BaseURL:       req.BaseURL,
		Key:           req.Key,
		CACertificate: req.CACertificate,
	})
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeGateway, "", req.Name, operationCreate, svcErr)
	}
	return successOutcome(resourceTypeGateway, registration.ID, registration.Name, operationCreate)
}

// updateImportedGateway applies the document to the gateway already registered under that name.
//
// A field the document omits is left as it is, which matters most for the key: re-running bootstrap
// over a file that names no key must not rotate the credential and leave the gateway unreachable.
func (s *importService) updateImportedGateway(
	ctx context.Context, id string, req gatewayDeclarativeYAML) ImportItemOutcome {
	update := gateway.UpdateRequest{Name: &req.Name, BaseURL: &req.BaseURL}
	if req.Key != "" {
		update.Key = &req.Key
	}
	if req.CACertificate != "" {
		update.CACertificate = &req.CACertificate
	}

	updated, svcErr := s.gatewayService.Update(ctx, id, update)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeGateway, id, req.Name, operationUpdate, svcErr)
	}
	return successOutcome(resourceTypeGateway, updated.ID, updated.Name, operationUpdate)
}

// findGatewayByName returns the registered gateway of that name, or nil when there is none.
func (s *importService) findGatewayByName(ctx context.Context, name string) (*gateway.Gateway, *common.ServiceError) {
	registered, svcErr := s.gatewayService.List(ctx)
	if svcErr != nil {
		return nil, svcErr
	}
	for i := range registered {
		if registered[i].Name == name {
			return &registered[i], nil
		}
	}
	return nil, nil //nolint:nilnil // absent is not an error; the caller registers instead
}
