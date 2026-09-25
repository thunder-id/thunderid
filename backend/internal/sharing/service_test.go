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

// newFakeStore returns an empty in-memory store.
func newFakeStore() *fakeStore {
	return &fakeStore{policies: map[string]Policy{}, values: map[string]map[string][]string{}}
}

// CreatePolicy records a policy, honoring the beforeCreate and failNext hooks a test may set.
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

// GetPolicy returns one policy by id.
func (f *fakeStore) GetPolicy(_ context.Context, id string) (Policy, error) {
	p, ok := f.policies[id]
	if !ok {
		return Policy{}, ErrPolicyNotFound
	}
	return p, nil
}

// GetPolicyByInitiator returns the one policy an organization unit holds for a resource.
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

// ListPoliciesForResource returns every policy recorded for one resource.
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

// ListPoliciesRelevantToChain mirrors the SQL predicate rather than returning everything, so a test
// that depends on the narrowed fetch being correct actually exercises it. A policy reaches the
// chain through a blanket scope or through a target anchored on one of its members; whether it
// really covers the asking unit is decided by the engine afterwards, exactly as in the database.
func (f *fakeStore) ListPoliciesRelevantToChain(
	_ context.Context, rt ResourceType, resourceID string, chainOUIDs []string,
) ([]Policy, error) {
	inChain := make(map[string]struct{}, len(chainOUIDs))
	for _, id := range chainOUIDs {
		inChain[id] = struct{}{}
	}

	out := make([]Policy, 0, len(f.policies))
	for _, p := range f.policies {
		if p.ResourceType != rt {
			continue
		}
		if resourceID != "" && p.ResourceID != resourceID {
			continue
		}
		for _, t := range p.Targets {
			_, anchored := inChain[t.OUID]
			if t.Scope == TargetScopeAllOUs || t.Scope == TargetScopeAllRoots || anchored {
				out = append(out, p)
				break
			}
		}
	}
	return out, nil
}

// ListStoredPolicyIDsForResource returns the ids written to the store for one resource, which is
// how a declared policy is told apart from one that has been edited.
func (f *fakeStore) ListStoredPolicyIDsForResource(
	_ context.Context, rt ResourceType, resourceID string,
) ([]string, error) {
	out := make([]string, 0, len(f.policies))
	for _, p := range f.policies {
		if p.ResourceType == rt && p.ResourceID == resourceID {
			out = append(out, p.ID)
		}
	}
	return out, nil
}

// ReplacePolicyContents rewrites a policy, refusing the write when the expected version is stale.
func (f *fakeStore) ReplacePolicyContents(_ context.Context, p Policy, expectedVersion int) error {
	current, ok := f.policies[p.ID]
	if !ok || current.Version != expectedVersion {
		return ErrPolicyNotFound
	}
	p.Version = current.Version + 1
	f.policies[p.ID] = p
	return nil
}

// DeletePolicy removes a policy.
func (f *fakeStore) DeletePolicy(_ context.Context, id string) error {
	delete(f.policies, id)
	return nil
}

// GetOverlayValues returns one organization unit's own values for a resource, by field.
func (f *fakeStore) GetOverlayValues(
	_ context.Context, rt ResourceType, resourceID, ouID string,
) (map[string][]string, error) {
	return f.values[string(rt)+resourceID+ouID], nil
}

// SetOverlayValue records one organization unit's value for one field.
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

// DeleteOverlayValue removes one organization unit's value for one field.
func (f *fakeStore) DeleteOverlayValue(
	_ context.Context, rt ResourceType, resourceID, ouID, fieldKey string,
) error {
	delete(f.values[string(rt)+resourceID+ouID], fieldKey)
	return nil
}

// fakeResolver answers ancestor queries from a fixed tree.
type fakeResolver struct{ ancestors map[string][]string }

// GetAncestorOUIDs returns the fixed ancestor chain for an organization unit.
func (f *fakeResolver) GetAncestorOUIDs(_ context.Context, ouID string) ([]string, *tidcommon.ServiceError) {
	return f.ancestors[ouID], nil
}

// IsAncestor reports whether one organization unit sits above another in the fixed tree.
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

// AllOUIDs returns every organization unit in the fixed tree.
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

// Transact runs the work inline, with no commit or rollback to model.
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

// ResourceType identifies the declared type.
func (d *testDeclaration) ResourceType() ResourceType { return testType }

// OwningOUID makes the declaration an OwnerResolver, which is how the framework learns who owns a
// resource it stores no row for.
func (d *testDeclaration) OwningOUID(_ context.Context, _ string) (string, *tidcommon.ServiceError) {
	if d.owner == "" {
		return ownerOU, nil
	}
	return d.owner, nil
}

// FieldDelimiter makes the declaration a FieldDelimiterResolver, which a type declaring a
// hierarchical field has to be.
// Deliberately not the engine's default of ":", so a test that passes only because both happen to
// agree is not mistaken for one that exercises the resource type's own separator.
func (d *testDeclaration) FieldDelimiter(
	_ context.Context, _, _ string,
) (string, *tidcommon.ServiceError) {
	return "/", nil
}

// Fields declares one editable reference-set field and one non-editable hierarchy field.
func (d *testDeclaration) Fields() []FieldDeclaration {
	editable := OverlayRule{Editable: true}
	locked := OverlayRule{Editable: false}
	return []FieldDeclaration{
		{Key: "assignments", Kind: FieldReferenceSet, Default: &editable},
		{Key: "assignments.user", FallbackKey: "assignments", Kind: FieldReferenceSet, Default: &editable},
		{Key: "permissions", Kind: FieldHierarchy, Default: &locked},
	}
}

// OnVisibilityLost makes the declaration a PolicyHooks, recording each call so a test can assert
// which organization units were cleaned up.
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

// TestServiceTestSuite runs the sharing service suite.
func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

// SetupTest rebuilds the store, declaration and service, so each test starts from a clean slate.
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

// The framework knows nothing about a type that never registered its declaration.
func (s *ServiceTestSuite) TestCreateRejectsAnUnregisteredResourceType() {
	_, svcErr := s.svc.CreatePolicy(context.Background(), "unknown", testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{AllRoots: true},
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorResourceTypeNotRegistered.Code, svcErr.Code)
}

// The three target modes are alternatives, so naming none or mixing them is malformed.
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

// Blanket and root scopes are the owner's call alone; a sharee reshares downward only.
func (s *ServiceTestSuite) TestCreateRejectsOwnerOnlyScopesFromASharee() {
	s.share(nil)

	for _, scope := range []TargetOUScope{{AllOUs: true}, {AllRoots: true}} {
		_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
			PolicyRequest{InitiatingOUID: rootOU, TargetOUScope: scope})

		s.Require().NotNil(svcErr)
		s.Equal(ErrorInvalidTargetOU.Code, svcErr.Code)
	}
}

// An organization unit that cannot see the resource has nothing to pass on.
func (s *ServiceTestSuite) TestCreateRejectsAReshareFromAnOUWithNoStanding() {
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: otherOU,
		TargetOUScope:  TargetOUScope{AllChildren: true},
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorNotShared.Code, svcErr.Code)
}

// A reshare records its stage and the covering policy it derives from, which fixes export order.
func (s *ServiceTestSuite) TestCreateRecordsStageAndParent() {
	owner := s.share(nil)

	reshared, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{InitiatingOUID: rootOU, TargetOUScope: TargetOUScope{AllChildren: true}})

	s.Require().Nil(svcErr)
	s.Equal(StageReshare, reshared.Stage)
	s.Equal(owner.ID, reshared.ParentPolicyID, "the covering policy is the parent")
}

// Which field keys are valid is the resource type's own declaration, so naming another is refused
// rather than stored and silently ignored.
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

// A field no policy named resolves to the type's declared default.
func (s *ServiceTestSuite) TestResolveFallsBackToTheDeclaredDefault() {
	s.share(nil)

	resolved, svcErr := s.svc.ResolveOverlayRules(context.Background(), testType, testResource, rootOU)

	s.Require().Nil(svcErr)
	s.Equal(SourceDefault, resolved.Sources["permissions"],
		"a field no policy named comes from the declaration, and says so")
	s.False(resolved.Rules["permissions"].Editable)
	s.True(resolved.Rules["assignments"].Editable)
}

// A field a policy did name is reported as coming from the policy, not the default.
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

// Visibility is carried hop by hop down the organization unit chain.
func (s *ServiceTestSuite) TestIsVisibleFollowsTheChain() {
	s.share(nil)

	rootVisible, _ := s.svc.IsVisible(context.Background(), testType, testResource, rootOU)
	childVisible, _ := s.svc.IsVisible(context.Background(), testType, testResource, childOU)
	otherVisible, _ := s.svc.IsVisible(context.Background(), testType, testResource, otherOU)

	s.True(rootVisible)
	s.False(childVisible, "a child is not visible merely because its root is")
	s.False(otherVisible)
}

// Without the version check two administrators editing one policy silently last-write-wins, and
// the loser's carve-outs are exactly what must not vanish.
func (s *ServiceTestSuite) TestUpdateRejectsAStaleVersion() {
	p := s.share(nil)

	_, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		Version:       p.Version + 1,
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorVersionMismatch.Code, svcErr.Code)
}

// A selective policy may grow within the one-hop rule.
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

// Exclusions are the only thing that narrows a blanket policy, so an edit may add them.
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

// Replaying an export in order keeps every step valid, so a parent has to precede what it covers.
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

// An exported policy carries back the scope that recreates it.
func (s *ServiceTestSuite) TestExportRoundTripsTheTargetScope() {
	s.share(nil)

	exported, svcErr := s.svc.ExportPolicies(context.Background(), testType, testResource)

	s.Require().Nil(svcErr)
	require.Len(s.T(), exported, 1)
	assert.Equal(s.T(), []string{rootOU}, exported[0].Request.TargetOUScope.RootOUIDs)
}

// A declared policy lives in memory until it is edited, and editing writes it to the database
// under the same id: the file states where sharing starts, the stored row is what it became.
func (s *ServiceTestSuite) TestDeclarativePolicyMaterializesOnFirstEdit() {
	declStore := newDeclarativePolicyStore()
	resolver := testResolver()
	svc := newService(s.store, declStore, resolver, resolver, &fakeTransactioner{}, nil, nil, false)
	svc.RegisterResourceType(s.decl)
	ctx := context.Background()

	declared, svcErr := svc.CreateDeclarativePolicy(ctx, testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{AllOUs: true}})
	s.Require().Nil(svcErr)
	s.Empty(s.store.policies, "a declared policy is not written to the database until it is edited")
	s.True(declared.Declared)

	edited, svcErr := svc.UpdatePolicy(ctx, declared.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{AllOUs: true, ExcludedOUIDs: []string{otherOU}},
		Version:       declared.Version,
	})
	s.Require().Nil(svcErr)

	s.Len(s.store.policies, 1, "the edit is persisted")
	s.Equal(declared.ID, edited.ID, "the policy keeps its identity across materialization")
	s.False(edited.Declared, "an edited policy is operator-owned from here on")
	s.Equal([]string{otherOU}, edited.ExcludedOUIDs)

	// The stored row supersedes the declared one rather than appearing alongside it.
	list, svcErr := svc.ListPolicies(ctx, testType, testResource)
	s.Require().Nil(svcErr)
	s.Require().Len(list, 1)
	s.False(list[0].Declared)
}

// Deleting the stored row reverts the policy to what the file declares, rather than removing
// sharing the file asked for.
func (s *ServiceTestSuite) TestDeletingAnEditedDeclaredPolicyRevertsToTheFile() {
	declStore := newDeclarativePolicyStore()
	resolver := testResolver()
	svc := newService(s.store, declStore, resolver, resolver, &fakeTransactioner{}, nil, nil, false)
	svc.RegisterResourceType(s.decl)
	ctx := context.Background()

	declared, svcErr := svc.CreateDeclarativePolicy(ctx, testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{AllOUs: true}})
	s.Require().Nil(svcErr)

	_, svcErr = svc.UpdatePolicy(ctx, declared.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{AllOUs: true, ExcludedOUIDs: []string{otherOU}},
		Version:       declared.Version,
	})
	s.Require().Nil(svcErr)
	s.Require().Nil(svc.DeletePolicy(ctx, declared.ID))

	reverted, svcErr := svc.GetPolicy(ctx, declared.ID)
	s.Require().Nil(svcErr)
	s.True(reverted.Declared, "the file's policy applies again")
	s.Empty(reverted.ExcludedOUIDs)
}

// The file owns whether the policy exists, so an unedited declared policy cannot be deleted.
func (s *ServiceTestSuite) TestAnUneditedDeclaredPolicyCannotBeDeleted() {
	declStore := newDeclarativePolicyStore()
	resolver := testResolver()
	svc := newService(s.store, declStore, resolver, resolver, &fakeTransactioner{}, nil, nil, false)
	svc.RegisterResourceType(s.decl)
	ctx := context.Background()

	declared, svcErr := svc.CreateDeclarativePolicy(ctx, testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{AllOUs: true}})
	s.Require().Nil(svcErr)

	svcErr = svc.DeletePolicy(ctx, declared.ID)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorPolicyDeclared.Code, svcErr.Code)
}

// Narrowing the fetch to the asking chain must not let a superseded file policy come back.
//
// Editing a declared policy writes it to the database under the same id. If the edit moves the
// policy's targets off this chain, the stored row is no longer fetched, while the file's original
// still names the chain. Merging the file version in on that basis would restore precisely the
// reach the edit removed, so which ids are stored is asked of the database rather than inferred
// from what the narrowed fetch happened to return.
func (s *ServiceTestSuite) TestAnEditedDeclaredPolicyDoesNotRevertThroughTheNarrowedFetch() {
	declStore := newDeclarativePolicyStore()
	resolver := testResolver()
	svc := newService(s.store, declStore, resolver, resolver, &fakeTransactioner{}, nil, nil, false)
	svc.RegisterResourceType(s.decl)
	ctx := context.Background()

	// The file shares rootOU's resource down childOU's subtree, so grandOU beneath it can see it.
	declared, svcErr := svc.CreateDeclarativePolicy(ctx, testType, testResource, rootOU,
		PolicyRequest{TargetOUScope: TargetOUScope{
			OUIDs: []TargetEntry{{OUID: childOU, AllChildren: true}},
		}})
	s.Require().Nil(svcErr)

	visible, svcErr := svc.IsVisible(ctx, testType, testResource, grandOU)
	s.Require().Nil(svcErr)
	s.Require().True(visible, "the file's policy reaches the subtree")

	// The edit moves the policy onto rootOU's other child, off grandOU's chain entirely, so the
	// stored row is no longer part of a fetch narrowed to that chain.
	_, svcErr = svc.UpdatePolicy(ctx, declared.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{
			OUIDs: []TargetEntry{{OUID: nestedOwnerOU, AllChildren: true}},
		},
		Version: declared.Version,
	})
	s.Require().Nil(svcErr)

	visible, svcErr = svc.IsVisible(ctx, testType, testResource, grandOU)
	s.Require().Nil(svcErr)
	s.False(visible, "the edit removed this subtree, and the file must not restore it")
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

// The ordinary case the one-hop rule allows.
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

// Each named organization unit carries its own overlay rules, so a repeat has no winner to pick.
// The pair differing only in allChildren is the one the database would not catch: the two rows
// take different scopes and satisfy the target uniqueness constraint, leaving one entry's rules
// attached to a target and the other's silently dropped.
func (s *ServiceTestSuite) TestCreateRejectsARepeatedChildTarget() {
	tests := []struct {
		name    string
		entries []TargetEntry
	}{
		{"the same organization unit twice", []TargetEntry{{OUID: childOU}, {OUID: childOU}}},
		{
			"differing only in allChildren",
			[]TargetEntry{{OUID: childOU}, {OUID: childOU, AllChildren: true}},
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			// A rejected case writes nothing, but a regression does, and the policy it leaves
			// behind would fail the next case as a duplicate policy rather than as a repeat.
			s.SetupTest()

			_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, rootOU,
				PolicyRequest{TargetOUScope: TargetOUScope{OUIDs: tt.entries}})

			s.Require().NotNil(svcErr)
			s.Equal(ErrorInvalidTargetOU.Code, svcErr.Code)
		})
	}
}

// Root targeting names a tree root; an organization unit with ancestors is not one.
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

// The cross-tree restriction is a deployment setting, so it can be turned off deliberately.
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

// The restriction applies to edits too, not only to the first create.
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

// newFakeCache returns an empty visibility cache.
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

// An organization unit no policy reaches sees nothing, even with the resource present.
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

// A blanket policy already reaches everything in its family, so the only edit open to it is a
// narrowing one. That holds for its overlay rules as much as for its targets: the error the service
// returns has always promised "exclusions or narrower overlay rules", and the rules half is what
// these cover.
func (s *ServiceTestSuite) TestUpdateHoldsABlanketPolicysRulesToNarrowOnly() {
	// blanketWith creates an owner-issued blanket policy bounded to a and b.
	blanketWith := func() Policy {
		s.SetupTest()
		p, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
			PolicyRequest{
				TargetOUScope: TargetOUScope{AllRoots: true},
				OverlayRules: map[string]OverlayRule{
					"assignments": {Editable: true, AllowedValues: members("a", "b")},
				},
			})
		s.Require().Nil(svcErr)
		return p
	}

	rejected := []struct {
		name  string
		rules map[string]OverlayRule
		claim string
	}{
		{
			"dropping the rule entirely",
			nil,
			"the field would fall back to the type's default, which the rule was overriding",
		},
		{
			"dropping the bound but keeping the rule",
			map[string]OverlayRule{"assignments": {Editable: true}},
			"a rule with no bound reaches the whole universe",
		},
		{
			"growing the bound",
			map[string]OverlayRule{
				"assignments": {Editable: true, AllowedValues: members("a", "b", "c")},
			},
			"c was never within this policy's reach",
		},
	}
	for _, tt := range rejected {
		s.Run(tt.name, func() {
			p := blanketWith()

			_, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
				TargetOUScope: TargetOUScope{AllRoots: true},
				OverlayRules:  tt.rules,
				Version:       p.Version,
			})

			s.Require().NotNil(svcErr, tt.claim)
			s.Equal(ErrorBlanketNarrowOnly.Code, svcErr.Code)
		})
	}

	s.Run("narrowing the bound is still allowed", func() {
		p := blanketWith()

		updated, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
			TargetOUScope: TargetOUScope{AllRoots: true},
			OverlayRules: map[string]OverlayRule{
				"assignments": {Editable: true, AllowedValues: members("a")},
			},
			Version: p.Version,
		})

		s.Require().Nil(svcErr)
		s.Require().Len(updated.Rules, 1)
		s.Equal([]string{"a"}, *updated.Rules[0].Resolved.AllowedValues)
	})

	s.Run("pinning a field the policy left editable is a narrowing", func() {
		p := blanketWith()

		_, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
			TargetOUScope: TargetOUScope{AllRoots: true},
			OverlayRules: map[string]OverlayRule{
				"assignments": {Editable: false, Value: members("a")},
			},
			Version: p.Version,
		})

		s.Require().Nil(svcErr)
	})
}

// The narrow-only rule belongs to the blanket family alone. A selective policy may grow within the
// one-hop rule, and re-materialization is built to restore what an ancestor had clamped, so holding
// it to its own previous rules would make that one-way.
func (s *ServiceTestSuite) TestUpdateStillLetsASelectivePolicyWidenItsRules() {
	p, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{
			TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
			OverlayRules: map[string]OverlayRule{
				"assignments": {Editable: true, AllowedValues: members("a")},
			},
		})
	s.Require().Nil(svcErr)

	updated, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules: map[string]OverlayRule{
			"assignments": {Editable: true, AllowedValues: members("a", "b")},
		},
		Version: p.Version,
	})

	s.Require().Nil(svcErr)
	s.Require().Len(updated.Rules, 1)
	s.ElementsMatch([]string{"a", "b"}, *updated.Rules[0].Resolved.AllowedValues)
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

// lateOwnerDeclaration resolves no owner until its owner field is set, which is what ownerOf
// reports as "not known" rather than as an error.
type lateOwnerDeclaration struct {
	*testDeclaration
	owner string
}

// OwningOUID reports no owner until the owner field is set.
func (d *lateOwnerDeclaration) OwningOUID(
	_ context.Context, _ string,
) (string, *tidcommon.ServiceError) {
	return d.owner, nil
}

// Ownership is asked before the cache is read, not merely kept out of it. While the resource type
// resolves no owner the policy-derived false is cached under the owner's own key, and only a policy
// write ever clears this cache, so a cache read placed first would keep serving that false long
// after ownership became resolvable.
func (s *ServiceTestSuite) TestOwnerIsAnsweredOverAStaleCachedDenial() {
	visibility := newFakeCache()
	decl := &lateOwnerDeclaration{testDeclaration: &testDeclaration{}}
	svc := newService(newFakeStore(), nil, testResolver(), testResolver(), &fakeTransactioner{},
		visibility, nil, false)
	svc.RegisterResourceType(decl)
	ctx := context.Background()

	visible, svcErr := svc.IsVisible(ctx, testType, testResource, ownerOU)
	s.Require().Nil(svcErr)
	s.Require().False(visible, "no owner is resolvable yet and no policy reaches this unit")
	s.Require().Equal(1, visibility.sets, "the denial is cached under the owner's own key")

	decl.owner = ownerOU

	visible, svcErr = svc.IsVisible(ctx, testType, testResource, ownerOU)
	s.Require().Nil(svcErr)
	s.True(visible, "the owner is answered rather than served the cached denial")
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

// fakeOverlayRuleCache records reads and writes so the write path can be checked for touching it.
type fakeOverlayRuleCache struct {
	values map[string]ResolvedOverlay
	gets   int
	sets   int
}

// newFakeOverlayRuleCache returns an empty resolved-overlay cache.
func newFakeOverlayRuleCache() *fakeOverlayRuleCache {
	return &fakeOverlayRuleCache{values: map[string]ResolvedOverlay{}}
}

func (c *fakeOverlayRuleCache) GetName() string { return "fake-overlay" }
func (c *fakeOverlayRuleCache) Set(_ context.Context, key cache.CacheKey, value ResolvedOverlay) error {
	c.sets++
	c.values[key.Key] = value
	return nil
}

func (c *fakeOverlayRuleCache) Get(_ context.Context, key cache.CacheKey) (ResolvedOverlay, bool) {
	c.gets++
	v, ok := c.values[key.Key]
	return v, ok
}

func (c *fakeOverlayRuleCache) Delete(_ context.Context, key cache.CacheKey) error {
	delete(c.values, key.Key)
	return nil
}

func (c *fakeOverlayRuleCache) Clear(_ context.Context) error {
	c.values = map[string]ResolvedOverlay{}
	return nil
}
func (c *fakeOverlayRuleCache) IsEnabled() bool           { return true }
func (c *fakeOverlayRuleCache) GetStats() cache.CacheStat { return cache.CacheStat{} }
func (c *fakeOverlayRuleCache) CleanupExpired()           {}

// Resolution inside a write must not read the cache, or it applies the ceiling as it stood before
// the edit, and must not populate it, or a rolled-back transaction leaves its state behind.
func (s *ServiceTestSuite) TestWritesDoNotConsultTheOverlayCache() {
	overlay := newFakeOverlayRuleCache()
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
	overlay := newFakeOverlayRuleCache()
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

// A hierarchy compared with no delimiter is raw string prefixing, under which a sibling path that
// merely starts with the same characters would be treated as living beneath it.
func (s *ServiceTestSuite) TestHierarchyNarrowingUsesTheResourceTypeDelimiter() {
	// reshareWithPath shares from the owner bounded to "billing", then reshares asking for path.
	reshareWithPath := func(path string) *tidcommon.ServiceError {
		s.SetupTest()
		ctx := context.Background()
		_, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
			TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
			OverlayRules: map[string]OverlayRule{
				"permissions": {Editable: false, Value: members("billing")},
			},
		})
		s.Require().Nil(svcErr)

		_, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
			InitiatingOUID: rootOU,
			TargetOUScope:  entry(childOU),
			OverlayRules: map[string]OverlayRule{
				"permissions": {Editable: false, Value: members(path)},
			},
		})
		return svcErr
	}

	s.Run("a path beneath the bound is within it", func() {
		s.Nil(reshareWithPath("billing/invoice"))
	})

	s.Run("the bound itself is within it", func() {
		s.Nil(reshareWithPath("billing"))
	})

	s.Run("a sibling sharing a character prefix is not", func() {
		svcErr := reshareWithPath("billingx")
		s.Require().NotNil(svcErr, "billingx does not live beneath billing")
		s.Equal(ErrorRuleWidens.Code, svcErr.Code)
	})

	// The engine defaults to ":" when no delimiter is resolved, so a path joined with it must not
	// be read as a descendant of a resource type that separates with "/".
	s.Run("the engine default separator is not the resource type's", func() {
		svcErr := reshareWithPath("billing:invoice")
		s.Require().NotNil(svcErr)
		s.Equal(ErrorRuleWidens.Code, svcErr.Code)
	})
}

// An organization unit the resource never reached has nothing it may do with it. Answering with the
// type's declared defaults would read as though it held the resource on default terms.
func (s *ServiceTestSuite) TestResolveGivesAnUnreachedOUNoRules() {
	s.share(nil)

	resolved, svcErr := s.svc.ResolveOverlayRules(context.Background(), testType, testResource, otherOU)

	s.Require().Nil(svcErr)
	s.False(resolved.Visible, "no policy reaches this organization unit")
	s.Empty(resolved.Rules, "an organization unit that cannot see the resource is given no rules")
	s.Empty(resolved.Sources)
	s.Empty(resolved.PolicyIDs)
}

// The owner always holds its own resource, so it is answered even with no policy at all.
func (s *ServiceTestSuite) TestResolveAnswersTheOwnerWithNoPolicy() {
	resolved, svcErr := s.svc.ResolveOverlayRules(context.Background(), testType, testResource, ownerOU)

	s.Require().Nil(svcErr)
	s.True(resolved.Visible)
}

// A reached organization unit still falls back to the declared defaults for fields no policy named.
func (s *ServiceTestSuite) TestResolveStillDefaultsForAReachedOU() {
	s.share(nil)

	resolved, svcErr := s.svc.ResolveOverlayRules(context.Background(), testType, testResource, rootOU)

	s.Require().Nil(svcErr)
	s.True(resolved.Visible)
	s.Equal(SourceDefault, resolved.Sources["permissions"])
}
