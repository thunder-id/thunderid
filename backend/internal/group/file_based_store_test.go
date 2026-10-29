// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"testing"

	entitypkg "github.com/thunder-id/thunderid/internal/entity"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	entitystore "github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"

	"github.com/stretchr/testify/suite"
)

// GroupFileBasedStoreTestSuite contains tests for the file-based group store.
type GroupFileBasedStoreTestSuite struct {
	suite.Suite
	store *fileBasedGroupStore
}

func TestGroupFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(GroupFileBasedStoreTestSuite))
}

func (suite *GroupFileBasedStoreTestSuite) SetupTest() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entitystore.KeyTypeGroup)
	suite.store = &fileBasedGroupStore{GenericFileBasedStore: genericStore}
}

func (suite *GroupFileBasedStoreTestSuite) seedGroup(grp groupDeclarativeResource) {
	err := suite.store.GenericFileBasedStore.Create(grp.ID, &grp)
	suite.Require().NoError(err)
}

func (suite *GroupFileBasedStoreTestSuite) TestGetGroupListCountAndList() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})
	suite.seedGroup(groupDeclarativeResource{ID: "grp2", Name: "Engineers", OUID: "ou1"})

	count, err := suite.store.GetGroupListCount(context.Background())

	suite.NoError(err)
	suite.Equal(2, count)

	groups, err := suite.store.GetGroupList(context.Background(), 10, 0)

	suite.NoError(err)
	suite.Len(groups, 2)
	ids := map[string]bool{}
	for _, g := range groups {
		ids[g.ID] = true
		suite.True(g.IsReadOnly)
	}
	suite.True(ids["grp1"])
	suite.True(ids["grp2"])

	paged, err := suite.store.GetGroupList(context.Background(), 1, 1)

	suite.NoError(err)
	suite.Len(paged, 1)
}

func (suite *GroupFileBasedStoreTestSuite) TestGetGroupAndIsDeclarative() {
	suite.seedGroup(groupDeclarativeResource{
		ID:          "grp1",
		Name:        "Admins",
		Description: "Admin group",
		OUID:        "ou1",
		Members: []Member{
			{ID: "user1", Type: memberTypeEntity},
		},
	})

	grp, err := suite.store.GetGroup(context.Background(), "grp1")

	suite.NoError(err)
	suite.Equal("grp1", grp.ID)
	suite.Equal("Admins", grp.Name)
	suite.Equal("Admin group", grp.Description)
	suite.Equal("ou1", grp.OUID)
	suite.True(grp.IsReadOnly)
	suite.Len(grp.Members, 1)

	isDeclarative, err := suite.store.IsGroupDeclarative(context.Background(), "grp1")
	suite.NoError(err)
	suite.True(isDeclarative)

	isDeclarative, err = suite.store.IsGroupDeclarative(context.Background(), "nonexistent")
	suite.NoError(err)
	suite.False(isDeclarative)
}

func (suite *GroupFileBasedStoreTestSuite) TestGetGroup_NotFound() {
	_, err := suite.store.GetGroup(context.Background(), "nonexistent")

	suite.Error(err)
	suite.Equal(ErrGroupNotFound, err)
}

func (suite *GroupFileBasedStoreTestSuite) TestGetGroupMembers() {
	suite.seedGroup(groupDeclarativeResource{
		ID:   "grp1",
		Name: "Admins",
		OUID: "ou1",
		Members: []Member{
			{ID: "user1", Type: memberTypeEntity},
			{ID: "group2", Type: MemberTypeGroup},
		},
	})

	count, err := suite.store.GetGroupMemberCount(context.Background(), "grp1")
	suite.NoError(err)
	suite.Equal(2, count)

	members, err := suite.store.GetGroupMembers(context.Background(), "grp1", 10, 0)
	suite.NoError(err)
	suite.Len(members, 2)

	paged, err := suite.store.GetGroupMembers(context.Background(), "grp1", 1, 1)
	suite.NoError(err)
	suite.Len(paged, 1)
}

func (suite *GroupFileBasedStoreTestSuite) TestGetGroupMembers_NonExistentGroup() {
	members, err := suite.store.GetGroupMembers(context.Background(), "nonexistent", 10, 0)

	suite.NoError(err)
	suite.Empty(members)
}

func (suite *GroupFileBasedStoreTestSuite) TestGetGroupMemberCount_NonExistentGroup() {
	count, err := suite.store.GetGroupMemberCount(context.Background(), "nonexistent")

	suite.NoError(err)
	suite.Equal(0, count)
}

func (suite *GroupFileBasedStoreTestSuite) TestCheckGroupNameConflictForCreate() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})
	suite.seedGroup(groupDeclarativeResource{ID: "grp2", Name: "Admins", OUID: "ou2"})

	err := suite.store.CheckGroupNameConflictForCreate(context.Background(), "Admins", "ou1")
	suite.Error(err)
	suite.Equal(ErrGroupNameConflict, err)

	err = suite.store.CheckGroupNameConflictForCreate(context.Background(), "Admins", "ou3")
	suite.NoError(err)

	err = suite.store.CheckGroupNameConflictForCreate(context.Background(), "Engineers", "ou1")
	suite.NoError(err)
}

func (suite *GroupFileBasedStoreTestSuite) TestCheckGroupNameConflictForUpdate() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})
	suite.seedGroup(groupDeclarativeResource{ID: "grp2", Name: "Admins", OUID: "ou1"})

	err := suite.store.CheckGroupNameConflictForUpdate(context.Background(), "Admins", "ou1", "grp1")
	suite.Error(err)
	suite.Equal(ErrGroupNameConflict, err)

	suite.seedGroup(groupDeclarativeResource{ID: "grp3", Name: "Solo", OUID: "ou3"})
	err = suite.store.CheckGroupNameConflictForUpdate(context.Background(), "Solo", "ou3", "grp3")
	suite.NoError(err)
}

func (suite *GroupFileBasedStoreTestSuite) TestGetGroupListByOUIDs() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})
	suite.seedGroup(groupDeclarativeResource{ID: "grp2", Name: "Engineers", OUID: "ou1"})
	suite.seedGroup(groupDeclarativeResource{ID: "grp3", Name: "Finance", OUID: "ou2"})

	count, err := suite.store.GetGroupListCountByOUIDs(context.Background(), []string{"ou1"})
	suite.NoError(err)
	suite.Equal(2, count)

	groups, err := suite.store.GetGroupListByOUIDs(context.Background(), []string{"ou1"}, 10, 0)
	suite.NoError(err)
	suite.Len(groups, 2)
	for _, g := range groups {
		suite.Equal("ou1", g.OUID)
		suite.True(g.IsReadOnly)
	}

	groups, err = suite.store.GetGroupListByOUIDs(context.Background(), []string{"ou1", "ou2"}, 10, 0)
	suite.NoError(err)
	suite.Len(groups, 3)
}

func (suite *GroupFileBasedStoreTestSuite) TestGetGroupsByOrganizationUnit() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})
	suite.seedGroup(groupDeclarativeResource{ID: "grp2", Name: "Engineers", OUID: "ou1"})
	suite.seedGroup(groupDeclarativeResource{ID: "grp3", Name: "Finance", OUID: "ou2"})

	count, err := suite.store.GetGroupsByOrganizationUnitCount(context.Background(), "ou1")
	suite.NoError(err)
	suite.Equal(2, count)

	groups, err := suite.store.GetGroupsByOrganizationUnit(context.Background(), "ou1", 10, 0)
	suite.NoError(err)
	suite.Len(groups, 2)
	for _, g := range groups {
		suite.Equal("ou1", g.OUID)
		suite.True(g.IsReadOnly)
	}

	paged, err := suite.store.GetGroupsByOrganizationUnit(context.Background(), "ou1", 1, 1)
	suite.NoError(err)
	suite.Len(paged, 1)
}

func (suite *GroupFileBasedStoreTestSuite) TestValidateGroupIDs() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})
	suite.seedGroup(groupDeclarativeResource{ID: "grp2", Name: "Engineers", OUID: "ou1"})

	invalid, err := suite.store.ValidateGroupIDs(context.Background(), []string{"grp1", "grp2"})
	suite.NoError(err)
	suite.Empty(invalid)

	invalid, err = suite.store.ValidateGroupIDs(context.Background(), []string{"grp1", "missing"})
	suite.NoError(err)
	suite.Equal([]string{"missing"}, invalid)

	invalid, err = suite.store.ValidateGroupIDs(context.Background(), []string{})
	suite.NoError(err)
	suite.Empty(invalid)
}

func (suite *GroupFileBasedStoreTestSuite) TestGetGroupsByIDs() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})
	suite.seedGroup(groupDeclarativeResource{ID: "grp2", Name: "Engineers", OUID: "ou1"})

	groups, err := suite.store.GetGroupsByIDs(context.Background(), []string{"grp1", "grp2"})
	suite.NoError(err)
	suite.Len(groups, 2)
	for _, g := range groups {
		suite.True(g.IsReadOnly)
	}

	groups, err = suite.store.GetGroupsByIDs(context.Background(), []string{"grp1", "missing"})
	suite.NoError(err)
	suite.Len(groups, 1)
	suite.Equal("grp1", groups[0].ID)

	groups, err = suite.store.GetGroupsByIDs(context.Background(), []string{})
	suite.NoError(err)
	suite.Empty(groups)
}

func (suite *GroupFileBasedStoreTestSuite) TestImmutability() {
	suite.seedGroup(groupDeclarativeResource{ID: "immutable-grp", Name: "Test Group", OUID: "ou1"})

	err := suite.store.CreateGroup(context.Background(), GroupDAO{ID: "new-grp", Name: "New"})
	suite.Error(err)

	err = suite.store.UpdateGroup(context.Background(), GroupDAO{ID: "immutable-grp", Name: "Updated"})
	suite.Error(err)

	err = suite.store.DeleteGroup(context.Background(), "immutable-grp")
	suite.Error(err)

	err = suite.store.AddGroupMembers(context.Background(), "immutable-grp", []Member{
		{ID: "user1", Type: MemberTypeUser},
	})
	suite.Error(err)

	err = suite.store.RemoveGroupMembers(context.Background(), "immutable-grp", []Member{
		{ID: "user1", Type: MemberTypeUser},
	})
	suite.Error(err)

	member := Member{ID: "user1", Type: memberTypeEntity}
	suite.Error(suite.store.AddMemberToGroups(context.Background(), member, []string{"immutable-grp"}))
	suite.Error(suite.store.RemoveMemberFromGroups(context.Background(), member, []string{"immutable-grp"}))

	groupIDs, err := suite.store.GetMemberGroupIDs(context.Background(), string(memberTypeEntity), "user1")
	suite.NoError(err)
	suite.Empty(groupIDs)
}

func (suite *GroupFileBasedStoreTestSuite) TestCreate_ImplementsStorer() {
	grp := &groupDeclarativeResource{
		ID:          "grp-create",
		Name:        "Test Create",
		Description: "Create implementation test",
		OUID:        "ou1",
	}

	err := suite.store.Create("grp-create", grp)
	suite.NoError(err)

	retrieved, err := suite.store.GetGroup(context.Background(), "grp-create")
	suite.NoError(err)
	suite.Equal("grp-create", retrieved.ID)
	suite.Equal("Test Create", retrieved.Name)
}

func (suite *GroupFileBasedStoreTestSuite) TestCreate_InvalidData() {
	err := suite.store.Create("bad-grp", "invalid string data")
	suite.Error(err)
	suite.Equal(ErrGroupDataCorrupted, err)
}

func (suite *GroupFileBasedStoreTestSuite) TestCreate_SetsIDFromParameter() {
	grp := &groupDeclarativeResource{
		Name: "Group Without ID",
		OUID: "ou1",
	}

	err := suite.store.Create("param-grp-id", grp)
	suite.NoError(err)

	retrieved, err := suite.store.GetGroup(context.Background(), "param-grp-id")
	suite.NoError(err)
	suite.Equal("param-grp-id", retrieved.ID)
}

// Declarative resource loading (parseToGroup) translates public types user/app/agent to the
// internal memberTypeEntity before storing. Tests seed with memberTypeEntity to match that flow.

func (suite *GroupFileBasedStoreTestSuite) TestGetTransitiveGroupsForEntity_SingleGroup() {
	suite.seedGroup(groupDeclarativeResource{
		ID:   "grp1",
		Name: "Administrators",
		OUID: "ou1",
		Members: []Member{
			{ID: "user1", Type: memberTypeEntity},
		},
	})

	groups, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.NoError(err)
	suite.Len(groups, 1)
	suite.Equal("grp1", groups[0].ID)
	suite.Equal("Administrators", groups[0].Name)
	suite.Equal("ou1", groups[0].OUID)
}

func (suite *GroupFileBasedStoreTestSuite) TestGetTransitiveGroupsForEntity_MultipleGroups() {
	suite.seedGroup(groupDeclarativeResource{
		ID:   "grp1",
		Name: "Administrators",
		OUID: "ou1",
		Members: []Member{
			{ID: "user1", Type: memberTypeEntity},
		},
	})
	suite.seedGroup(groupDeclarativeResource{
		ID:   "grp2",
		Name: "Engineers",
		OUID: "ou1",
		Members: []Member{
			{ID: "user1", Type: memberTypeEntity},
			{ID: "user2", Type: memberTypeEntity},
		},
	})

	groups, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.NoError(err)
	suite.Len(groups, 2)
	ids := map[string]bool{}
	for _, g := range groups {
		ids[g.ID] = true
	}
	suite.True(ids["grp1"])
	suite.True(ids["grp2"])
}

func (suite *GroupFileBasedStoreTestSuite) TestGetTransitiveGroupsForEntity_NotMember() {
	suite.seedGroup(groupDeclarativeResource{
		ID:   "grp1",
		Name: "Administrators",
		OUID: "ou1",
		Members: []Member{
			{ID: "user2", Type: memberTypeEntity},
		},
	})

	groups, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.NoError(err)
	suite.Empty(groups)
}

func (suite *GroupFileBasedStoreTestSuite) TestGetTransitiveGroupsForEntity_EmptyStore() {
	groups, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.NoError(err)
	suite.Empty(groups)
}

// Group-type members (nested group references) must not match an entity lookup even when their ID
// matches, because they represent group membership, not direct entity membership.
func (suite *GroupFileBasedStoreTestSuite) TestGetTransitiveGroupsForEntity_IgnoresGroupTypeMembers() {
	suite.seedGroup(groupDeclarativeResource{
		ID:   "grp1",
		Name: "Administrators",
		OUID: "ou1",
		Members: []Member{
			{ID: "user1", Type: MemberTypeGroup}, // same ID but group type — must not match
			{ID: "user2", Type: memberTypeEntity},
		},
	})

	groups, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.NoError(err)
	suite.Empty(groups)
}

func (suite *GroupFileBasedStoreTestSuite) TestGetTransitiveGroupsForEntity_GroupWithNoMembers() {
	suite.seedGroup(groupDeclarativeResource{
		ID:      "grp1",
		Name:    "Empty",
		OUID:    "ou1",
		Members: []Member{},
	})

	groups, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.NoError(err)
	suite.Empty(groups)
}

// GetTransitiveGroupsForEntity must walk group->group edges so that a parent group whose
// only connection to the entity is through a child group is also returned.
func (suite *GroupFileBasedStoreTestSuite) TestGetTransitiveGroupsForEntity_TransitiveGroupMembership() {
	// groupA directly contains user1
	suite.seedGroup(groupDeclarativeResource{
		ID:   "groupA",
		Name: "Child Group",
		OUID: "ou1",
		Members: []Member{
			{ID: "user1", Type: memberTypeEntity},
		},
	})
	// groupB contains groupA as a nested group member
	suite.seedGroup(groupDeclarativeResource{
		ID:   "groupB",
		Name: "Parent Group",
		OUID: "ou1",
		Members: []Member{
			{ID: "groupA", Type: MemberTypeGroup},
		},
	})

	groups, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.NoError(err)
	suite.Len(groups, 2)
	ids := map[string]bool{}
	for _, g := range groups {
		ids[g.ID] = true
	}
	suite.True(ids["groupA"], "direct parent group must be returned")
	suite.True(ids["groupB"], "transitive parent group must be returned")
}

func (suite *GroupFileBasedStoreTestSuite) TestGetTransitiveGroupsForEntity_DeepNesting() {
	suite.seedGroup(groupDeclarativeResource{
		ID: "g1", Name: "Level1", OUID: "ou1",
		Members: []Member{{ID: "user1", Type: memberTypeEntity}},
	})
	suite.seedGroup(groupDeclarativeResource{
		ID: "g2", Name: "Level2", OUID: "ou1",
		Members: []Member{{ID: "g1", Type: MemberTypeGroup}},
	})
	suite.seedGroup(groupDeclarativeResource{
		ID: "g3", Name: "Level3", OUID: "ou1",
		Members: []Member{{ID: "g2", Type: MemberTypeGroup}},
	})

	groups, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.NoError(err)
	suite.Len(groups, 3)
}

func (suite *GroupFileBasedStoreTestSuite) TestGetTransitiveGroupsForEntity_NoCycleOnCircularGroups() {
	suite.seedGroup(groupDeclarativeResource{
		ID: "gA", Name: "A", OUID: "ou1",
		Members: []Member{
			{ID: "user1", Type: memberTypeEntity},
			{ID: "gB", Type: MemberTypeGroup},
		},
	})
	suite.seedGroup(groupDeclarativeResource{
		ID: "gB", Name: "B", OUID: "ou1",
		Members: []Member{{ID: "gA", Type: MemberTypeGroup}},
	})

	groups, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.NoError(err)
	ids := map[string]bool{}
	for _, g := range groups {
		ids[g.ID] = true
	}
	suite.True(ids["gA"])
	suite.True(ids["gB"])
	suite.Len(groups, 2, "each group must appear exactly once despite the cycle")
}

// Ensure the membership provider satisfies entity.GroupMembershipProvider.
var _ entitypkg.GroupMembershipProvider = membershipProvider{}

// A malformed declarative entry must fail the ancestor lookup rather than being skipped. A skipped
// entry could be a parent, and a short ancestor list understates what membership in a group confers.
func (suite *GroupFileBasedStoreTestSuite) TestGetDirectGroupParents_MalformedEntryFailsClosed() {
	parent := groupDeclarativeResource{
		ID:      "parent-grp",
		Name:    "Parent",
		OUID:    "ou1",
		Members: []Member{{ID: "child-grp", Type: MemberTypeGroup}},
	}
	suite.Require().NoError(suite.store.GenericFileBasedStore.Create(parent.ID, &parent))
	// Bypass the typed Create so a malformed entry lands in the store.
	suite.Require().NoError(suite.store.GenericFileBasedStore.Create("broken-grp", "not a group"))

	parents, err := suite.store.GetDirectGroupParents(context.Background(), []string{"child-grp"})

	suite.Error(err, "a malformed declarative group must not be silently skipped")
	suite.Nil(parents)
}

// The happy path must still resolve parents when every entry parses.
func (suite *GroupFileBasedStoreTestSuite) TestGetDirectGroupParents_ResolvesWhenAllEntriesParse() {
	parent := groupDeclarativeResource{
		ID:      "ok-parent",
		Name:    "Parent",
		OUID:    "ou1",
		Members: []Member{{ID: "ok-child", Type: MemberTypeGroup}},
	}
	suite.Require().NoError(suite.store.GenericFileBasedStore.Create(parent.ID, &parent))

	parents, err := suite.store.GetDirectGroupParents(context.Background(), []string{"ok-child"})

	suite.NoError(err)
	suite.Contains(parents, "ok-parent")
}
