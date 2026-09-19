// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cache"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const (
	testType     = ResourceType("role")
	testResource = "resource-1"
	ownerOU      = "owner-ou"
	rootOU       = "root-ou"
	childOU      = "child-ou"
	grandOU      = "grand-ou"
	otherOU      = "other-ou"
	// nestedOwnerOU owns a resource while sitting inside a tree, which is the case the cross-tree
	// restriction exists for.
	nestedOwnerOU = "nested-owner-ou"
	greatOU       = "great-ou"
)

// fakeStore is an in-memory storeInterface, so the service's orchestration is testable without a
// database while the decisions it delegates stay covered by the engine's own tests.
type fakeStore struct {
	policies map[string]Policy
	values   map[string]map[string][]string
	failNext error
	// beforeCreate runs at the start of CreatePolicy, so a test can simulate another writer
	// committing in the window between the pre-flight check and this insert.
	beforeCreate func()
}

func newFakeStore() *fakeStore {
	return &fakeStore{policies: map[string]Policy{}, values: map[string]map[string][]string{}}
}

func (f *fakeStore) CreatePolicy(_ context.Context, p Policy) error {
	if f.beforeCreate != nil {
		f.beforeCreate()
	}
	if f.failNext != nil {
		err := f.failNext
		f.failNext = nil
		return err
	}
	f.policies[p.ID] = p
	return nil
}

func (f *fakeStore) GetPolicy(_ context.Context, id string) (Policy, error) {
	p, ok := f.policies[id]
	if !ok {
		return Policy{}, ErrPolicyNotFound
	}
	return p, nil
}

func (f *fakeStore) GetPolicyByInitiator(
	_ context.Context, rt ResourceType, resourceID, initiatingOUID string,
) (Policy, error) {
	for _, p := range f.policies {
		if p.ResourceType == rt && p.ResourceID == resourceID && p.InitiatingOUID == initiatingOUID {
			return p, nil
		}
	}
	return Policy{}, ErrPolicyNotFound
}

func (f *fakeStore) ListPoliciesForResource(
	_ context.Context, rt ResourceType, resourceID string,
) ([]Policy, error) {
	out := make([]Policy, 0, len(f.policies))
	for _, p := range f.policies {
		if p.ResourceType == rt && p.ResourceID == resourceID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeStore) ListPoliciesRelevantToChain(
	ctx context.Context, rt ResourceType, _ []string,
) ([]Policy, error) {
	return f.ListPoliciesForResource(ctx, rt, testResource)
}

func (f *fakeStore) ReplacePolicyContents(_ context.Context, p Policy, expectedVersion int) error {
	current, ok := f.policies[p.ID]
	if !ok || current.Version != expectedVersion {
		return ErrPolicyNotFound
	}
	p.Version = current.Version + 1
	f.policies[p.ID] = p
	return nil
}

func (f *fakeStore) DeletePolicy(_ context.Context, id string) error {
	delete(f.policies, id)
	return nil
}

func (f *fakeStore) GetOverlayValues(
	_ context.Context, rt ResourceType, resourceID, ouID string,
) (map[string][]string, error) {
	return f.values[string(rt)+resourceID+ouID], nil
}

func (f *fakeStore) SetOverlayValue(
	_ context.Context, rt ResourceType, resourceID, ouID, fieldKey string, value []string,
) error {
	key := string(rt) + resourceID + ouID
	if f.values[key] == nil {
		f.values[key] = map[string][]string{}
	}
	f.values[key][fieldKey] = value
	return nil
}

func (f *fakeStore) DeleteOverlayValue(
	_ context.Context, rt ResourceType, resourceID, ouID, fieldKey string,
) error {
	delete(f.values[string(rt)+resourceID+ouID], fieldKey)
	return nil
}

// fakeResolver answers ancestor queries from a fixed tree.
type fakeResolver struct{ ancestors map[string][]string }

func (f *fakeResolver) GetAncestorOUIDs(_ context.Context, ouID string) ([]string, *tidcommon.ServiceError) {
	return f.ancestors[ouID], nil
}

func (f *fakeResolver) IsAncestor(
	_ context.Context, ancestorOUID, descendantOUID string,
) (bool, *tidcommon.ServiceError) {
	for _, id := range f.ancestors[descendantOUID] {
		if id == ancestorOUID {
			return true, nil
		}
	}
	return false, nil
}

// DescendantOUIDs and AllOUIDs make the fake an OUEnumerator, deriving the downward view by
// inverting the same ancestor table the upward walks use.
func (f *fakeResolver) DescendantOUIDs(_ context.Context, ouID string) ([]string, *tidcommon.ServiceError) {
	out := []string{}
	for id, ancestors := range f.ancestors {
		for _, a := range ancestors {
			if a == ouID {
				out = append(out, id)
				break
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

func (f *fakeResolver) AllOUIDs(_ context.Context) ([]string, *tidcommon.ServiceError) {
	out := make([]string, 0, len(f.ancestors))
	for id := range f.ancestors {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// fakeTransactioner runs the work inline; the fake store has nothing to roll back.
type fakeTransactioner struct{}

func (f *fakeTransactioner) Transact(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// testDeclaration is a resource type with one editable reference-set field and one non-editable
// hierarchy field, which is enough to exercise both containment kinds.
type testDeclaration struct {
	owner     string
	lostCalls []lostVisibilityCall
}

type lostVisibilityCall struct {
	resourceID string
	ouID       string
	fields     []string
}

func (d *testDeclaration) ResourceType() ResourceType { return testType }

// OwningOUID makes the declaration an OwnerResolver, which is how the framework learns who owns a
// resource it stores no row for.
func (d *testDeclaration) OwningOUID(_ context.Context, _ string) (string, *tidcommon.ServiceError) {
	if d.owner == "" {
		return ownerOU, nil
	}
	return d.owner, nil
}

func (d *testDeclaration) Fields() []FieldDeclaration {
	editable := OverlayRule{Editable: true}
	locked := OverlayRule{Editable: false}
	return []FieldDeclaration{
		{Key: "assignments", Kind: FieldReferenceSet, Default: &editable},
		{Key: "assignments.user", FallbackKey: "assignments", Kind: FieldReferenceSet, Default: &editable},
		{Key: "permissions", Kind: FieldHierarchy, Default: &locked},
	}
}

func (d *testDeclaration) OnVisibilityLost(_ context.Context, resourceID, ouID string, fields []string) error {
	d.lostCalls = append(d.lostCalls, lostVisibilityCall{resourceID, ouID, fields})
	return nil
}

type ServiceTestSuite struct {
	suite.Suite
	store *fakeStore
	decl  *testDeclaration
	svc   ServiceInterface
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	s.store = newFakeStore()
	s.decl = &testDeclaration{}
	resolver := testResolver()
	s.svc = newService(s.store, nil, resolver, resolver, &fakeTransactioner{}, nil, nil, false)
	s.svc.RegisterResourceType(s.decl)
}

// testResolver is the fixture tree: two roots, one of which has a child and a grandchild, plus a
// non-root organization unit that owns a resource of its own.
func testResolver() *fakeResolver {
	return &fakeResolver{ancestors: map[string][]string{
		rootOU:        {},
		childOU:       {rootOU},
		grandOU:       {childOU, rootOU},
		otherOU:       {},
		ownerOU:       {},
		nestedOwnerOU: {rootOU},
		greatOU:       {grandOU, childOU, rootOU},
	}}
}

// share issues the owner's policy reaching one root.
func (s *ServiceTestSuite) share(rules map[string]OverlayRule) Policy {
	p, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules:  rules,
	})
	s.Require().Nil(svcErr)
	return p
}

func (s *ServiceTestSuite) TestCreateRejectsAnUnregisteredResourceType() {
	_, svcErr := s.svc.CreatePolicy(context.Background(), "unknown", testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{AllRoots: true},
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorResourceTypeNotRegistered.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestCreateRequiresExactlyOneTargetMode() {
	tests := []struct {
		name  string
		scope TargetOUScope
	}{
		{"no mode", TargetOUScope{}},
		{"two modes", TargetOUScope{AllRoots: true, AllChildren: true}},
		{"three modes", TargetOUScope{AllOUs: true, AllRoots: true, AllChildren: true}},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			_, svcErr := s.svc.CreatePolicy(
				context.Background(), testType, testResource, ownerOU,
				PolicyRequest{TargetOUScope: tt.scope})

			s.Require().NotNil(svcErr)
			s.Equal(ErrorInvalidRequestFormat.Code, svcErr.Code)
		})
	}
}

// The constraint the whole design rests on: one policy per organization unit per resource is what
// makes an edit well-defined and ends duplicate accumulation.
func (s *ServiceTestSuite) TestCreateRefusesASecondPolicyForTheSameOU() {
	first := s.share(nil)

	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{otherOU}},
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorPolicyExists.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, first.ID,
		"the refusal names the policy to edit instead")
}

func (s *ServiceTestSuite) TestCreateRejectsOwnerOnlyScopesFromASharee() {
	s.share(nil)

	for _, scope := range []TargetOUScope{{AllOUs: true}, {AllRoots: true}} {
		_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
			PolicyRequest{InitiatingOUID: rootOU, TargetOUScope: scope})

		s.Require().NotNil(svcErr)
		s.Equal(ErrorInvalidTargetOU.Code, svcErr.Code)
	}
}

func (s *ServiceTestSuite) TestCreateRejectsAReshareFromAnOUWithNoStanding() {
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: otherOU,
		TargetOUScope:  TargetOUScope{AllChildren: true},
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorNotShared.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestCreateRecordsStageAndParent() {
	owner := s.share(nil)

	reshared, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{InitiatingOUID: rootOU, TargetOUScope: TargetOUScope{AllChildren: true}})

	s.Require().Nil(svcErr)
	s.Equal(StageReshare, reshared.Stage)
	s.Equal(owner.ID, reshared.ParentPolicyID, "the covering policy is the parent")
}

func (s *ServiceTestSuite) TestCreateRejectsAnUnknownFieldKey() {
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules:  map[string]OverlayRule{"nonsense": {Editable: true}},
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorUnknownFieldKey.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "nonsense", "the rejection names the field")
}

// A sharee may not hand on more than it holds, and the rejection says which field was at fault.
func (s *ServiceTestSuite) TestCreateRejectsARuleThatWidens() {
	s.share(map[string]OverlayRule{"assignments": {Editable: false}})

	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{
			InitiatingOUID: rootOU,
			TargetOUScope:  TargetOUScope{AllChildren: true},
			OverlayRules:   map[string]OverlayRule{"assignments": {Editable: true}},
		})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorRuleWidens.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "assignments")
}

func (s *ServiceTestSuite) TestResolveFallsBackToTheDeclaredDefault() {
	s.share(nil)

	resolved, svcErr := s.svc.ResolveOverlayRules(context.Background(), testType, testResource, rootOU)

	s.Require().Nil(svcErr)
	s.Equal(SourceDefault, resolved.Sources["permissions"],
		"a field no policy named comes from the declaration, and says so")
	s.False(resolved.Rules["permissions"].Editable)
	s.True(resolved.Rules["assignments"].Editable)
}

func (s *ServiceTestSuite) TestResolveReportsAPolicySource() {
	s.share(map[string]OverlayRule{"assignments": {Editable: false}})

	resolved, svcErr := s.svc.ResolveOverlayRules(context.Background(), testType, testResource, rootOU)

	s.Require().Nil(svcErr)
	s.Equal(SourcePolicy, resolved.Sources["assignments"])
	s.False(resolved.Rules["assignments"].Editable)
	s.NotEmpty(resolved.PolicyIDs, "an intersection is traceable to its inputs")
}

// The diamond the intersection resolver exists for: two policies cover one organization unit, and
// the narrower one must win on every dimension rather than the deeper one winning wholesale.
func (s *ServiceTestSuite) TestResolveIntersectsEveryCoveringPolicy() {
	s.share(map[string]OverlayRule{"assignments": {Editable: true}})
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  TargetOUScope{AllChildren: true},
		OverlayRules:   map[string]OverlayRule{"assignments": {Editable: false}},
	})
	s.Require().Nil(svcErr)

	resolved, svcErr := s.svc.ResolveOverlayRules(context.Background(), testType, testResource, childOU)

	s.Require().Nil(svcErr)
	s.False(resolved.Rules["assignments"].Editable,
		"the narrower covering policy decides, whichever depth it sits at")
}

func (s *ServiceTestSuite) TestIsVisibleFollowsTheChain() {
	s.share(nil)

	rootVisible, _ := s.svc.IsVisible(context.Background(), testType, testResource, rootOU)
	childVisible, _ := s.svc.IsVisible(context.Background(), testType, testResource, childOU)
	otherVisible, _ := s.svc.IsVisible(context.Background(), testType, testResource, otherOU)

	s.True(rootVisible)
	s.False(childVisible, "a child is not visible merely because its root is")
	s.False(otherVisible)
}

func (s *ServiceTestSuite) TestUpdateRejectsAStaleVersion() {
	p := s.share(nil)

	_, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		Version:       p.Version + 1,
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorVersionMismatch.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestUpdateGrowsASelectivePolicy() {
	p := s.share(nil)

	updated, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU, otherOU}},
		Version:       p.Version,
	})

	s.Require().Nil(svcErr)
	s.Len(updated.Targets, 2, "a selective policy may add targets it could have named at creation")
}

// A blanket policy already reaches everything in its family, so converting it would change the
// meaning of every reshare derived from it.
func (s *ServiceTestSuite) TestUpdateRefusesToConvertABlanketPolicy() {
	p, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{AllRoots: true},
	})
	s.Require().Nil(svcErr)

	_, svcErr = s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		Version:       p.Version,
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorBlanketNarrowOnly.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestUpdateAddsExclusionsToABlanketPolicy() {
	p, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{AllRoots: true},
	})
	s.Require().Nil(svcErr)

	updated, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{AllRoots: true, ExcludedRootOUIDs: []string{otherOU}},
		Version:       p.Version,
	})

	s.Require().Nil(svcErr)
	s.Equal([]string{otherOU}, updated.ExcludedOUIDs)
}

// Deleting one of two covering policies must not wipe state the surviving one still authorizes,
// which is why the cleanup is driven by visibility after the delete rather than by target lists.
func (s *ServiceTestSuite) TestDeleteOnlyCleansUpOUsThatActuallyLoseVisibility() {
	first := s.share(map[string]OverlayRule{"assignments": {Editable: true}})
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  TargetOUScope{OUIDs: []TargetEntry{{OUID: childOU}}},
	})
	s.Require().Nil(svcErr)

	// Removing the owner's policy cuts the root off, and everything under it with it.
	s.Require().Nil(s.svc.DeletePolicy(context.Background(), first.ID))

	notified := make([]string, 0, len(s.decl.lostCalls))
	for _, c := range s.decl.lostCalls {
		notified = append(notified, c.ouID)
		s.Equal([]string{"assignments"}, c.fields,
			"only the fields the removed policy governed are cleaned up")
	}
	s.ElementsMatch([]string{rootOU, childOU, grandOU, greatOU, nestedOwnerOU}, notified)
}

// The candidate set is what a removed target could have reached; the hook only fires for those that
// no longer see the resource once the delete has happened.
func (s *ServiceTestSuite) TestDeleteSkipsOUsAnotherPolicyStillCovers() {
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{AllOUs: true}})
	s.Require().Nil(svcErr)

	reshare, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{
			InitiatingOUID: rootOU,
			TargetOUScope:  TargetOUScope{AllChildren: true},
			OverlayRules:   map[string]OverlayRule{"assignments": {Editable: true}},
		})
	s.Require().Nil(svcErr)

	s.Require().Nil(s.svc.DeletePolicy(context.Background(), reshare.ID))

	s.Empty(s.decl.lostCalls,
		"the deployment-wide policy still covers every unit the removed one reached")
}

func (s *ServiceTestSuite) TestExportOrdersParentsBeforeTheirReshares() {
	ownerPolicy := s.share(nil)
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  TargetOUScope{AllChildren: true},
	})
	s.Require().Nil(svcErr)

	exported, svcErr := s.svc.ExportPolicies(context.Background(), testType, testResource)

	s.Require().Nil(svcErr)
	require.Len(s.T(), exported, 2)
	s.Equal(ownerOU, exported[0].InitiatingOUID, "replaying in order keeps every step valid")
	s.Equal(rootOU, exported[1].InitiatingOUID)
	_ = ownerPolicy
}

func (s *ServiceTestSuite) TestExportRoundTripsTheTargetScope() {
	s.share(nil)

	exported, svcErr := s.svc.ExportPolicies(context.Background(), testType, testResource)

	s.Require().Nil(svcErr)
	require.Len(s.T(), exported, 1)
	assert.Equal(s.T(), []string{rootOU}, exported[0].Request.TargetOUScope.RootOUIDs)
}

func (s *ServiceTestSuite) TestDeclarativePolicyIsHeldInMemoryAndCannotBeEdited() {
	declStore := newDeclarativePolicyStore()
	resolver := &fakeResolver{ancestors: map[string][]string{rootOU: {}, ownerOU: {}}}
	svc := newService(s.store, declStore, resolver, resolver, &fakeTransactioner{}, nil, nil, false)
	svc.RegisterResourceType(s.decl)

	p, svcErr := svc.CreateDeclarativePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}}})
	s.Require().Nil(svcErr)

	s.Empty(s.store.policies, "a declared policy is never written to the database")

	_, svcErr = svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}}, Version: p.Version})
	s.Require().NotNil(svcErr)
	s.Equal(ErrorPolicyDeclared.Code, svcErr.Code)

	s.Require().NotNil(svc.DeletePolicy(context.Background(), p.ID))
}

// Re-loading a file replaces its policy rather than appending, which is what stops restarts
// accumulating duplicates.
func (s *ServiceTestSuite) TestDeclarativeReplayIsIdempotent() {
	declStore := newDeclarativePolicyStore()
	first := Policy{ID: "a", ResourceType: testType, ResourceID: testResource, InitiatingOUID: ownerOU}
	second := Policy{ID: "b", ResourceType: testType, ResourceID: testResource, InitiatingOUID: ownerOU}

	declStore.seed(first)
	declStore.seed(second)

	s.Len(declStore.listForResource(testType, testResource), 1)
}

// entry is a named target organization unit within a children-mode scope.
func entry(ouID string) TargetOUScope {
	return TargetOUScope{OUIDs: []TargetEntry{{OUID: ouID}}}
}

// The one-hop rule: a policy may only name organization units directly beneath its initiator, so
// naming a grandchild, a sibling tree or the initiator itself is refused.
func (s *ServiceTestSuite) TestCreateRejectsTargetsThatAreNotDirectChildren() {
	tests := []struct {
		name  string
		ouID  string
		claim string
	}{
		{"a grandchild skips a hop", grandOU, "reaching two levels down is the child's decision"},
		{"a foreign tree", otherOU, "a target outside the initiator's tree is not beneath it"},
		{"a root above the initiator", rootOU, "an ancestor is not a child"},
		{"the initiator itself", ownerOU, "a policy cannot target its own initiator"},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
				PolicyRequest{TargetOUScope: entry(tt.ouID)})

			s.Require().NotNil(svcErr, tt.claim)
			s.Equal(ErrorInvalidTargetOU.Code, svcErr.Code)
		})
	}
}

func (s *ServiceTestSuite) TestCreateAcceptsADirectChildTarget() {
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, rootOU,
		PolicyRequest{TargetOUScope: entry(childOU)})

	s.Require().Nil(svcErr)
}

// A subtree target still only names one hop down; the depth beneath it comes from the scope.
func (s *ServiceTestSuite) TestCreateAcceptsADirectChildCarryingItsSubtree() {
	p, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, rootOU,
		PolicyRequest{TargetOUScope: TargetOUScope{
			OUIDs: []TargetEntry{{OUID: childOU, AllChildren: true}},
		}})

	s.Require().Nil(svcErr)
	s.Require().Len(p.Targets, 1)
	s.Equal(TargetScopeOUSubtree, p.Targets[0].Scope)
}

func (s *ServiceTestSuite) TestCreateRejectsANamedRootThatIsNotARoot() {
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{childOU}}})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidTargetOU.Code, svcErr.Code)
}

// A root organization unit reaching other roots is the ordinary business-to-business case.
func (s *ServiceTestSuite) TestCreateAllowsARootOwnerToReachAnotherTree() {
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{otherOU}}})

	s.Require().Nil(svcErr)
}

// An owner sitting inside a tree is what the configuration gates.
func (s *ServiceTestSuite) TestCreateRejectsCrossTreeReachFromANestedOwner() {
	tests := []struct {
		name  string
		scope TargetOUScope
	}{
		{"a named foreign root", TargetOUScope{RootOUIDs: []string{otherOU}}},
		{"every root", TargetOUScope{AllRoots: true}},
		{"every organization unit", TargetOUScope{AllOUs: true}},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource,
				nestedOwnerOU, PolicyRequest{TargetOUScope: tt.scope})

			s.Require().NotNil(svcErr)
			s.Equal(ErrorCrossTreeShareRestricted.Code, svcErr.Code)
		})
	}
}

// Reaching the initiator's own tree root never leaves the tree, so it is not gated.
func (s *ServiceTestSuite) TestCreateAllowsANestedOwnerToReachItsOwnTreeRoot() {
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, nestedOwnerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}}})

	s.Require().Nil(svcErr)
}

func (s *ServiceTestSuite) TestCreateAllowsCrossTreeReachWhenConfigured() {
	permissive := newService(newFakeStore(), nil, testResolver(), testResolver(), &fakeTransactioner{}, nil, nil, true)
	permissive.RegisterResourceType(&testDeclaration{})

	_, svcErr := permissive.CreatePolicy(context.Background(), testType, testResource,
		nestedOwnerOU, PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{otherOU}}})

	s.Require().Nil(svcErr)
}

// Editing a policy rebuilds its targets, so an edit is not a way around the one-hop rule.
func (s *ServiceTestSuite) TestUpdateRejectsATargetThatIsNotADirectChild() {
	created, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, rootOU,
		PolicyRequest{TargetOUScope: entry(childOU)})
	s.Require().Nil(svcErr)

	_, svcErr = s.svc.UpdatePolicy(context.Background(), created.ID,
		PolicyRequest{TargetOUScope: entry(grandOU), Version: created.Version})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidTargetOU.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestUpdateRejectsCrossTreeReachFromANestedOwner() {
	created, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, nestedOwnerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}}})
	s.Require().Nil(svcErr)

	_, svcErr = s.svc.UpdatePolicy(context.Background(), created.ID,
		PolicyRequest{
			TargetOUScope: TargetOUScope{RootOUIDs: []string{otherOU}},
			Version:       created.Version,
		})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorCrossTreeShareRestricted.Code, svcErr.Code)
}

// A policy a resource file declares runs the identical validation, so a file cannot assert reach
// its initiator does not have.
func (s *ServiceTestSuite) TestCreateDeclarativeRejectsInvalidTargets() {
	declSvc := newService(newFakeStore(), newDeclarativePolicyStore(), testResolver(), testResolver(),
		&fakeTransactioner{}, nil, nil, false)
	declSvc.RegisterResourceType(&testDeclaration{})

	tests := []struct {
		name  string
		owner string
		scope TargetOUScope
		code  string
	}{
		{"a grandchild skips a hop", rootOU, entry(grandOU), ErrorInvalidTargetOU.Code},
		{
			"a nested owner reaching a foreign tree", nestedOwnerOU,
			TargetOUScope{RootOUIDs: []string{otherOU}}, ErrorCrossTreeShareRestricted.Code,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			_, svcErr := declSvc.CreateDeclarativePolicy(context.Background(), testType, testResource,
				tt.owner, PolicyRequest{TargetOUScope: tt.scope})

			s.Require().NotNil(svcErr)
			s.Equal(tt.code, svcErr.Code)
		})
	}
}

// buildMultiCoveredChain sets up an organization unit reached by two policies at once.
//
// rootOU shares childOU's whole subtree, which reaches grandOU from two levels up, while childOU
// separately names grandOU directly. grandOU is therefore covered by both, and its effective rules
// are the intersection of the two, so neither one alone is its ceiling.
func (s *ServiceTestSuite) buildMultiCoveredChain() (subtree, direct, reshare Policy) {
	ctx := context.Background()
	// Only the owner names a bound; every reshare below inherits it, which is what lets a narrowing
	// anywhere above show up at the bottom.
	inherit := map[string]OverlayRule{"assignments": {Editable: true}}

	_, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules: map[string]OverlayRule{
			"assignments": {Editable: true, AllowedValues: members("a", "b", "c")},
		},
	})
	s.Require().Nil(svcErr)

	subtree, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  TargetOUScope{OUIDs: []TargetEntry{{OUID: childOU, AllChildren: true}}},
		OverlayRules:   inherit,
	})
	s.Require().Nil(svcErr)

	direct, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: childOU,
		TargetOUScope:  entry(grandOU),
		OverlayRules:   inherit,
	})
	s.Require().Nil(svcErr)

	reshare, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: grandOU,
		TargetOUScope:  entry(greatOU),
		OverlayRules:   inherit,
	})
	s.Require().Nil(svcErr)

	s.Require().Equal([]string{"a", "b", "c"}, s.resolvedBound(reshare.ID),
		"the reshare starts at the owner's bound, inherited through both covering policies")
	return subtree, direct, reshare
}

// resolvedBound reads the stored allowed set of a policy's assignments rule.
func (s *ServiceTestSuite) resolvedBound(policyID string) []string {
	p, svcErr := s.svc.GetPolicy(context.Background(), policyID)
	s.Require().Nil(svcErr)
	for _, r := range p.Rules {
		if r.FieldKey == "assignments" {
			s.Require().NotNil(r.Resolved.AllowedValues)
			return *r.Resolved.AllowedValues
		}
	}
	s.Require().Fail("no assignments rule stored on " + policyID)
	return nil
}

// Narrowing either covering policy has to reach the reshare beneath them. Following a single stored
// parent would only catch whichever one happened to be recorded.
func (s *ServiceTestSuite) TestUpdateRematerializesThroughEveryCoveringPolicy() {
	narrow := map[string]OverlayRule{"assignments": {Editable: true, AllowedValues: members("a")}}

	s.Run("narrowing the subtree policy", func() {
		s.SetupTest()
		subtree, _, reshare := s.buildMultiCoveredChain()

		_, svcErr := s.svc.UpdatePolicy(context.Background(), subtree.ID, PolicyRequest{
			InitiatingOUID: rootOU,
			TargetOUScope:  TargetOUScope{OUIDs: []TargetEntry{{OUID: childOU, AllChildren: true}}},
			OverlayRules:   narrow,
			Version:        subtree.Version,
		})
		s.Require().Nil(svcErr)

		s.Equal([]string{"a"}, s.resolvedBound(reshare.ID))
	})

	s.Run("narrowing the direct policy", func() {
		s.SetupTest()
		_, direct, reshare := s.buildMultiCoveredChain()

		_, svcErr := s.svc.UpdatePolicy(context.Background(), direct.ID, PolicyRequest{
			InitiatingOUID: childOU,
			TargetOUScope:  entry(grandOU),
			OverlayRules:   narrow,
			Version:        direct.Version,
		})
		s.Require().Nil(svcErr)

		s.Equal([]string{"a"}, s.resolvedBound(reshare.ID))
	})
}

// Recomputing every reshare must not rewrite the ones that did not change, or an unrelated edit
// would bump their version and fail a concurrent editor holding a perfectly current read.
func (s *ServiceTestSuite) TestUpdateLeavesUnaffectedPoliciesAtTheirVersion() {
	subtree, _, reshare := s.buildMultiCoveredChain()
	narrowed := PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  TargetOUScope{OUIDs: []TargetEntry{{OUID: childOU, AllChildren: true}}},
		OverlayRules:   map[string]OverlayRule{"assignments": {Editable: true, AllowedValues: members("a")}},
	}

	narrowed.Version = subtree.Version
	updated, svcErr := s.svc.UpdatePolicy(context.Background(), subtree.ID, narrowed)
	s.Require().Nil(svcErr)

	afterChange, svcErr := s.svc.GetPolicy(context.Background(), reshare.ID)
	s.Require().Nil(svcErr)
	s.Greater(afterChange.Version, reshare.Version, "a reshare whose ceiling moved is rewritten")

	// The same edit again resolves to the same rules, so nothing beneath it should be touched.
	narrowed.Version = updated.Version
	_, svcErr = s.svc.UpdatePolicy(context.Background(), subtree.ID, narrowed)
	s.Require().Nil(svcErr)

	afterNoChange, svcErr := s.svc.GetPolicy(context.Background(), reshare.ID)
	s.Require().Nil(svcErr)
	s.Equal(afterChange.Version, afterNoChange.Version, "an unchanged reshare keeps its version")
}

// A declared policy and a stored one can only collide on id through a uuid collision, but the
// export has to stay total if they ever do: one must not silently stand in for the other.
func (s *ServiceTestSuite) TestExportKeepsBothPoliciesWhenIDsCollide() {
	declStore := newDeclarativePolicyStore()
	svc := newService(newFakeStore(), declStore, testResolver(), testResolver(), &fakeTransactioner{}, nil, nil, false)
	svc.RegisterResourceType(&testDeclaration{})

	stored, svcErr := svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}}})
	s.Require().Nil(svcErr)

	declStore.seed(Policy{
		ID:             stored.ID,
		ResourceType:   testType,
		ResourceID:     testResource,
		OwningOUID:     ownerOU,
		InitiatingOUID: otherOU,
		Stage:          StageShare,
		Declared:       true,
		Targets:        []Target{{ID: newID(), Scope: TargetScopeRoot, OUID: otherOU}},
	})

	exported, svcErr := svc.ExportPolicies(context.Background(), testType, testResource)
	s.Require().Nil(svcErr)

	s.Require().Len(exported, 2)
	s.ElementsMatch([]string{ownerOU, otherOU},
		[]string{exported[0].InitiatingOUID, exported[1].InitiatingOUID},
		"one policy stood in for the other instead of both being exported")
}

// fakeCache is a minimal in-memory cache, so the caching path itself can be asserted on.
type fakeCache struct {
	values map[string]bool
	sets   int
}

func newFakeCache() *fakeCache { return &fakeCache{values: map[string]bool{}} }

func (c *fakeCache) GetName() string { return "fake" }
func (c *fakeCache) Set(_ context.Context, key cache.CacheKey, value bool) error {
	c.sets++
	c.values[key.Key] = value
	return nil
}

func (c *fakeCache) Get(_ context.Context, key cache.CacheKey) (bool, bool) {
	v, ok := c.values[key.Key]
	return v, ok
}
func (c *fakeCache) Delete(_ context.Context, key cache.CacheKey) error {
	delete(c.values, key.Key)
	return nil
}
func (c *fakeCache) Clear(_ context.Context) error { c.values = map[string]bool{}; return nil }
func (c *fakeCache) IsEnabled() bool               { return true }
func (c *fakeCache) GetStats() cache.CacheStat     { return cache.CacheStat{} }
func (c *fakeCache) CleanupExpired()               {}

// The owner can see its own resource before anyone has shared anything, so visibility cannot be
// derived from the policies alone: there are none to derive it from.
func (s *ServiceTestSuite) TestOwnerSeesAResourceWithNoPolicies() {
	visible, svcErr := s.svc.IsVisible(context.Background(), testType, testResource, ownerOU)

	s.Require().Nil(svcErr)
	s.True(visible)
}

func (s *ServiceTestSuite) TestAStrangerSeesNothingWithNoPolicies() {
	visible, svcErr := s.svc.IsVisible(context.Background(), testType, testResource, otherOU)

	s.Require().Nil(svcErr)
	s.False(visible)
}

// A contract guard rather than a regression test for the short-circuit: while every policy for the
// resource is fetched, the engine still finds the owner on policies[0] and answers this correctly on
// its own. It becomes load-bearing once fetching is narrowed to the asking unit's chain, because a
// policy aimed at another tree would then not be fetched at all.
func (s *ServiceTestSuite) TestOwnerSeesAResourceSharedOnlyElsewhere() {
	s.share(nil)

	visible, svcErr := s.svc.IsVisible(context.Background(), testType, testResource, ownerOU)

	s.Require().Nil(svcErr)
	s.True(visible)
}

// A wrong answer that gets cached stays wrong for the life of the cache, so the owner's result must
// never reach it.
func (s *ServiceTestSuite) TestOwnerVisibilityIsNeverCached() {
	visibility := newFakeCache()
	svc := newService(newFakeStore(), nil, testResolver(), testResolver(), &fakeTransactioner{}, visibility, nil, false)
	svc.RegisterResourceType(&testDeclaration{})

	for range 2 {
		visible, svcErr := svc.IsVisible(context.Background(), testType, testResource, ownerOU)
		s.Require().Nil(svcErr)
		s.True(visible)
	}

	s.Zero(visibility.sets, "the owner's answer was written to the cache")
}

// One policy per organization unit is enforced by the database, not by the check that precedes the
// insert, so the request that loses the race has to be told which policy already holds the slot.
func (s *ServiceTestSuite) TestCreateNamesTheWinnerWhenTwoCreatesRace() {
	winner := Policy{
		ID:             "winning-policy",
		ResourceType:   testType,
		ResourceID:     testResource,
		OwningOUID:     ownerOU,
		InitiatingOUID: ownerOU,
		Stage:          StageShare,
		Version:        1,
	}
	s.store.beforeCreate = func() {
		s.store.policies[winner.ID] = winner
		s.store.failNext = errors.New("duplicate key value violates unique constraint")
		s.store.beforeCreate = nil
	}

	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}}})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorPolicyExists.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, winner.ID,
		"the refusal names the policy to edit instead of reporting a server error")
}

// A failure that is not a lost race still has to surface as one, or a real write fault would be
// reported as a routine conflict.
func (s *ServiceTestSuite) TestCreateStillReportsAGenuineWriteFailure() {
	s.store.failNext = errors.New("disk on fire")

	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}}})

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// Deleting an all-children policy takes visibility away from every unit beneath the initiator, so
// each of them needs its state cleaned up.
func (s *ServiceTestSuite) TestDeleteCleansUpEveryOUBeneathAnAllChildrenTarget() {
	s.share(nil)
	reshare, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{
			InitiatingOUID: rootOU,
			TargetOUScope:  TargetOUScope{AllChildren: true},
			OverlayRules:   map[string]OverlayRule{"assignments": {Editable: true}},
		})
	s.Require().Nil(svcErr)

	s.Require().Nil(s.svc.DeletePolicy(context.Background(), reshare.ID))

	notified := make([]string, 0, len(s.decl.lostCalls))
	for _, c := range s.decl.lostCalls {
		notified = append(notified, c.ouID)
	}
	// Every unit beneath rootOU, at any depth, not just the target's named anchor.
	s.ElementsMatch([]string{childOU, grandOU, greatOU, nestedOwnerOU}, notified,
		"every unit the policy reached lost visibility and must be cleaned up")
}

// A blanket target carries no organization unit id at all, so reading the targets alone cleans up
// nobody. Losing a root also cuts off everything beneath it.
func (s *ServiceTestSuite) TestDeleteCleansUpAfterABlanketTarget() {
	for _, scope := range []TargetOUScope{{AllRoots: true}, {AllOUs: true}} {
		s.SetupTest()
		p, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
			PolicyRequest{
				TargetOUScope: scope,
				OverlayRules:  map[string]OverlayRule{"assignments": {Editable: true}},
			})
		s.Require().Nil(svcErr)

		s.Require().Nil(s.svc.DeletePolicy(context.Background(), p.ID))

		notified := make([]string, 0, len(s.decl.lostCalls))
		for _, c := range s.decl.lostCalls {
			notified = append(notified, c.ouID)
		}
		s.ElementsMatch(
			[]string{rootOU, childOU, grandOU, greatOU, otherOU, nestedOwnerOU}, notified,
			"a blanket target reaches the deployment, and the owner is excluded by still seeing it")
	}
}

// A subtree target names its anchor, but reaches everything under it too.
func (s *ServiceTestSuite) TestDeleteCleansUpBeneathASubtreeTarget() {
	s.share(nil)
	p, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{
			InitiatingOUID: rootOU,
			TargetOUScope:  TargetOUScope{OUIDs: []TargetEntry{{OUID: childOU, AllChildren: true}}},
			OverlayRules:   map[string]OverlayRule{"assignments": {Editable: true}},
		})
	s.Require().Nil(svcErr)

	s.Require().Nil(s.svc.DeletePolicy(context.Background(), p.ID))

	notified := make([]string, 0, len(s.decl.lostCalls))
	for _, c := range s.decl.lostCalls {
		notified = append(notified, c.ouID)
	}
	s.ElementsMatch([]string{childOU, grandOU, greatOU}, notified)
}

// fakeOverlayCache records reads and writes so the write path can be checked for touching it.
type fakeOverlayCache struct {
	values map[string]ResolvedOverlay
	gets   int
	sets   int
}

func newFakeOverlayCache() *fakeOverlayCache {
	return &fakeOverlayCache{values: map[string]ResolvedOverlay{}}
}

func (c *fakeOverlayCache) GetName() string { return "fake-overlay" }
func (c *fakeOverlayCache) Set(_ context.Context, key cache.CacheKey, value ResolvedOverlay) error {
	c.sets++
	c.values[key.Key] = value
	return nil
}

func (c *fakeOverlayCache) Get(_ context.Context, key cache.CacheKey) (ResolvedOverlay, bool) {
	c.gets++
	v, ok := c.values[key.Key]
	return v, ok
}

func (c *fakeOverlayCache) Delete(_ context.Context, key cache.CacheKey) error {
	delete(c.values, key.Key)
	return nil
}

func (c *fakeOverlayCache) Clear(_ context.Context) error {
	c.values = map[string]ResolvedOverlay{}
	return nil
}
func (c *fakeOverlayCache) IsEnabled() bool           { return true }
func (c *fakeOverlayCache) GetStats() cache.CacheStat { return cache.CacheStat{} }
func (c *fakeOverlayCache) CleanupExpired()           {}

// Resolution inside a write must not read the cache, or it applies the ceiling as it stood before
// the edit, and must not populate it, or a rolled-back transaction leaves its state behind.
func (s *ServiceTestSuite) TestWritesDoNotConsultTheOverlayCache() {
	overlay := newFakeOverlayCache()
	resolver := testResolver()
	svc := newService(s.store, nil, resolver, resolver, &fakeTransactioner{}, nil, overlay, false)
	svc.RegisterResourceType(&testDeclaration{})
	ctx := context.Background()

	owner, svcErr := svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules: map[string]OverlayRule{
			"assignments": {Editable: true, AllowedValues: members("a", "b")},
		},
	})
	s.Require().Nil(svcErr)

	overlay.gets, overlay.sets = 0, 0

	// A reshare resolves its initiator's ceiling, which is exactly the read that used to be cached.
	_, svcErr = svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  entry(childOU),
		OverlayRules:   map[string]OverlayRule{"assignments": {Editable: true}},
	})
	s.Require().Nil(svcErr)

	s.Zero(overlay.gets, "the write path read the cache")
	s.Zero(overlay.sets, "the write path populated the cache from inside the transaction")
	s.NotNil(owner)
}

// The cached entry point still caches, so the fix does not simply disable the cache.
func (s *ServiceTestSuite) TestReadsStillUseTheOverlayCache() {
	overlay := newFakeOverlayCache()
	resolver := testResolver()
	svc := newService(s.store, nil, resolver, resolver, &fakeTransactioner{}, nil, overlay, false)
	svc.RegisterResourceType(&testDeclaration{})
	ctx := context.Background()

	_, svcErr := svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules:  map[string]OverlayRule{"assignments": {Editable: true}},
	})
	s.Require().Nil(svcErr)
	overlay.sets = 0

	_, svcErr = svc.ResolveOverlayRules(ctx, testType, testResource, rootOU)
	s.Require().Nil(svcErr)
	s.Equal(1, overlay.sets, "a read populates the cache")

	_, svcErr = svc.ResolveOverlayRules(ctx, testType, testResource, rootOU)
	s.Require().Nil(svcErr)
	s.Equal(1, overlay.sets, "a second read is served from it")
}
