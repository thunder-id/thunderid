// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/managedresourcemock"
)

var errMarkerStore = errors.New("marker store unavailable")

// A managed import takes over by handle only a resource an earlier managed import deployed: one an admin
// made here would be marked under this side's id and could never be removed by the control plane.
func TestAManagedImportDoesNotAdoptAnUnmarkedResourceByHandle(t *testing.T) {
	store := managedresourcemock.NewStoreInterfaceMock(t)
	svc := &importService{managed: store}
	managedCtx := managedresource.WithManagedImport(context.Background())

	store.EXPECT().IsManaged(mock.Anything, "flow-1", managedresource.TypeFlow).Return(false, nil).Once()
	refusal := svc.refuseUnmarkedAdoption(managedCtx, "flow-1")
	require.NotNil(t, refusal)
	assert.Equal(t, ErrorUnmanagedHandleMatch.Code, refusal.Code)

	store.EXPECT().IsManaged(mock.Anything, "flow-2", managedresource.TypeFlow).Return(true, nil).Once()
	assert.Nil(t, svc.refuseUnmarkedAdoption(managedCtx, "flow-2"))

	store.EXPECT().IsManaged(mock.Anything, "flow-3", managedresource.TypeFlow).Return(false, errMarkerStore).Once()
	refusal = svc.refuseUnmarkedAdoption(managedCtx, "flow-3")
	require.NotNil(t, refusal)
	assert.Equal(t, tidcommon.InternalServerError.Code, refusal.Code)

	assert.Nil(t, svc.refuseUnmarkedAdoption(context.Background(), "flow-1"))
}

const managedGroupDocument = `resource_type: group
id: group-new
name: Engineers
ouId: ou-1
members:
  - id: user-1
    type: user
`

// isManagedImport matches a context only the managed import holds.
var isManagedImport = mock.MatchedBy(managedresource.IsManagedImport)

func managedImportService(t *testing.T, groupSvc groupAdapter, roleSvc roleAdapter,
	assignments roleAssignmentAdapter) (*importService, *managedresourcemock.StoreInterfaceMock) {
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, roleSvc, assignments, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, discardingBus{}).(*importService)
	store := managedresourcemock.NewStoreInterfaceMock(t)
	svc.managed = store
	return svc, store
}

// A managed import records the group it writes, and the members it deployed: one it deployed before and
// does not send any more is removed, and every one it sends is recorded.
func TestAManagedImportMarksTheGroupAndReconcilesItsMembers(t *testing.T) {
	groupSvc := &fakeGroupService{}
	svc, store := managedImportService(t, groupSvc, nil, nil)
	store.EXPECT().PartKeys(isManagedImport, managedresource.TypeGroup, "group-new", managedresource.PartMember).
		Return([]string{"user/user-gone"}, nil).Once()
	store.EXPECT().UnmarkParts(isManagedImport, managedresource.TypeGroup, "group-new",
		managedresource.PartMember, []string{"user/user-gone"}).Return(nil).Once()
	store.EXPECT().MarkParts(isManagedImport, managedresource.TypeGroup, "group-new",
		managedresource.PartMember, []string{"user/user-1"}).Return(nil).Once()
	store.EXPECT().Mark(isManagedImport, managedresource.TypeGroup, "group-new").Return(nil).Once()

	resp, svcErr := svc.ImportResources(context.Background(),
		&ImportRequest{Content: managedGroupDocument, Managed: true})

	require.Nil(t, svcErr)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, []group.Member{{ID: "user-gone", Type: group.MemberTypeUser}}, groupSvc.removed)
}

// A member recorded as deployed by an earlier deploy is not recorded again by the next, so a redeploy of a
// large group writes only what changed.
func TestAMemberAlreadyRecordedIsNotRecordedAgain(t *testing.T) {
	groupSvc := &fakeGroupService{}
	svc, store := managedImportService(t, groupSvc, nil, nil)
	store.EXPECT().PartKeys(isManagedImport, managedresource.TypeGroup, "group-new", managedresource.PartMember).
		Return([]string{"user/user-1"}, nil).Once()
	store.EXPECT().UnmarkParts(isManagedImport, managedresource.TypeGroup, "group-new",
		managedresource.PartMember, []string(nil)).Return(nil).Once()
	store.EXPECT().MarkParts(isManagedImport, managedresource.TypeGroup, "group-new",
		managedresource.PartMember, []string(nil)).Return(nil).Once()
	store.EXPECT().Mark(isManagedImport, managedresource.TypeGroup, "group-new").Return(nil).Once()

	resp, svcErr := svc.ImportResources(context.Background(),
		&ImportRequest{Content: managedGroupDocument, Managed: true})

	require.Nil(t, svcErr)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Empty(t, groupSvc.removed)
}

// An import that is not managed, or that is a dry run, marks nothing.
func TestAnImportThatIsNotManagedOrIsADryRunMarksNothing(t *testing.T) {
	for name, request := range map[string]*ImportRequest{
		"not managed": {Content: managedGroupDocument},
		"dry run":     {Content: managedGroupDocument, Managed: true, DryRun: true},
	} {
		t.Run(name, func(t *testing.T) {
			groupSvc := &fakeGroupService{}
			svc, _ := managedImportService(t, groupSvc, nil, nil)

			resp, svcErr := svc.ImportResources(context.Background(), request)

			require.Nil(t, svcErr)
			assert.Equal(t, statusSuccess, resp.Results[0].Status)
			assert.Empty(t, groupSvc.removed)
		})
	}
}

// A resource written but not marked would stay editable, so it is reported as failed for the control plane
// to deploy again.
func TestAResourceThatCannotBeMarkedIsReportedAsFailed(t *testing.T) {
	svc, store := managedImportService(t, &fakeGroupService{}, nil, nil)
	store.EXPECT().PartKeys(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	store.EXPECT().UnmarkParts(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	store.EXPECT().MarkParts(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	store.EXPECT().Mark(mock.Anything, managedresource.TypeGroup, "group-new").Return(errMarkerStore).Once()

	resp, svcErr := svc.ImportResources(context.Background(),
		&ImportRequest{Content: managedGroupDocument, Managed: true})

	require.Nil(t, svcErr)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, tidcommon.InternalServerError.Code, resp.Results[0].Code)
	assert.Equal(t, 1, resp.Summary.Failed)
}

// failingRemoval refuses one member's removal with the given error.
type failingRemoval struct {
	fakeGroupService
	memberID string
	err      *tidcommon.ServiceError
}

func (f *failingRemoval) RemoveGroupMembers(ctx context.Context, groupID string, members []group.Member) (
	*group.Group, *tidcommon.ServiceError) {
	if len(members) == 1 && members[0].ID == f.memberID {
		return nil, f.err
	}
	return f.fakeGroupService.RemoveGroupMembers(ctx, groupID, members)
}

// A member deployed before that no longer exists, as a user the same deploy removed does not, is taken as
// gone; any other refusal fails the group.
func TestAStaleMemberThatIsGoneIsTakenAsRemoved(t *testing.T) {
	gone := &failingRemoval{memberID: "user-gone", err: &group.ErrorInvalidMemberID}
	svc, store := managedImportService(t, gone, nil, nil)
	store.EXPECT().PartKeys(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]string{"user/user-gone", "not-a-key"}, nil)
	store.EXPECT().UnmarkParts(mock.Anything, managedresource.TypeGroup, "group-new",
		managedresource.PartMember, []string{"user/user-gone"}).Return(nil)
	store.EXPECT().MarkParts(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	store.EXPECT().Mark(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	resp, svcErr := svc.ImportResources(context.Background(),
		&ImportRequest{Content: managedGroupDocument, Managed: true})
	require.Nil(t, svcErr)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)

	refused := &failingRemoval{memberID: "user-gone", err: &tidcommon.ErrorUnauthorized}
	svc, store = managedImportService(t, refused, nil, nil)
	store.EXPECT().PartKeys(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]string{"user/user-gone"}, nil)

	resp, svcErr = svc.ImportResources(context.Background(),
		&ImportRequest{Content: managedGroupDocument, Managed: true})
	require.Nil(t, svcErr)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
}

// A failure of the marker store while reconciling members fails the group.
func TestAMarkerFailureWhileReconcilingMembersFailsTheGroup(t *testing.T) {
	cases := map[string]func(store *managedresourcemock.StoreInterfaceMock){
		"reading the deployed members": func(store *managedresourcemock.StoreInterfaceMock) {
			store.EXPECT().PartKeys(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil, errMarkerStore)
		},
		"forgetting the removed members": func(store *managedresourcemock.StoreInterfaceMock) {
			store.EXPECT().PartKeys(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
			store.EXPECT().UnmarkParts(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(errMarkerStore)
		},
		"recording the sent members": func(store *managedresourcemock.StoreInterfaceMock) {
			store.EXPECT().PartKeys(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
			store.EXPECT().UnmarkParts(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil)
			store.EXPECT().MarkParts(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(errMarkerStore)
		},
	}
	for name, expect := range cases {
		t.Run(name, func(t *testing.T) {
			svc, store := managedImportService(t, &fakeGroupService{}, nil, nil)
			expect(store)

			resp, svcErr := svc.ImportResources(context.Background(),
				&ImportRequest{Content: managedGroupDocument, Managed: true})

			require.Nil(t, svcErr)
			assert.Equal(t, statusFailed, resp.Results[0].Status)
		})
	}
}

// A managed import places each assignment in the organization unit it is made in, records it under that
// unit, and removes an assignment it deployed before and does not send any more.
func TestAManagedImportReconcilesARolesAssignments(t *testing.T) {
	assignments := &fakeRoleAssignmentService{}
	svc, store := managedImportService(t, nil, &fakeRoleService{}, assignments)
	store.EXPECT().PartKeys(isManagedImport, managedresource.TypeRole, "role-1", managedresource.PartAssignment).
		Return([]string{"ou-2/group/group-gone"}, nil).Once()
	store.EXPECT().UnmarkParts(isManagedImport, managedresource.TypeRole, "role-1",
		managedresource.PartAssignment, []string{"ou-2/group/group-gone"}).Return(nil).Once()
	store.EXPECT().MarkParts(isManagedImport, managedresource.TypeRole, "role-1",
		managedresource.PartAssignment, []string{"owner-ou/user/user-1"}).Return(nil).Once()
	store.EXPECT().Mark(isManagedImport, managedresource.TypeRole, "role-1").Return(nil).Once()

	content := strings.Join([]string{
		"resource_type: role",
		"id: role-1",
		"name: Viewer",
		"ouId: ou-1",
		"permissions: []",
		"assignments:",
		"  - id: user-1",
		"    type: user",
		"",
	}, "\n")
	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{Content: content, Managed: true})

	require.Nil(t, svcErr)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status, resp.Results[0].Message)
	assert.Equal(t, []role.RoleAssignment{{ID: "user-1", Type: role.AssigneeTypeUser, OUID: "owner-ou"}},
		assignments.assignments)
	assert.Equal(t, []roleAssignmentRemoval{{
		roleID: "role-1", ouID: "ou-2",
		assignments: []role.RoleAssignment{{ID: "group-gone", Type: role.AssigneeTypeGroup}},
	}}, assignments.removals)
}

// An agent type is marked as an agent type, though its outcome names it a user type like any entity type, so
// that what the agent type service checks is what was marked.
func TestAnAgentTypeIsMarkedAsAnAgentType(t *testing.T) {
	for _, docType := range []string{resourceTypeAgentType, resourceTypeEntityType} {
		svc, store := managedImportService(t, nil, nil, nil)
		store.EXPECT().Mark(mock.Anything, docType, "type-1").Return(nil).Once()

		outcome := svc.markManaged(context.Background(), &ImportRequest{Managed: true}, docType,
			ImportItemOutcome{Status: statusSuccess, ResourceType: resourceTypeEntityType, ResourceID: "type-1"})

		assert.Equal(t, statusSuccess, outcome.Status)
	}
}

// A managed import that removes members of a group, or assignees of a role, forgets those entries only, so a
// member added back here is not taken for one the control plane deployed. One that cannot is reported as failed.
func TestAManagedMemberRemovalForgetsTheEntries(t *testing.T) {
	cases := map[string]struct {
		deletion     ResourceDeletion
		resourceType string
		part         string
		keys         []string
	}{
		"group members": {
			deletion: ResourceDeletion{ResourceType: removalTypeGroupMember, ID: "group-1",
				Members: []RemovedMember{{ID: "user-1", Type: "user"}, {ID: "group-2", Type: "group"}}},
			resourceType: managedresource.TypeGroup, part: managedresource.PartMember,
			keys: []string{"user/user-1", "group/group-2"},
		},
		"role assignees": {
			deletion: ResourceDeletion{ResourceType: removalTypeRoleAssignment, ID: "role-1",
				Members: []RemovedMember{{ID: "user-1", Type: "user", OUID: "ou-1"}}},
			resourceType: managedresource.TypeRole, part: managedresource.PartAssignment,
			keys: []string{"ou-1/user/user-1"},
		},
	}
	for name, tc := range cases {
		for _, unmarkErr := range []error{nil, errMarkerStore} {
			t.Run(name, func(t *testing.T) {
				svc, store := managedImportService(t, nil, nil, nil)
				store.EXPECT().UnmarkParts(isManagedImport, tc.resourceType, tc.deletion.ID, tc.part, tc.keys).
					Return(unmarkErr).Once()

				outcome := svc.unmarkManaged(managedresource.WithManagedImport(context.Background()), tc.deletion,
					ImportItemOutcome{Status: statusSuccess})

				if unmarkErr == nil {
					assert.Equal(t, statusSuccess, outcome.Status)
				} else {
					assert.Equal(t, statusFailed, outcome.Status)
				}
			})
		}
	}
}

// A managed import that removes a resource forgets it; one that cannot is reported as failed.
func TestAManagedDeletionForgetsTheResource(t *testing.T) {
	for name, unmarkErr := range map[string]error{"forgotten": nil, "cannot be forgotten": errMarkerStore} {
		t.Run(name, func(t *testing.T) {
			svc := newDeleteTestService(newAppService("app-1"), nil, nil).(*importService)
			store := managedresourcemock.NewStoreInterfaceMock(t)
			svc.managed = store
			store.EXPECT().Unmark(isManagedImport, resourceTypeApplication, "app-1").Return(unmarkErr).Once()

			resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{
				Deletions: []ResourceDeletion{{ResourceType: resourceTypeApplication, ID: "app-1"}},
				Options:   deleteOptions(),
				Managed:   true,
			})

			require.Nil(t, svcErr)
			if unmarkErr == nil {
				assert.Equal(t, statusSuccess, resp.Results[0].Status)
				assert.Equal(t, 1, resp.Summary.Deleted)
			} else {
				assert.Equal(t, statusFailed, resp.Results[0].Status)
				assert.Equal(t, 1, resp.Summary.Failed)
			}
		})
	}
}

// An import that is not managed removes a resource without touching the marks.
func TestAPlainDeletionLeavesTheMarksAlone(t *testing.T) {
	svc := newDeleteTestService(newAppService("app-1"), nil, nil).(*importService)
	svc.managed = managedresourcemock.NewStoreInterfaceMock(t)

	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{
		Deletions: []ResourceDeletion{{ResourceType: resourceTypeApplication, ID: "app-1"}},
		Options:   deleteOptions(),
	})

	require.Nil(t, svcErr)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
}

// Only the management key reaches the managed import, and the handler is what sets it as managed: a body
// cannot.
func TestTheManagedImportTakesOnlyTheManagementKey(t *testing.T) {
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime(t.TempDir(), &config.Config{}))
	t.Cleanup(config.ResetServerRuntime)

	var received *ImportRequest
	handler := newImportHandler(&fakeImportService{
		importFn: func(_ context.Context, r *ImportRequest) (*ImportResponse, *tidcommon.ServiceError) {
			received = r
			return &ImportResponse{}, nil
		},
	})

	refused := httptest.NewRecorder()
	handler.HandleManagedImportRequest(refused,
		httptest.NewRequest(http.MethodPost, "/import/managed", strings.NewReader(`{"content":"x"}`)))
	assert.Equal(t, http.StatusForbidden, refused.Code)
	assert.Contains(t, refused.Body.String(), ErrorManagedImportNotAllowed.Code)
	assert.Nil(t, received)

	request := httptest.NewRequest(http.MethodPost, "/import/managed",
		strings.NewReader(`{"content":"x","managed":false,"Managed":false}`))
	keyContext := security.NewSecurityContextForTest("management-api-key", "", "", nil, nil)
	request = request.WithContext(security.WithSecurityContextTest(request.Context(), keyContext))
	accepted := httptest.NewRecorder()
	handler.HandleManagedImportRequest(accepted, request)
	assert.Equal(t, http.StatusOK, accepted.Code)
	require.NotNil(t, received)
	assert.True(t, received.Managed)

	plain := httptest.NewRecorder()
	handler.HandleImportRequest(plain, httptest.NewRequest(http.MethodPost, "/import",
		strings.NewReader(`{"content":"x","Managed":true}`)))
	assert.Equal(t, http.StatusOK, plain.Code)
	assert.False(t, received.Managed, "a body cannot make an import managed")
}

// A resource with no id, as a translation is, is marked by its name.
func TestAResourceWithNoIDIsMarkedByItsName(t *testing.T) {
	assert.Equal(t, "app-1", managedID(ImportItemOutcome{ResourceID: "app-1", ResourceName: "App"}))
	assert.Equal(t, "fr", managedID(ImportItemOutcome{ResourceName: "fr"}))
}

// failingAssignments refuses what it is told to.
type failingAssignments struct {
	fakeRoleAssignmentService
	resolveErr *tidcommon.ServiceError
	removeErr  *tidcommon.ServiceError
}

func (f *failingAssignments) ResolveAssignmentOUs(ctx context.Context, assignments []role.RoleAssignment) (
	[]role.RoleAssignment, *tidcommon.ServiceError) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	return f.fakeRoleAssignmentService.ResolveAssignmentOUs(ctx, assignments)
}

func (f *failingAssignments) RemoveAssignments(ctx context.Context, id, ouID string,
	assignments []role.RoleAssignment) *tidcommon.ServiceError {
	if f.removeErr != nil {
		return f.removeErr
	}
	return f.fakeRoleAssignmentService.RemoveAssignments(ctx, id, ouID, assignments)
}

const managedRoleDocument = `resource_type: role
id: role-1
name: Viewer
ouId: ou-1
permissions: []
assignments:
  - id: user-1
    type: user
`

// Each failure while reconciling a role's assignments fails the role, except an assignee that is already
// gone, which is taken as unassigned.
func TestAFailureWhileReconcilingAssignmentsFailsTheRole(t *testing.T) {
	cases := map[string]struct {
		assignments *failingAssignments
		expect      func(store *managedresourcemock.StoreInterfaceMock)
		status      string
	}{
		"the units cannot be resolved": {
			assignments: &failingAssignments{resolveErr: &role.ErrorInvalidAssignmentID},
			expect:      func(*managedresourcemock.StoreInterfaceMock) {},
			status:      statusFailed,
		},
		"the deployed assignments cannot be read": {
			assignments: &failingAssignments{},
			expect: func(store *managedresourcemock.StoreInterfaceMock) {
				store.EXPECT().PartKeys(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errMarkerStore)
			},
			status: statusFailed,
		},
		"a stale assignment is refused": {
			assignments: &failingAssignments{removeErr: &tidcommon.ErrorUnauthorized},
			expect: func(store *managedresourcemock.StoreInterfaceMock) {
				store.EXPECT().PartKeys(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return([]string{"ou-2/user/user-gone"}, nil)
			},
			status: statusFailed,
		},
		"a stale assignee is already gone": {
			assignments: &failingAssignments{removeErr: &role.ErrorRoleAssignmentNotFound},
			expect: func(store *managedresourcemock.StoreInterfaceMock) {
				store.EXPECT().PartKeys(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return([]string{"ou-2/user/user-gone", "not-a-key"}, nil)
				store.EXPECT().UnmarkParts(mock.Anything, managedresource.TypeRole, "role-1",
					managedresource.PartAssignment, []string{"ou-2/user/user-gone", "not-a-key"}).Return(nil)
				store.EXPECT().MarkParts(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything).Return(nil)
				store.EXPECT().Mark(mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			status: statusSuccess,
		},
		"the removed assignments cannot be forgotten": {
			assignments: &failingAssignments{},
			expect: func(store *managedresourcemock.StoreInterfaceMock) {
				store.EXPECT().PartKeys(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
				store.EXPECT().UnmarkParts(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything).Return(errMarkerStore)
			},
			status: statusFailed,
		},
		"the sent assignments cannot be recorded": {
			assignments: &failingAssignments{},
			expect: func(store *managedresourcemock.StoreInterfaceMock) {
				store.EXPECT().PartKeys(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
				store.EXPECT().UnmarkParts(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything).Return(nil)
				store.EXPECT().MarkParts(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything).Return(errMarkerStore)
			},
			status: statusFailed,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, store := managedImportService(t, nil, &fakeRoleService{}, tc.assignments)
			tc.expect(store)

			resp, svcErr := svc.ImportResources(context.Background(),
				&ImportRequest{Content: managedRoleDocument, Managed: true})

			require.Nil(t, svcErr)
			assert.Equal(t, tc.status, resp.Results[0].Status, resp.Results[0].Message)
		})
	}
}
