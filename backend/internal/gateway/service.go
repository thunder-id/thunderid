// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// defaultMaxGateways is what a deployment administers when nothing is configured. A standalone
// deployment pairs with one data plane.
const defaultMaxGateways = 1

// ServiceInterface is the gateway management surface.
type ServiceInterface interface {
	List(ctx context.Context) ([]Gateway, *tidcommon.ServiceError)
	Get(ctx context.Context, id string) (*Gateway, *tidcommon.ServiceError)
	Register(ctx context.Context, req RegisterRequest) (*Gateway, *tidcommon.ServiceError)
	Delete(ctx context.Context, id string) *tidcommon.ServiceError
	// Apply sends the configuration this control plane currently holds to the gateway.
	Apply(ctx context.Context, id string) (*ApplyResult, *tidcommon.ServiceError)
	// Adopt registers a gateway declared in a file, or updates the one already registered under
	// that name. It is what makes reading those files on every start idempotent.
	Adopt(ctx context.Context, req RegisterRequest) *tidcommon.ServiceError
}

type service struct {
	store storeInterface
	// exporter yields the configuration an apply sends. It is nil on a server that registers
	// gateways but does not apply to them, which is why Apply checks before using it.
	exporter  export.ExportServiceInterface
	dataPlane applier
	logger    *log.Logger
}

func newService(store storeInterface, exporter export.ExportServiceInterface, dataPlane applier) ServiceInterface {
	return &service{
		store:     store,
		exporter:  exporter,
		dataPlane: dataPlane,
		logger:    log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GatewayService")),
	}
}

// maxGateways is how many gateways this deployment may administer.
func maxGateways() int {
	if !config.IsServerRuntimeInitialized() {
		return defaultMaxGateways
	}
	if configured := config.GetServerRuntime().Config.Server.MaxGateways; configured > 0 {
		return configured
	}
	return defaultMaxGateways
}

func (s *service) List(ctx context.Context) ([]Gateway, *tidcommon.ServiceError) {
	gateways, err := s.store.List(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to list the gateways", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	for i := range gateways {
		gateways[i].ClientSecret = ""
	}
	return gateways, nil
}

func (s *service) Get(ctx context.Context, id string) (*Gateway, *tidcommon.ServiceError) {
	if strings.TrimSpace(id) == "" {
		return nil, &ErrorInvalidGatewayID
	}
	gw, err := s.store.GetByID(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "Failed to read the gateway", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if gw == nil {
		return nil, &ErrorGatewayNotFound
	}
	gw.ClientSecret = ""
	return gw, nil
}

// Register records a data plane this control plane administers.
func (s *service) Register(ctx context.Context, req RegisterRequest) (*Gateway, *tidcommon.ServiceError) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, &ErrorGatewayNameRequired
	}
	// Without all three there is no way to reach the data plane, and a registration that cannot be
	// applied to is worse than none: it looks configured.
	if strings.TrimSpace(req.BaseURL) == "" || strings.TrimSpace(req.ClientID) == "" ||
		strings.TrimSpace(req.ClientSecret) == "" {
		return nil, &ErrorGatewayConnectionRequired
	}

	count, err := s.store.Count(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to count the gateways", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if count >= maxGateways() {
		return nil, &ErrorGatewayLimitReached
	}

	existing, err := s.store.GetByName(ctx, name)
	if err != nil {
		s.logger.Error(ctx, "Failed to check the gateway name", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if existing != nil {
		return nil, &ErrorGatewayNameTaken
	}

	// The client secret is held the way every other stored credential is, encrypted with the
	// deployment's configuration key rather than in the clear.
	stored, svcErr := s.protect(ctx, req.ClientSecret)
	if svcErr != nil {
		return nil, svcErr
	}

	gw := &Gateway{
		ID:           uuid.New().String(),
		Name:         name,
		BaseURL:      strings.TrimRight(strings.TrimSpace(req.BaseURL), "/"),
		ClientID:     strings.TrimSpace(req.ClientID),
		ClientSecret: stored,
		Scope:        strings.TrimSpace(req.Scope),
	}
	if err := s.store.Create(ctx, gw); err != nil {
		s.logger.Error(ctx, "Failed to register the gateway", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	s.logger.Info(ctx, "Registered a gateway", log.String("gatewayId", gw.ID))
	gw.ClientSecret = ""
	return gw, nil
}

func (s *service) Delete(ctx context.Context, id string) *tidcommon.ServiceError {
	if strings.TrimSpace(id) == "" {
		return &ErrorInvalidGatewayID
	}
	existing, err := s.store.GetByID(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "Failed to read the gateway", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if existing == nil {
		return &ErrorGatewayNotFound
	}
	if err := s.store.Delete(ctx, id); err != nil {
		s.logger.Error(ctx, "Failed to remove the gateway", log.Error(err))
		return &tidcommon.InternalServerError
	}
	s.logger.Info(ctx, "Removed a gateway", log.String("gatewayId", id))
	return nil
}

// Adopt registers a declared gateway, or updates the one already registered under that name.
//
// A file names a gateway; it cannot know an id this server generated. Matching on the name is what
// lets the same file be read on every start without registering the gateway again, and lets an
// edited file change the gateway it already described.
func (s *service) Adopt(ctx context.Context, req RegisterRequest) *tidcommon.ServiceError {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return &ErrorGatewayNameRequired
	}
	if strings.TrimSpace(req.BaseURL) == "" || strings.TrimSpace(req.ClientID) == "" ||
		strings.TrimSpace(req.ClientSecret) == "" {
		return &ErrorGatewayConnectionRequired
	}

	existing, err := s.store.GetByName(ctx, name)
	if err != nil {
		s.logger.Error(ctx, "Failed to look up the gateway", log.Error(err))
		return &tidcommon.InternalServerError
	}

	stored, svcErr := s.protect(ctx, req.ClientSecret)
	if svcErr != nil {
		return svcErr
	}

	if existing == nil {
		count, err := s.store.Count(ctx)
		if err != nil {
			s.logger.Error(ctx, "Failed to count the gateways", log.Error(err))
			return &tidcommon.InternalServerError
		}
		if count >= maxGateways() {
			return &ErrorGatewayLimitReached
		}
		gw := &Gateway{
			ID: uuid.New().String(), Name: name,
			BaseURL:  strings.TrimRight(strings.TrimSpace(req.BaseURL), "/"),
			ClientID: strings.TrimSpace(req.ClientID), ClientSecret: stored,
			Scope: strings.TrimSpace(req.Scope),
		}
		if err := s.store.Create(ctx, gw); err != nil {
			s.logger.Error(ctx, "Failed to register the declared gateway", log.Error(err))
			return &tidcommon.InternalServerError
		}
		return nil
	}

	existing.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	existing.ClientID = strings.TrimSpace(req.ClientID)
	existing.ClientSecret = stored
	existing.Scope = strings.TrimSpace(req.Scope)
	if err := s.store.Update(ctx, existing); err != nil {
		s.logger.Error(ctx, "Failed to update the declared gateway", log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// protect encrypts a client secret for storage.
func (s *service) protect(ctx context.Context, secret string) (string, *tidcommon.ServiceError) {
	property, err := cmodels.NewProperty("clientSecret", secret, true)
	if err != nil {
		s.logger.Error(ctx, "Failed to protect the gateway credential", log.Error(err))
		return "", &tidcommon.InternalServerError
	}
	stored, err := cmodels.SerializePropertiesToJSONArray([]cmodels.Property{*property})
	if err != nil {
		s.logger.Error(ctx, "Failed to protect the gateway credential", log.Error(err))
		return "", &tidcommon.InternalServerError
	}
	return stored, nil
}
