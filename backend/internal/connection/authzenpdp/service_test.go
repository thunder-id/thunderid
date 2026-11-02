// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

// newTestOUService is an organization unit service in which "ou-1" exists.
func newTestOUService(t *testing.T) *oumock.OrganizationUnitServiceInterfaceMock {
	t.Helper()
	ouService := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouService.On("ResolveOwner", mock.Anything, "ou-1", "", mock.Anything).Return("ou-1", nil).Maybe()
	ouService.On("ResolveOwnerForUpdate", mock.Anything, "ou-1", "", "ou-1", mock.Anything).
		Return("ou-1", nil).Maybe()
	ouService.EXPECT().ResolveOwnerForUpdate(mock.Anything, "", "", mock.Anything, mock.Anything).
		RunAndReturn(func(
			_ context.Context, _, _, currentID string, _ ou.OwnerErrors,
		) (string, *tidcommon.ServiceError) {
			return currentID, nil
		}).Maybe()
	return ouService
}

// newTestAuthZENPDPService creates the service over the given store and defaults, with an organization unit service in
// which "ou-1" exists.
func newTestAuthZENPDPService(
	t *testing.T, store authZENPDPStoreInterface, defaults config.AuthZENPDPConfig,
) *AuthZENPDPService {
	return newAuthZENPDPService(store, defaults, transaction.NewNoOpTransactioner(), nil, newTestOUService(t))
}

type serviceStoreStub struct {
	connection  AuthZENPDPConnection
	connections []AuthZENPDPConnection
	id          string
	createCalls int
	updateCalls int
	deleteCalls int
	err         error
	updateErr   error
	missing     bool
}

type transactionerStub struct {
	calls int
}

func (t *transactionerStub) Transact(ctx context.Context, operation func(context.Context) error) error {
	t.calls++
	return operation(ctx)
}

func (s *serviceStoreStub) CreateAuthZENPDP(_ context.Context, connection AuthZENPDPConnection) error {
	s.createCalls++
	s.connection = connection
	return s.err
}

func (s *serviceStoreStub) ListAuthZENPDPs(context.Context) ([]AuthZENPDPConnection, error) {
	return s.connections, s.err
}

func (s *serviceStoreStub) CountAuthZENPDPs(context.Context) (int, error) {
	return len(s.connections), s.err
}

func (s *serviceStoreStub) GetAuthZENPDP(_ context.Context, id string) (*AuthZENPDPConnection, error) {
	s.id = id
	if s.err != nil {
		return nil, s.err
	}
	if s.missing {
		return nil, nil
	}
	return &s.connection, nil
}

func (s *serviceStoreStub) GetAuthZENPDPByName(
	_ context.Context, name string,
) (*AuthZENPDPConnection, error) {
	if s.connection.Name == name {
		return &s.connection, nil
	}
	return nil, nil
}

func (s *serviceStoreStub) UpdateAuthZENPDP(
	_ context.Context, id string, connection AuthZENPDPConnection,
) error {
	s.updateCalls++
	s.id = id
	s.connection = connection
	if s.updateErr != nil {
		return s.updateErr
	}
	return s.err
}

func (s *serviceStoreStub) DeleteAuthZENPDP(_ context.Context, id string) error {
	s.deleteCalls++
	s.id = id
	return s.err
}

func TestServiceDelegatesStoreOperations(t *testing.T) {
	ctx := context.Background()
	store := &serviceStoreStub{
		connections: []AuthZENPDPConnection{{ID: "pdp-1"}},
	}
	service := newTestAuthZENPDPService(t, store, config.AuthZENPDPConfig{})
	request := ConnectionRequest{
		OUID:          "ou-1",
		Name:          "AuthZEN PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
	}

	created, svcErr := service.CreateAuthZENPDPConnection(ctx, request)
	require.Nil(t, svcErr)
	require.Equal(t, *created, store.connection)

	connections, svcErr := service.ListAuthZENPDPs(ctx)
	require.Nil(t, svcErr)
	require.Equal(t, store.connections, connections)

	result, svcErr := service.GetAuthZENPDP(ctx, created.ID)
	require.Nil(t, svcErr)
	require.Equal(t, created.ID, store.id)
	require.Equal(t, store.connection, *result)

	updated, svcErr := service.UpdateAuthZENPDPConnection(ctx, created.ID, request)
	require.Nil(t, svcErr)
	require.Equal(t, created.ID, updated.ID)
	require.Equal(t, created.ID, store.id)
	require.Nil(t, service.DeleteAuthZENPDPConnection(ctx, created.ID))
	require.Equal(t, created.ID, store.id)
}

func TestServiceListAndGetTranslateStoreErrors(t *testing.T) {
	store := &serviceStoreStub{err: fmt.Errorf("store unavailable")}
	service := newTestAuthZENPDPService(t, store, config.AuthZENPDPConfig{})

	connections, listErr := service.ListAuthZENPDPs(context.Background())
	require.Nil(t, connections)
	require.Equal(t, tidcommon.InternalServerError.Code, listErr.Code)

	connection, getErr := service.GetAuthZENPDP(context.Background(), "pdp-1")
	require.Nil(t, connection)
	require.Equal(t, tidcommon.InternalServerError.Code, getErr.Code)
}

func TestServiceDeleteConnectionMapsImmutableStore(t *testing.T) {
	store := &serviceStoreStub{err: ErrAuthZENPDPIsImmutable}
	service := newTestAuthZENPDPService(t, store, config.AuthZENPDPConfig{})

	svcErr := service.DeleteAuthZENPDPConnection(context.Background(), "pdp-1")

	require.Equal(t, declarativeresource.ErrorDeclarativeResourceDeleteOperation.Code, svcErr.Code)
}

func TestServiceAppliesDefaultsToWritesAndReads(t *testing.T) {
	retries := 2
	store := &serviceStoreStub{
		connection:  AuthZENPDPConnection{ID: "pdp-1", Name: "Existing", TimeoutMS: -1, RetryCount: -1},
		connections: []AuthZENPDPConnection{{ID: "pdp-2", TimeoutMS: -1, RetryCount: -1}},
	}
	service := newTestAuthZENPDPService(t, store, config.AuthZENPDPConfig{TimeoutMS: 1200, RetryCount: &retries})

	created, svcErr := service.CreateAuthZENPDPConnection(context.Background(), ConnectionRequest{
		OUID:          "ou-1",
		Name:          "AuthZEN PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
	})
	require.Nil(t, svcErr)
	require.NotNil(t, created)
	require.Equal(t, 1200, store.connection.TimeoutMS)
	require.Equal(t, 2, store.connection.RetryCount)

	found, svcErr := service.GetAuthZENPDP(context.Background(), "pdp-1")
	require.Nil(t, svcErr)
	require.Equal(t, 1200, found.TimeoutMS)
	require.Equal(t, 2, found.RetryCount)

	connections, svcErr := service.ListAuthZENPDPs(context.Background())
	require.Nil(t, svcErr)
	require.Equal(t, 1200, connections[0].TimeoutMS)
	require.Equal(t, 2, connections[0].RetryCount)
}

func TestCreateConnectionReturnsAppliedDefaults(t *testing.T) {
	retries := 2
	store := &serviceStoreStub{}
	service := newTestAuthZENPDPService(t, store, config.AuthZENPDPConfig{TimeoutMS: 1200, RetryCount: &retries})

	created, svcErr := service.CreateAuthZENPDPConnection(context.Background(), ConnectionRequest{
		OUID:          "ou-1",
		ID:            "imported-pdp-id",
		Name:          "AuthZEN PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
	})

	require.Nil(t, svcErr)
	require.Equal(t, "imported-pdp-id", created.ID)
	require.Equal(t, 1200, created.TimeoutMS)
	require.Equal(t, 2, created.RetryCount)
	require.Equal(t, *created, store.connection)
}

func TestCreateConnectionUsesTransaction(t *testing.T) {
	store := &serviceStoreStub{}
	transactioner := &transactionerStub{}
	service := newAuthZENPDPService(store, config.AuthZENPDPConfig{}, transactioner, nil, newTestOUService(t))

	created, svcErr := service.CreateAuthZENPDPConnection(context.Background(), ConnectionRequest{
		OUID:          "ou-1",
		Name:          "AuthZEN PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
	})

	require.Nil(t, svcErr)
	require.NotEmpty(t, created.ID)
	require.Equal(t, 1, transactioner.calls)
	require.Equal(t, *created, store.connection)
}

var _ providers.Transactioner = (*transactionerStub)(nil)

func TestServiceRejectsInvalidConnectionBeforePersistence(t *testing.T) {
	store := &serviceStoreStub{}
	service := newTestAuthZENPDPService(t, store, config.AuthZENPDPConfig{})
	request := ConnectionRequest{
		OUID:          "ou-1",
		Name:          "AuthZEN PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "ftp://pdp.example.com/evaluations",
	}

	_, createErr := service.CreateAuthZENPDPConnection(context.Background(), request)
	_, updateErr := service.UpdateAuthZENPDPConnection(context.Background(), "pdp-1", request)
	require.NotNil(t, createErr)
	require.NotNil(t, updateErr)
	require.Zero(t, store.createCalls)
	require.Zero(t, store.updateCalls)
}

func TestServiceRejectsMaskedAPIKeyHeaderBeforePersistence(t *testing.T) {
	store := &serviceStoreStub{}
	service := newTestAuthZENPDPService(t, store, config.AuthZENPDPConfig{})
	request := ConnectionRequest{
		OUID:     "ou-1",
		Name:     "AuthZEN PDP",
		Endpoint: "https://pdp.example.com/evaluation",
		Authentication: &outboundauth.Authentication{
			Type:       string(outboundauth.TypeAPIKey),
			Properties: map[string]string{"X-API-Key": "******"},
		},
	}

	_, createErr := service.CreateAuthZENPDPConnection(context.Background(), request)
	_, updateErr := service.UpdateAuthZENPDPConnection(context.Background(), "pdp-1", request)

	require.Equal(t, "CON-1013", createErr.Code)
	require.Equal(t, "CON-1013", updateErr.Code)
	require.Zero(t, store.createCalls)
	require.Zero(t, store.updateCalls)
}

func TestServiceUpdateRejectsRemoteHTTPForStoredAuthentication(t *testing.T) {
	connection := AuthZENPDPConnection{
		ID:       "pdp-1",
		Name:     "AuthZEN PDP",
		Endpoint: "https://pdp.example.com/evaluation",
	}
	connection.AuthenticationProperties = testAuthProperties(t, outboundauth.Authentication{
		Type: string(outboundauth.TypeBearer), Properties: map[string]string{"token": "secret"},
	})
	store := &serviceStoreStub{connection: connection}
	service := newTestAuthZENPDPService(t, store, config.AuthZENPDPConfig{})

	_, svcErr := service.UpdateAuthZENPDPConnection(context.Background(), "pdp-1", ConnectionRequest{
		Name:     "AuthZEN PDP",
		Endpoint: "http://pdp.example.com/evaluation",
		Authentication: &outboundauth.Authentication{
			Type: string(outboundauth.TypeBearer), Properties: map[string]string{"token": "secret"},
		},
	})

	require.NotNil(t, svcErr)
	require.Zero(t, store.updateCalls)
}

func TestServiceUpdateKeepsOmittedAuthenticationOnlyForSameEndpoint(t *testing.T) {
	connection := AuthZENPDPConnection{
		ID: "pdp-1", Name: "AuthZEN PDP", Endpoint: "https://pdp.example.com/evaluation",
	}
	connection.AuthenticationProperties = testAuthProperties(t, outboundauth.Authentication{
		Type: string(outboundauth.TypeBearer), Properties: map[string]string{"token": "secret"},
	})
	store := &serviceStoreStub{connection: connection}
	service := newTestAuthZENPDPService(t, store, config.AuthZENPDPConfig{})

	_, svcErr := service.UpdateAuthZENPDPConnection(context.Background(), "pdp-1", ConnectionRequest{
		Name: "AuthZEN PDP", Endpoint: connection.Endpoint,
	})
	require.Nil(t, svcErr)
	config, err := outboundauth.FromProperties(store.connection.AuthenticationProperties)
	require.NoError(t, err)
	require.Equal(t, "secret", config.Get("token"))

	_, svcErr = service.UpdateAuthZENPDPConnection(context.Background(), "pdp-1", ConnectionRequest{
		Name: "AuthZEN PDP", Endpoint: "https://other.example.com/evaluation",
	})
	require.NotNil(t, svcErr)
	require.Equal(t, "CON-1014", svcErr.Code)
	require.Equal(t, 1, store.updateCalls)
}

func TestServiceUpdateRequiresFreshAuthenticationForChangedBatchEndpoint(t *testing.T) {
	connection := AuthZENPDPConnection{
		ID: "pdp-1", Name: "AuthZEN PDP", Endpoint: "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
	}
	connection.AuthenticationProperties = testAuthProperties(t, outboundauth.Authentication{
		Type: string(outboundauth.TypeBearer), Properties: map[string]string{"token": "old-token"},
	})
	store := &serviceStoreStub{connection: connection}
	service := newTestAuthZENPDPService(t, store, config.AuthZENPDPConfig{})
	request := ConnectionRequest{
		Name: "AuthZEN PDP", Endpoint: connection.Endpoint,
		BatchEndpoint: "https://other.example.com/evaluations",
	}

	_, svcErr := service.UpdateAuthZENPDPConnection(context.Background(), connection.ID, request)
	require.NotNil(t, svcErr)
	require.Equal(t, "CON-1014", svcErr.Code)
	require.Zero(t, store.updateCalls)

	request.Authentication = &outboundauth.Authentication{
		Type: string(outboundauth.TypeBearer), Properties: map[string]string{"token": "******"},
	}
	_, svcErr = service.UpdateAuthZENPDPConnection(context.Background(), connection.ID, request)
	require.NotNil(t, svcErr)
	require.Equal(t, "CON-1014", svcErr.Code)
	require.Zero(t, store.updateCalls)

	request.Authentication.Properties["token"] = "new-token"
	_, svcErr = service.UpdateAuthZENPDPConnection(context.Background(), connection.ID, request)
	require.Nil(t, svcErr)
	require.Equal(t, 1, store.updateCalls)
	stored, err := outboundauth.FromProperties(store.connection.AuthenticationProperties)
	require.NoError(t, err)
	require.Equal(t, "new-token", stored.Get("token"))
}

func TestServiceRejectsEmptyConnectionName(t *testing.T) {
	for _, name := range []string{"", " \t "} {
		t.Run(fmt.Sprintf("name %q", name), func(t *testing.T) {
			store := &serviceStoreStub{}
			service := newTestAuthZENPDPService(t, store, config.AuthZENPDPConfig{})
			request := ConnectionRequest{
				OUID:     "ou-1",
				Name:     name,
				Endpoint: "https://pdp.example.com/evaluation",
			}

			created, createErr := service.CreateAuthZENPDPConnection(context.Background(), request)
			updated, updateErr := service.UpdateAuthZENPDPConnection(context.Background(), "pdp-1", request)

			require.Nil(t, created)
			require.Equal(t, ErrorInvalidName.Code, createErr.Code)
			require.Nil(t, updated)
			require.Equal(t, ErrorInvalidName.Code, updateErr.Code)
			require.Zero(t, store.createCalls)
			require.Zero(t, store.updateCalls)
		})
	}
}

func TestServiceAcceptsConnectionWithoutBatchEndpoint(t *testing.T) {
	store := &serviceStoreStub{}
	service := newTestAuthZENPDPService(t, store, config.AuthZENPDPConfig{})

	created, svcErr := service.CreateAuthZENPDPConnection(context.Background(), ConnectionRequest{
		OUID:     "ou-1",
		Name:     "AuthZEN PDP",
		Endpoint: "https://pdp.example.com/evaluation",
	})

	require.Nil(t, svcErr)
	require.NotNil(t, created)
	require.Empty(t, created.BatchEndpoint)
}

func TestCreateConnectionValidatesSubjectAttributeMappings(t *testing.T) {
	entityTypes := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	entityTypes.EXPECT().GetAttributesForEntityType(
		mock.Anything,
		"employee",
		entitytype.AttributeFilter{AllowNonCredential: true},
	).Return(map[entitytype.TypeCategory][]entitytype.AttributeInfo{
		entitytype.TypeCategoryUser: {{Attribute: "email"}},
	}, nil)
	store := &serviceStoreStub{}
	service := newAuthZENPDPService(
		store, config.AuthZENPDPConfig{}, transaction.NewNoOpTransactioner(), entityTypes, newTestOUService(t))

	created, svcErr := service.CreateAuthZENPDPConnection(context.Background(), ConnectionRequest{
		OUID:          "ou-1",
		Name:          "AuthZEN PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
		SubjectAttributeMappings: []SubjectAttributeMapping{{
			EntityType: "employee",
			Attributes: []SubjectAttributeRow{
				{Attribute: "email", PDPAttribute: "mail"},
				{Attribute: reservedSubjectGroupsAttribute},
			},
		}},
	})

	require.Nil(t, svcErr)
	require.NotNil(t, created)
	require.Equal(t, 1, store.createCalls)
}

func TestCreateConnectionRejectsUnknownSubjectMappingAttribute(t *testing.T) {
	entityTypes := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	entityTypes.EXPECT().GetAttributesForEntityType(
		mock.Anything,
		"employee",
		entitytype.AttributeFilter{AllowNonCredential: true},
	).Return(map[entitytype.TypeCategory][]entitytype.AttributeInfo{
		entitytype.TypeCategoryUser: {{Attribute: "email"}},
	}, nil)
	store := &serviceStoreStub{}
	service := newAuthZENPDPService(
		store, config.AuthZENPDPConfig{}, transaction.NewNoOpTransactioner(), entityTypes, newTestOUService(t))

	created, svcErr := service.CreateAuthZENPDPConnection(context.Background(), ConnectionRequest{
		OUID:          "ou-1",
		Name:          "AuthZEN PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
		SubjectAttributeMappings: []SubjectAttributeMapping{{
			EntityType: "employee",
			Attributes: []SubjectAttributeRow{{Attribute: "unknown"}},
		}},
	})

	require.Nil(t, created)
	require.Equal(t, ErrorInvalidSubjectAttributeMapping.Code, svcErr.Code)
	require.Zero(t, store.createCalls)
}

func TestCreateConnectionRejectsUnknownSubjectMappingType(t *testing.T) {
	entityTypes := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	entityTypes.EXPECT().GetAttributesForEntityType(
		mock.Anything,
		"unknown",
		entitytype.AttributeFilter{AllowNonCredential: true},
	).Return(nil, &entitytype.ErrorEntityTypeNotFound)
	store := &serviceStoreStub{}
	service := newAuthZENPDPService(
		store, config.AuthZENPDPConfig{}, transaction.NewNoOpTransactioner(), entityTypes, newTestOUService(t))

	created, svcErr := service.CreateAuthZENPDPConnection(context.Background(), ConnectionRequest{
		OUID:          "ou-1",
		Name:          "AuthZEN PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
		SubjectAttributeMappings: []SubjectAttributeMapping{{
			EntityType: "unknown",
			Attributes: []SubjectAttributeRow{{Attribute: "email"}},
		}},
	})

	require.Nil(t, created)
	require.Equal(t, ErrorInvalidSubjectAttributeMapping.Code, svcErr.Code)
	require.Zero(t, store.createCalls)
}

func TestCreateConnectionRejectsAmbiguousSubjectMappingType(t *testing.T) {
	entityTypes := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	entityTypes.EXPECT().GetAttributesForEntityType(
		mock.Anything,
		"employee",
		entitytype.AttributeFilter{AllowNonCredential: true},
	).Return(map[entitytype.TypeCategory][]entitytype.AttributeInfo{
		entitytype.TypeCategoryUser:  {{Attribute: "email"}},
		entitytype.TypeCategoryAgent: {{Attribute: "email"}},
	}, nil)
	store := &serviceStoreStub{}
	service := newAuthZENPDPService(
		store, config.AuthZENPDPConfig{}, transaction.NewNoOpTransactioner(), entityTypes, newTestOUService(t))

	created, svcErr := service.CreateAuthZENPDPConnection(context.Background(), ConnectionRequest{
		OUID:     "ou-1",
		Name:     "AuthZEN PDP",
		Endpoint: "https://pdp.example.com/evaluation",
		SubjectAttributeMappings: []SubjectAttributeMapping{{
			EntityType: "employee",
			Attributes: []SubjectAttributeRow{{Attribute: "email"}},
		}},
	})

	require.Nil(t, created)
	require.Equal(t, ErrorInvalidSubjectAttributeMapping.Code, svcErr.Code)
	require.Zero(t, store.createCalls)
}

func TestUpdateConnectionRejectsMissingConnection(t *testing.T) {
	store := &serviceStoreStub{missing: true}
	service := newTestAuthZENPDPService(t, store, config.AuthZENPDPConfig{})

	updated, svcErr := service.UpdateAuthZENPDPConnection(context.Background(), "pdp-1", ConnectionRequest{
		Name:          "AuthZEN PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
	})

	require.Nil(t, updated)
	require.Equal(t, ErrorNotFound.Code, svcErr.Code)
	require.Zero(t, store.updateCalls)
}

func TestUpdateConnectionRejectsImmutableConnection(t *testing.T) {
	store := &serviceStoreStub{
		connection: AuthZENPDPConnection{
			ID: "pdp-1", Name: "AuthZEN PDP",
			Endpoint: "https://pdp.example.com/evaluation", BatchEndpoint: "https://pdp.example.com/evaluations",
		},
		updateErr: ErrAuthZENPDPIsImmutable,
	}
	service := newTestAuthZENPDPService(t, store, config.AuthZENPDPConfig{})

	updated, svcErr := service.UpdateAuthZENPDPConnection(context.Background(), "pdp-1", ConnectionRequest{
		Name:          store.connection.Name,
		Endpoint:      store.connection.Endpoint,
		BatchEndpoint: store.connection.BatchEndpoint,
	})

	require.Nil(t, updated)
	require.Equal(t, declarativeresource.ErrorDeclarativeResourceUpdateOperation.Code, svcErr.Code)
}

func TestCompositeStorePrefersDatabaseAndFallsBackToFile(t *testing.T) {
	ctx := context.Background()
	connection := AuthZENPDPConnection{ID: "db-1", Name: "Database"}
	db := &serviceStoreStub{connection: connection, connections: []AuthZENPDPConnection{connection}}
	file := &fileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeAuthZENPDP),
	}
	require.NoError(t, file.CreateAuthZENPDP(ctx, AuthZENPDPConnection{ID: "file-1", Name: "File"}))
	store := newCompositeStore(file, db)

	found, err := store.GetAuthZENPDP(ctx, "db-1")
	require.NoError(t, err)
	require.Equal(t, "db-1", found.ID)
	require.False(t, found.IsReadOnly)

	db.missing = true
	found, err = store.GetAuthZENPDP(ctx, "file-1")
	require.NoError(t, err)
	require.Equal(t, "file-1", found.ID)
	require.True(t, found.IsReadOnly)

	require.NoError(t, store.CreateAuthZENPDP(ctx, connection))
	require.NoError(t, store.UpdateAuthZENPDP(ctx, "db-1", connection))
	require.NoError(t, store.DeleteAuthZENPDP(ctx, "db-1"))
	require.Equal(t, 1, db.updateCalls)
	require.Equal(t, 1, db.deleteCalls)

	connections, err := store.ListAuthZENPDPs(ctx)
	require.NoError(t, err)
	require.Len(t, connections, 2)
	count, err := store.CountAuthZENPDPs(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	for _, listed := range connections {
		require.Equal(t, listed.ID == "file-1", listed.IsReadOnly)
	}

	require.ErrorIs(t, store.UpdateAuthZENPDP(ctx, "file-1", connection), ErrAuthZENPDPIsImmutable)
	require.ErrorIs(t, store.DeleteAuthZENPDP(ctx, "file-1"), ErrAuthZENPDPIsImmutable)
	require.Equal(t, 1, db.updateCalls)
	require.Equal(t, 1, db.deleteCalls)
}

func TestCompositeStoreListRejectsExcessRecords(t *testing.T) {
	connections := make([]AuthZENPDPConnection, 1001)
	for i := range connections {
		connections[i].ID = fmt.Sprintf("pdp-%d", i)
	}
	db := &serviceStoreStub{connections: connections}
	file := &serviceStoreStub{}
	store := newCompositeStore(file, db)

	listed, err := store.ListAuthZENPDPs(context.Background())
	require.Nil(t, listed)
	require.ErrorIs(t, err, ErrAuthZENPDPResultLimitExceeded)
}

func TestFileBasedStoreRejectsMutableOperations(t *testing.T) {
	store := newFileBasedStore()
	connection := AuthZENPDPConnection{ID: "pdp-1", Name: "PDP"}

	require.NoError(t, store.CreateAuthZENPDP(context.Background(), connection))
	listed, err := store.ListAuthZENPDPs(context.Background())
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.True(t, listed[0].IsReadOnly)
	missing, err := store.GetAuthZENPDP(context.Background(), "missing")
	require.NoError(t, err)
	require.Nil(t, missing)
	missing, err = store.GetAuthZENPDPByName(context.Background(), "missing")
	require.NoError(t, err)
	require.Nil(t, missing)
	require.ErrorIs(t,
		store.UpdateAuthZENPDP(context.Background(), connection.ID, connection), ErrAuthZENPDPIsImmutable)
	require.ErrorIs(t, store.DeleteAuthZENPDP(context.Background(), connection.ID), ErrAuthZENPDPIsImmutable)
}

func TestServiceCreateResolvesConnectionOU(t *testing.T) {
	ouService := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	ouService.On("ResolveOwner", mock.Anything, "", "acme", mock.Anything).Return("ou-acme", nil).Once()
	ouService.On("ResolveOwner", mock.Anything, "", "", mock.Anything).
		Return("", &ErrorOURequired).Once()
	ouService.On("ResolveOwner", mock.Anything, "missing", "", mock.Anything).
		Return("", &ErrorInvalidOU).Once()
	store := &serviceStoreStub{}
	service := newAuthZENPDPService(store, config.AuthZENPDPConfig{}, transaction.NewNoOpTransactioner(), nil,
		ouService)
	base := ConnectionRequest{Name: "AuthZEN PDP", Endpoint: "https://pdp.example.com/evaluation"}

	byHandle := base
	byHandle.OUHandle = "acme"
	created, svcErr := service.CreateAuthZENPDPConnection(context.Background(), byHandle)
	require.Nil(t, svcErr)
	require.Equal(t, "ou-acme", created.OUID)

	_, svcErr = service.CreateAuthZENPDPConnection(context.Background(), base)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorOURequired.Code, svcErr.Code)

	unknown := base
	unknown.OUID = "missing"
	_, svcErr = service.CreateAuthZENPDPConnection(context.Background(), unknown)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInvalidOU.Code, svcErr.Code)
}

func TestServiceUpdateKeepsConnectionOU(t *testing.T) {
	connection := AuthZENPDPConnection{
		ID: "pdp-1", Name: "AuthZEN PDP", OUID: "ou-1", Endpoint: "https://pdp.example.com/evaluation",
		TimeoutMS: -1, RetryCount: -1,
	}
	for _, test := range []struct {
		name      string
		requested string
		wantCode  string
	}{
		{name: "omitted", requested: ""},
		{name: "same", requested: "ou-1"},
		{name: "changed", requested: "ou-2", wantCode: ErrorOUUpdateNotAllowed.Code},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &serviceStoreStub{connection: connection}
			ouService := newTestOUService(t)
			if test.wantCode != "" {
				ouService.On("ResolveOwnerForUpdate", mock.Anything, "ou-2", "", "ou-1", mock.Anything).
					Return("", &ErrorOUUpdateNotAllowed).Once()
			}
			service := newAuthZENPDPService(
				store, config.AuthZENPDPConfig{}, transaction.NewNoOpTransactioner(), nil, ouService)

			updated, svcErr := service.UpdateAuthZENPDPConnection(context.Background(), "pdp-1", ConnectionRequest{
				OUID: test.requested, Name: "AuthZEN PDP", Endpoint: connection.Endpoint,
			})

			if test.wantCode != "" {
				require.NotNil(t, svcErr)
				require.Equal(t, test.wantCode, svcErr.Code)
				require.Zero(t, store.updateCalls)
				return
			}
			require.Nil(t, svcErr)
			require.Equal(t, "ou-1", updated.OUID)
		})
	}
}
