// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// defaultMaxGateways is what a deployment administers when nothing is configured. A standalone
// deployment pairs with one data plane.
//
// The number is a guard, not an invariant: see MaxGateways in the engine config for what it does and
// does not promise.
const defaultMaxGateways = 1

// ServiceInterface is the gateway management surface.
type ServiceInterface interface {
	List(ctx context.Context) ([]Gateway, *tidcommon.ServiceError)
	Get(ctx context.Context, id string) (*Gateway, *tidcommon.ServiceError)
	Register(ctx context.Context, req RegisterRequest) (*Gateway, *tidcommon.ServiceError)
	Delete(ctx context.Context, id string) *tidcommon.ServiceError
	// Adopt registers a gateway declared in a file, or updates the one already registered under
	// that name. It is what makes reading those files on every start idempotent.
	Adopt(ctx context.Context, req RegisterRequest) *tidcommon.ServiceError
}

type service struct {
	store  storeInterface
	logger *log.Logger
}

func newService(store storeInterface) ServiceInterface {
	return &service{
		store:  store,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GatewayService")),
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
		gateways[i].Key = ""
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
	gw.Key = ""
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
	dataPlaneID := strings.TrimSpace(req.DataPlaneID)
	if strings.TrimSpace(req.BaseURL) == "" || dataPlaneID == "" || strings.TrimSpace(req.Key) == "" {
		return nil, &ErrorGatewayConnectionRequired
	}
	baseURL, ok := normalizeBaseURL(req.BaseURL)
	if !ok {
		return nil, &ErrorInvalidBaseURL
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

	// One data plane registers once. Registering the same one twice, typically under its internal
	// and its external hostname, leaves two rows that are applied to independently and drift apart.
	owner, err := s.store.GetByDataPlaneID(ctx, dataPlaneID)
	if err != nil {
		s.logger.Error(ctx, "Failed to check the data plane", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if owner != nil {
		return nil, &ErrorDataPlaneAlreadyRegistered
	}

	// The key is held the way every other stored credential is, encrypted with the deployment's
	// configuration key rather than in the clear.
	stored, svcErr := s.protect(ctx, req.Key)
	if svcErr != nil {
		return nil, svcErr
	}

	gw := &Gateway{
		ID:            uuid.New().String(),
		Name:          name,
		DataPlaneID:   dataPlaneID,
		BaseURL:       baseURL,
		Key:           stored,
		CACertificate: strings.TrimSpace(req.CACertificate),
	}
	// The insert carries the capacity check and returns the row it wrote, so what comes back is what
	// is stored, timestamps included. No row means the limit refused it.
	registered, err := s.store.Create(ctx, gw, maxGateways())
	if err != nil {
		s.logger.Error(ctx, "Failed to register the gateway", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if registered == nil {
		return nil, &ErrorGatewayLimitReached
	}

	s.logger.Info(ctx, "Registered a gateway", log.String("gatewayId", registered.ID),
		log.String("dataPlaneId", registered.DataPlaneID))
	registered.Key = ""
	return registered, nil
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
	dataPlaneID := strings.TrimSpace(req.DataPlaneID)
	if strings.TrimSpace(req.BaseURL) == "" || dataPlaneID == "" || strings.TrimSpace(req.Key) == "" {
		return &ErrorGatewayConnectionRequired
	}
	baseURL, ok := normalizeBaseURL(req.BaseURL)
	if !ok {
		return &ErrorInvalidBaseURL
	}

	existing, err := s.store.GetByName(ctx, name)
	if err != nil {
		s.logger.Error(ctx, "Failed to look up the gateway", log.Error(err))
		return &tidcommon.InternalServerError
	}

	stored, svcErr := s.protect(ctx, req.Key)
	if svcErr != nil {
		return svcErr
	}

	// A declared gateway may not claim a data plane another one already owns, whether it is being
	// created or updated. Without this the unique constraint refuses the write as a database error.
	owner, err := s.store.GetByDataPlaneID(ctx, dataPlaneID)
	if err != nil {
		s.logger.Error(ctx, "Failed to check the data plane", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if owner != nil && (existing == nil || owner.ID != existing.ID) {
		return &ErrorDataPlaneAlreadyRegistered
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
			DataPlaneID:   dataPlaneID,
			BaseURL:       baseURL,
			Key:           stored,
			CACertificate: strings.TrimSpace(req.CACertificate),
		}
		created, err := s.store.Create(ctx, gw, maxGateways())
		if err != nil {
			s.logger.Error(ctx, "Failed to register the declared gateway", log.Error(err))
			return &tidcommon.InternalServerError
		}
		if created == nil {
			return &ErrorGatewayLimitReached
		}
		return nil
	}

	existing.BaseURL = baseURL
	existing.DataPlaneID = dataPlaneID
	existing.Key = stored
	existing.CACertificate = strings.TrimSpace(req.CACertificate)
	if err := s.store.Update(ctx, existing); err != nil {
		s.logger.Error(ctx, "Failed to update the declared gateway", log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// normalizeBaseURL trims a base URL to how it is stored, and reports whether it is one at all.
//
// Emptiness is not enough of a check. "/" trims to nothing and "https://" trims to "https:", and both
// would be stored as an address no call can ever reach. The failure would surface much later, as a
// gateway that looks registered and never answers, so it is refused here where the caller can still
// see which field was wrong.
func normalizeBaseURL(raw string) (string, bool) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return "", false
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	return trimmed, true
}

// protect encrypts a management token for storage.
func (s *service) protect(ctx context.Context, secret string) (string, *tidcommon.ServiceError) {
	property, err := cmodels.NewProperty("key", secret, true)
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
