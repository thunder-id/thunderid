// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// The delete capability is optional per adapter, so the shared fakes gain delete methods here rather
// than in the adapter interfaces themselves.

// DeleteIdentityProvider mirrors the real service, which succeeds even when the id is absent.
func (f *fakeIDPService) DeleteIdentityProvider(_ context.Context, idpID string) *tidcommon.ServiceError {
	if v, ok := f.byID[idpID]; ok {
		delete(f.byName, v.Name)
		delete(f.byID, idpID)
	}
	return nil
}

func (f *fakeSenderService) DeleteSender(_ context.Context, id string) *tidcommon.ServiceError {
	if _, ok := f.byID[id]; !ok {
		return &notification.ErrorSenderNotFound
	}
	delete(f.byID, id)
	return nil
}

func newDeleteTestService(
	appSvc *fakeApplicationService, idpSvc *fakeIDPService, senderSvc *fakeSenderService,
) ImportServiceInterface {
	return newImportService(
		appSvc, idpSvc, senderSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

func newAppService(ids ...string) *fakeApplicationService {
	existing := map[string]*providers.Application{}
	for _, id := range ids {
		existing[id] = &providers.Application{ID: id, Name: id}
	}
	return &fakeApplicationService{existing: existing}
}

func deleteOptions() *ImportOptions {
	return &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime}
}

func TestImportResources_DeletesResource(t *testing.T) {
	appSvc := newAppService("app-1", "app-2")
	svc := newDeleteTestService(appSvc, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Deletions: []ResourceDeletion{{ResourceType: resourceTypeApplication, ID: "app-1"}},
		Options:   deleteOptions(),
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.Summary.Deleted)
	assert.Equal(t, 0, resp.Summary.Failed)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, operationDelete, resp.Results[0].Operation)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)

	assert.NotContains(t, appSvc.existing, "app-1")
	assert.Contains(t, appSvc.existing, "app-2", "unrelated resources must be left alone")
}

func TestImportResources_DeleteAbsentResourceIsIdempotent(t *testing.T) {
	svc := newDeleteTestService(newAppService(), nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Deletions: []ResourceDeletion{{ResourceType: resourceTypeApplication, ID: "ghost"}},
		Options:   deleteOptions(),
	})

	require.Nil(t, err)
	assert.Equal(t, 0, resp.Summary.Failed)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, "resource already absent", resp.Results[0].Message)
	assert.Equal(t, 0, resp.Summary.Deleted,
		"a resource that was already absent was not removed by this request")
}

func TestImportResources_UpsertAndDeleteInOneRequest(t *testing.T) {
	appSvc := newAppService("old-app")
	svc := newDeleteTestService(appSvc, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content:   "resource_type: application\nid: new-app\nname: New App\n",
		Deletions: []ResourceDeletion{{ResourceType: resourceTypeApplication, ID: "old-app"}},
		Options:   deleteOptions(),
	})

	require.Nil(t, err)
	assert.Equal(t, 1, resp.Summary.Imported)
	assert.Equal(t, 1, resp.Summary.Deleted)
	assert.Equal(t, 0, resp.Summary.Failed)

	assert.Contains(t, appSvc.existing, "new-app")
	assert.NotContains(t, appSvc.existing, "old-app")

	// The upsert has to land before the removal: a request that replaces a resource must have the
	// replacement in place before its predecessor is pruned. Both ids succeed in either order, so
	// the recorded sequence is the only thing that shows the ordering held.
	assert.Equal(t, []string{"create:new-app", "delete:old-app"}, appSvc.sequence)
}

func TestImportResources_DeleteDryRunDoesNotRemove(t *testing.T) {
	appSvc := newAppService("app-1")
	svc := newDeleteTestService(appSvc, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Deletions: []ResourceDeletion{{ResourceType: resourceTypeApplication, ID: "app-1"}},
		DryRun:    true,
		Options:   deleteOptions(),
	})

	require.Nil(t, err)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Contains(t, appSvc.existing, "app-1", "dry run must not delete")
	assert.Equal(t, 0, resp.Summary.Deleted, "a dry run removed nothing, so it counted nothing")
}

func TestImportResources_DeleteRequiresID(t *testing.T) {
	svc := newDeleteTestService(newAppService(), nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Deletions: []ResourceDeletion{{ResourceType: resourceTypeApplication}},
		Options:   deleteOptions(),
	})

	require.Nil(t, err)
	assert.Equal(t, 0, resp.Summary.Deleted)
	assert.Equal(t, 1, resp.Summary.Failed)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, ErrorInvalidImportRequest.Code, resp.Results[0].Code)
}

func TestImportResources_DeleteUnsupportedResourceType(t *testing.T) {
	svc := newDeleteTestService(newAppService(), nil, nil)

	// Translations and server configuration have no delete operation in their domain services.
	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Deletions: []ResourceDeletion{
			{ResourceType: resourceTypeTranslation, ID: "fr-FR"},
			{ResourceType: resourceTypeServerConfig, ID: "oauth"},
		},
		Options: deleteOptions(),
	})

	require.Nil(t, err)
	assert.Equal(t, 0, resp.Summary.Deleted)
	assert.Equal(t, 2, resp.Summary.Failed)
	for _, result := range resp.Results {
		assert.Equal(t, statusFailed, result.Status)
		assert.Equal(t, ErrorDeleteNotSupported.Code, result.Code)
	}
}

func TestImportResources_DeleteUnconfiguredAdapterIsReported(t *testing.T) {
	// The flow adapter is nil, so a flow deletion cannot be served.
	svc := newDeleteTestService(newAppService(), nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Deletions: []ResourceDeletion{{ResourceType: resourceTypeFlow, ID: "flow-1"}},
		Options:   deleteOptions(),
	})

	require.Nil(t, err)
	assert.Equal(t, 1, resp.Summary.Failed)
	assert.Equal(t, ErrorDeleteNotSupported.Code, resp.Results[0].Code)
}

func TestImportResources_DeleteConnectionRoutesToOwningService(t *testing.T) {
	idpSvc := &fakeIDPService{
		byID:   map[string]*providers.IDPDTO{"idp-1": {ID: "idp-1", Name: "Google"}},
		byName: map[string]*providers.IDPDTO{"Google": {ID: "idp-1", Name: "Google"}},
	}
	senderSvc := &fakeSenderService{
		byID: map[string]*ncommon.NotificationSenderDTO{"sender-1": {ID: "sender-1", Name: "Twilio"}},
	}
	svc := newDeleteTestService(newAppService(), idpSvc, senderSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Deletions: []ResourceDeletion{
			{ResourceType: resourceTypeConnection, ID: "idp-1"},
			{ResourceType: resourceTypeConnection, ID: "sender-1"},
		},
		Options: deleteOptions(),
	})

	require.Nil(t, err)
	assert.Equal(t, 2, resp.Summary.Deleted)
	assert.Equal(t, 0, resp.Summary.Failed)

	assert.NotContains(t, idpSvc.byID, "idp-1")
	// The identity provider delete succeeds for an unknown id, so the sender must be resolved by
	// ownership rather than by chaining deletes, otherwise this deletion would silently no-op.
	assert.NotContains(t, senderSvc.byID, "sender-1")
}

func TestOrderDeletionsByDependencies_ReversesImportOrder(t *testing.T) {
	ordered := orderDeletionsByDependencies([]ResourceDeletion{
		{ResourceType: resourceTypeOrganizationUnit, ID: "ou"},
		{ResourceType: resourceTypeConnection, ID: "conn"},
		{ResourceType: resourceTypeRole, ID: "role"},
		{ResourceType: resourceTypeApplication, ID: "app"},
	})

	got := make([]string, 0, len(ordered))
	for _, deletion := range ordered {
		got = append(got, deletion.ResourceType)
	}

	// Dependents are removed before the resources they reference.
	assert.Equal(t, []string{
		resourceTypeRole,
		resourceTypeApplication,
		resourceTypeConnection,
		resourceTypeOrganizationUnit,
	}, got)
}

func TestImportResources_EmptyRequestStillRejected(t *testing.T) {
	svc := newDeleteTestService(newAppService(), nil, nil)

	_, err := svc.ImportResources(context.Background(), &ImportRequest{Options: deleteOptions()})

	require.NotNil(t, err)
	assert.Equal(t, ErrorInvalidImportRequest.Code, err.Code)
}

// A lookup that fails for any reason other than not-found is the probe itself failing. Carrying on
// past it would let the id fall through every service and be reported as already gone, so a
// deletion that never happened would be counted as a success.
func TestImportResources_DeleteReportsALookupFailure(t *testing.T) {
	idpSvc := &fakeIDPService{
		byID:   map[string]*providers.IDPDTO{"conn-1": {ID: "conn-1", Name: "conn-1"}},
		byName: map[string]*providers.IDPDTO{},
		getErr: &tidcommon.ServiceError{
			Type:  tidcommon.ServerErrorType,
			Code:  "IDP-5000",
			Error: tidcommon.I18nMessage{DefaultValue: "the database is unreachable"},
		},
	}
	svc := newDeleteTestService(newAppService(), idpSvc, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Deletions: []ResourceDeletion{{ResourceType: resourceTypeConnection, ID: "conn-1"}},
		Options:   deleteOptions(),
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status,
		"a failed lookup was reported as a successful deletion")
	assert.Equal(t, "IDP-5000", resp.Results[0].Code)
	assert.Equal(t, 0, resp.Summary.Deleted)
	assert.Equal(t, 1, resp.Summary.Failed)

	assert.Contains(t, idpSvc.byID, "conn-1", "the connection was removed despite the failed lookup")
}

// A not-found from one service is not a failure: it means the next service should be asked. A
// connection is stored as either an identity provider or a notification sender, so the id the first
// does not own has to reach the second.
func TestImportResources_DeleteFallsThroughOnNotFound(t *testing.T) {
	idpSvc := &fakeIDPService{byID: map[string]*providers.IDPDTO{}, byName: map[string]*providers.IDPDTO{}}
	senderSvc := &fakeSenderService{
		byID: map[string]*ncommon.NotificationSenderDTO{"conn-1": {ID: "conn-1"}},
	}
	svc := newDeleteTestService(newAppService(), idpSvc, senderSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Deletions: []ResourceDeletion{{ResourceType: resourceTypeConnection, ID: "conn-1"}},
		Options:   deleteOptions(),
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status, "message: %s", resp.Results[0].Message)
	assert.Equal(t, 1, resp.Summary.Deleted)
	assert.NotContains(t, senderSvc.byID, "conn-1", "the sender the id belonged to was left in place")
}

// A connection neither service owns was not removed by this request, and is reported as such.
//
// The probes have just confirmed there is nothing there, so reporting a removal would count one that
// did not happen and would drop the message that says why the outcome is a success.
func TestImportResources_DeleteAConnectionNeitherServiceOwns(t *testing.T) {
	idpSvc := &fakeIDPService{byID: map[string]*providers.IDPDTO{}, byName: map[string]*providers.IDPDTO{}}
	senderSvc := &fakeSenderService{byID: map[string]*ncommon.NotificationSenderDTO{}}
	svc := newDeleteTestService(newAppService(), idpSvc, senderSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Deletions: []ResourceDeletion{{ResourceType: resourceTypeConnection, ID: "owned-by-nobody"}},
		Options:   deleteOptions(),
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status, "an absent connection is still a success")
	assert.Equal(t, "resource already absent", resp.Results[0].Message)
	assert.Equal(t, 0, resp.Summary.Deleted, "a connection that was never there was counted as removed")
	assert.Equal(t, 0, resp.Summary.Failed)
}
