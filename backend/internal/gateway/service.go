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
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ServiceInterface is the gateway management surface.
type ServiceInterface interface {
	List(ctx context.Context) ([]Gateway, *tidcommon.ServiceError)
	Get(ctx context.Context, id string) (*Gateway, *tidcommon.ServiceError)
	// Register records a gateway and issues the key this control plane will present to it. The
	// key is in the result and nowhere else afterwards.
	Register(ctx context.Context, req RegisterRequest) (*Registration, *tidcommon.ServiceError)
	// Update changes a registration's name, address or certificate. What the request omits is left
	// alone.
	Update(ctx context.Context, id string, req UpdateRequest) (*Gateway, *tidcommon.ServiceError)
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
// maxGateways is how many gateways this deployment may administer.
//
// The value comes from configuration and nothing here supplies a fallback: config/default.json
// carries it, so an unset deployment.yaml still reads a number rather than this code deciding one.
func maxGateways() int {
	if !config.IsServerRuntimeInitialized() {
		return 0
	}
	return config.GetServerRuntime().Config.Gateway.MaxGateways
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

// Register records a gateway this control plane administers.
func (s *service) Register(ctx context.Context,
	req RegisterRequest) (*Registration, *tidcommon.ServiceError) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, &ErrorGatewayNameRequired
	}
	// Without an address there is no way to reach the gateway, and a registration that cannot be
	// applied to is worse than none: it looks configured.
	if strings.TrimSpace(req.BaseURL) == "" {
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

	// One gateway registers once, and its address is what decides which one it is. Two rows for
	// the same deployment would be applied to independently and drift apart.
	owner, err := s.store.GetByBaseURL(ctx, baseURL)
	if err != nil {
		s.logger.Error(ctx, "Failed to check the gateway address", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if owner != nil {
		return nil, &ErrorGatewayAlreadyRegistered
	}

	// A registration that names no key gets one. Generating it here means it is not typed, mailed
	// or committed on the way in, which is the usual way to register. A caller that already holds
	// one, because the gateway was configured with it first, supplies it instead and that is
	// what is stored. Either way it is held encrypted with the deployment's configuration key, the
	// way every other stored credential is.
	key := strings.TrimSpace(req.Key)
	if key == "" {
		generated, svcErr := s.newKey(ctx)
		if svcErr != nil {
			return nil, svcErr
		}
		key = generated
	}
	stored, svcErr := s.protect(ctx, key)
	if svcErr != nil {
		return nil, svcErr
	}

	gw := &Gateway{
		ID:            uuid.New().String(),
		Name:          name,
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
		log.String("baseUrl", registered.BaseURL))
	registered.Key = ""
	// The generated key travels in the response and is not kept anywhere readable.
	return &Registration{Gateway: *registered, Key: key}, nil
}

// newKey generates the token this control plane will present to a gateway.
func (s *service) newKey(ctx context.Context) (string, *tidcommon.ServiceError) {
	key, err := cryptolib.GenerateSecureToken()
	if err != nil {
		s.logger.Error(ctx, "Failed to generate a gateway credential", log.Error(err))
		return "", &tidcommon.InternalServerError
	}
	return key, nil
}

// Update applies the fields the request names and leaves the rest as they are.
func (s *service) Update(ctx context.Context, id string,
	req UpdateRequest) (*Gateway, *tidcommon.ServiceError) {
	if strings.TrimSpace(id) == "" {
		return nil, &ErrorInvalidGatewayID
	}
	if req.Name == nil && req.BaseURL == nil && req.CACertificate == nil && req.Key == nil {
		return nil, &ErrorNothingToUpdate
	}

	existing, err := s.store.GetByID(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "Failed to read the gateway", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if existing == nil {
		return nil, &ErrorGatewayNotFound
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, &ErrorGatewayNameRequired
		}
		// A name is how a declared gateway is matched, so two registrations may not share one. Its
		// own row is not a clash: renaming a gateway to what it is already called is a no-op, not a
		// conflict.
		if clash, clashErr := s.store.GetByName(ctx, name); clashErr != nil {
			s.logger.Error(ctx, "Failed to check the gateway name", log.Error(clashErr))
			return nil, &tidcommon.InternalServerError
		} else if clash != nil && clash.ID != existing.ID {
			return nil, &ErrorGatewayNameTaken
		}
		existing.Name = name
	}

	if req.BaseURL != nil {
		baseURL, ok := normalizeBaseURL(*req.BaseURL)
		if !ok {
			return nil, &ErrorInvalidBaseURL
		}
		// The address identifies the gateway, so moving one onto an address another already
		// answers at would leave two registrations for the same deployment. Its own address is not a
		// clash: re-sending the address it already has is a no-op.
		if owner, ownerErr := s.store.GetByBaseURL(ctx, baseURL); ownerErr != nil {
			s.logger.Error(ctx, "Failed to check the gateway address", log.Error(ownerErr))
			return nil, &tidcommon.InternalServerError
		} else if owner != nil && owner.ID != existing.ID {
			return nil, &ErrorGatewayAlreadyRegistered
		}
		existing.BaseURL = baseURL
	}

	if req.CACertificate != nil {
		existing.CACertificate = strings.TrimSpace(*req.CACertificate)
	}

	// A key the edit carries replaces the stored one, which is how a credential is rotated: the
	// same value then has to reach the gateway, which holds it as its management API key.
	//
	// An edit that carries none keeps it. Nothing is generated here, because a caller who renamed a
	// gateway did not ask for its credential to change, and a new one would stop the gateway
	// being reachable until it was given the new value.
	if req.Key != nil {
		key := strings.TrimSpace(*req.Key)
		if key == "" {
			return nil, &ErrorGatewayKeyRequired
		}
		stored, svcErr := s.protect(ctx, key)
		if svcErr != nil {
			return nil, svcErr
		}
		existing.Key = stored
	}

	if err := s.store.Update(ctx, existing); err != nil {
		s.logger.Error(ctx, "Failed to update the gateway", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	s.logger.Info(ctx, "Updated a gateway", log.String("gatewayId", existing.ID))
	existing.Key = ""
	return existing, nil
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
	if strings.TrimSpace(req.BaseURL) == "" {
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

	// A declared gateway may not claim an address another gateway already owns, whether it is being
	// created or updated. Without this the unique constraint refuses the write as a database error.
	owner, err := s.store.GetByBaseURL(ctx, baseURL)
	if err != nil {
		s.logger.Error(ctx, "Failed to check the gateway address", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if owner != nil && (existing == nil || owner.ID != existing.ID) {
		return &ErrorGatewayAlreadyRegistered
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
		// First time this file has been seen, so it needs a key like any other registration: the one
		// the declaration names, or a generated one when it names none.
		key := strings.TrimSpace(req.Key)
		if key == "" {
			generated, svcErr := s.newKey(ctx)
			if svcErr != nil {
				return svcErr
			}
			key = generated
		}
		stored, svcErr := s.protect(ctx, key)
		if svcErr != nil {
			return svcErr
		}
		gw := &Gateway{
			ID: uuid.New().String(), Name: name,
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
	existing.CACertificate = strings.TrimSpace(req.CACertificate)

	// A declaration naming a key sets it, because the file is the state this gateway is meant to be
	// in and an operator who changed the key there meant the change.
	//
	// A declaration naming none leaves the stored key alone. These files are read on every start, so
	// minting a new one here would rotate the credential on each restart and drop the gateway
	// until someone noticed.
	if declaredKey := strings.TrimSpace(req.Key); declaredKey != "" {
		stored, svcErr := s.protect(ctx, declaredKey)
		if svcErr != nil {
			return svcErr
		}
		existing.Key = stored
	}

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
