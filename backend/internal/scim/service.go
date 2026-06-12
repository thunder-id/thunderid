package scim

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/user"
)

// SCIMServiceInterface defines the SCIM service operations.
type SCIMServiceInterface interface {
	GetServiceProviderConfig(ctx context.Context, baseURL string) SCIMServiceProviderConfig
}

// scimService coordinates SCIM operations, delegating user and entity type
// operations to existing ThunderID services.
type scimService struct {
	userService       user.UserServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	cfg               config.SCIMConfig

	// configVersion is a short deterministic hash of the SCIM config used
	// as the ETag/version value. It changes only when operator toggles a
	// capability flag — not on every server restart.
	configVersion string
}

// newSCIMService creates a new scimService instance.
func newSCIMService(
	userService user.UserServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	cfg config.SCIMConfig,
) *scimService {
	return &scimService{
		userService:       userService,
		entityTypeService: entityTypeService,
		cfg:               cfg,
		configVersion:     computeSCIMConfigVersion(cfg),
	}
}

// computeSCIMConfigVersion produces a stable weak ETag value from the SCIM
// config JSON. The format follows RFC 7232 weak validator convention: W/"<value>".
// It changes whenever an operator toggles a capability flag, ensuring SCIM
// clients can detect ServiceProviderConfig changes via conditional GET.
func computeSCIMConfigVersion(cfg config.SCIMConfig) string {
	b, _ := json.Marshal(cfg)
	h := sha256.Sum256(b)
	return fmt.Sprintf(`W/"%s"`, hex.EncodeToString(h[:8]))
}

func (s *scimService) GetServiceProviderConfig(_ context.Context, baseURL string) SCIMServiceProviderConfig {
	location := baseURL + "/scim/v2/ServiceProviderConfig"

	meta := SCIMMeta{
		ResourceType: "ServiceProviderConfig",
		Created:      scimServiceProviderConfigCreated,
		LastModified: scimServiceProviderConfigCreated, // equals Created — resource never modified by users
		Location:     location,
	}

	// RFC 7643 §3.1: "version" is optional and subject to etag support.
	// Only include it when the server advertises ETag support.
	if s.cfg.ETagSupported {
		meta.Version = s.configVersion
	}

	return SCIMServiceProviderConfig{
		Schemas: []string{SCIMServiceProviderConfigSchemaURN},
		Patch:   SCIMSupportedFeature{Supported: s.cfg.PatchSupported},
		Bulk: SCIMBulkConfig{
			Supported:      s.cfg.BulkSupported,
			MaxOperations:  s.cfg.BulkMaxOperations,
			MaxPayloadSize: s.cfg.BulkMaxPayloadSize,
		},
		Filter: SCIMFilterConfig{
			Supported:  s.cfg.FilterSupported,
			MaxResults: s.cfg.FilterMaxResults,
		},
		ChangePassword: SCIMSupportedFeature{Supported: s.cfg.ChangePasswordSupported},
		Sort:           SCIMSupportedFeature{Supported: s.cfg.SortSupported},
		ETag:           SCIMSupportedFeature{Supported: s.cfg.ETagSupported},
		AuthenticationSchemes: []SCIMAuthenticationScheme{
			{
				Type:        "oauthbearertoken",
				Name:        "OAuth Bearer Token",
				Description: "Authentication using an OAuth 2.0 Bearer Token",
			},
		},
		Meta: meta,
	}
}
