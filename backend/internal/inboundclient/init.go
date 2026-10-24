// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package inboundclient

import (
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/cimd"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/cache"
	dre "github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Option adjusts the inbound client service for the plane it runs on.
type Option func(*inboundClientService)

// KeepingValueReferences is for a control plane, which keeps a reference to the redirect URIs each
// deployment holds where the URIs belong.
func KeepingValueReferences() Option {
	return func(s *inboundClientService) { s.keepsReferences = true }
}

// Initialize initializes the inbound client service.
func Initialize(
	cacheManager cache.CacheManagerInterface,
	certService cert.CertificateServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	themeMgt thememgt.ThemeMgtServiceInterface,
	layoutMgt layoutmgt.LayoutMgtServiceInterface,
	flowMgt flowmgt.FlowMgtServiceInterface,
	entityType entitytype.EntityTypeServiceInterface,
	cryptoProvider providers.RuntimeCryptoProvider,
	jweService jwe.JWEServiceInterface,
	cimdService cimd.CIMDServiceInterface,
	sharingService sharing.SharingServiceInterface,
	sharedTypes map[providers.EntityCategory]sharing.ResourceType,
	options ...Option,
) (InboundClientServiceInterface, error) {
	store, transactioner, err := initializeStore(cacheManager)
	if err != nil {
		return nil, err
	}
	service := newInboundClientService(store, transactioner, certService, entityProvider,
		themeMgt, layoutMgt, flowMgt, entityType, cryptoProvider, jweService, cimdService,
		sharingService, sharedTypes)
	for _, option := range options {
		option(service)
	}
	return service, nil
}

// initializeStore always creates a composite store (DB + in-memory file store).
func initializeStore(cacheManager cache.CacheManagerInterface) (
	inboundClientStoreInterface, providers.Transactioner, error) {
	fileStore := newFileBasedStore(dre.KeyTypeInboundAuth)
	dbStore, transactioner, err := newStore()
	if err != nil {
		return nil, nil, err
	}
	inboundClientCache := cache.GetCache[*inboundmodel.InboundClient](cacheManager, inboundClientCacheName)
	oauthProfileCache := cache.GetCache[*providers.OAuthProfile](cacheManager, oauthProfileCacheName)
	cached := newCachedBackStore(dbStore, inboundClientCache, oauthProfileCache)
	return newCompositeStore(fileStore, cached), transactioner, nil
}
