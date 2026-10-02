// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"errors"
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cache"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/cachemock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
	"github.com/thunder-id/thunderid/tests/mocks/transactionmock"
)

const (
	testType = ResourceType("role")
	// fieldlessType declares no overlay field. Sharing it grants the right to be named and nothing
	// else, which is the whole of what sharing means for a resource whose configuration is global
	// rather than per-organization-unit state layered on a shared definition.
	fieldlessType = ResourceType("fieldless")
	testResource  = "resource-1"
	ownerOU       = "owner-ou"
	rootOU        = "root-ou"
	childOU       = "child-ou"
	grandOU       = "grand-ou"
	otherOU       = "other-ou"
	// nestedOwnerOU owns a resource while sitting inside a tree, which is the case the cross-tree
	// restriction exists for.
	nestedOwnerOU = "nested-owner-ou"
	greatOU       = "great-ou"
	// declaredID is the id a declaration carries, the way a declared role carries its own. The
	// same value is reused when a test re-applies the file, which is what makes that an upsert.
	declaredID = "01900000-0000-7000-8000-0000000000d1"
)

// fieldlessDeclaration is a resource type that declares no field. It brings the two required
// capabilities and nothing more: with no field to name, there is no delimiter to resolve, no member
// to validate and no per-organization-unit state to clean up when visibility goes.
type fieldlessDeclaration struct{}

func (d *fieldlessDeclaration) ResourceType() ResourceType { return fieldlessType }
func (d *fieldlessDeclaration) Fields() []FieldDeclaration { return nil }
func (d *fieldlessDeclaration) OwningOUID(
	_ context.Context, _ string,
) (string, *tidcommon.ServiceError) {
	return ownerOU, nil
}

// storeState is the in-memory state behind the generated store mock. The mock supplies the
// interface, so a change to sharingPolicyStoreInterface breaks compilation here rather than going
// unnoticed; this type supplies the behavior, because the service's orchestration is what the
// tests exercise and a create followed by a read has to give the policy back.
type storeState struct {
	policies map[string]Policy
	// order is the fake's CREATED_AT: the ids in the order they were inserted. A map alone gives
	// reads a random order, which a paginated listing cannot be tested against.
	order    []string
	values   map[string]map[string][]string
	failNext error
	// beforeCreate runs at the start of CreatePolicy, so a test can simulate another writer
	// committing in the window between the pre-flight check and this insert.
	beforeCreate func()
	// replaced records the id of every policy whose contents were rewritten, so a test can assert
	// that an edit left one alone. A write followed by a delete looks the same from the outside as
	// no write at all, so the write itself is what has to be observed.
	replaced []string
}

// mustMockStore is newMockStore for the tests that build their own service and never touch the
// state directly.
func mustMockStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *sharingPolicyStoreInterfaceMock {
	m, _ := newMockStore(t)
	return m
}

// newStoreState returns empty store state.
func newStoreState() *storeState {
	return &storeState{policies: map[string]Policy{}, values: map[string]map[string][]string{}}
}

// newMockStore returns a generated store mock backed by fresh state, along with that state so a
// test can seed it or inject a failure. Every method is wired as Maybe, since no test needs every
// one of them and an unused one is not a failure.
func newMockStore(t interface {
	mock.TestingT
	Cleanup(func())
}) (*sharingPolicyStoreInterfaceMock, *storeState) {
	st := newStoreState()
	m := newSharingPolicyStoreInterfaceMock(t)
	e := m.EXPECT()
	a := mock.Anything
	e.CreatePolicy(a, a).RunAndReturn(st.CreatePolicy).Maybe()
	e.GetPolicy(a, a).RunAndReturn(st.GetPolicy).Maybe()
	e.GetPolicyByInitiator(a, a, a, a).RunAndReturn(st.GetPolicyByInitiator).Maybe()
	e.ListPoliciesForResource(a, a, a, a, a).RunAndReturn(st.ListPoliciesForResource).Maybe()
	e.ListAllPoliciesForResource(a, a, a).RunAndReturn(st.ListAllPoliciesForResource).Maybe()
	e.CountPoliciesForResource(a, a, a).RunAndReturn(st.CountPoliciesForResource).Maybe()
	e.ListPoliciesRelevantToChain(a, a, a, a).RunAndReturn(st.ListPoliciesRelevantToChain).Maybe()
	e.ReplacePolicyContents(a, a, a).RunAndReturn(st.ReplacePolicyContents).Maybe()
	e.DeletePolicy(a, a).RunAndReturn(st.DeletePolicy).Maybe()
	e.GetOverlayValues(a, a, a, a).RunAndReturn(st.GetOverlayValues).Maybe()
	e.SetOverlayValue(a, a, a, a, a, a).RunAndReturn(st.SetOverlayValue).Maybe()
	e.DeleteOverlayValue(a, a, a, a, a).RunAndReturn(st.DeleteOverlayValue).Maybe()
	e.DeleteOverlayValuesForOU(a, a, a, a).RunAndReturn(st.DeleteOverlayValuesForOU).Maybe()
	return m, st
}

// CreatePolicy records a policy, honoring the beforeCreate and failNext hooks a test may set.
// asStored mirrors what a read from the database gives back. hydrate only ever appends rule rows,
// so a policy that stores none comes back with Rules still nil rather than an empty slice, and a
// fake that kept the empty slice would hide every nil-versus-empty difference from the tests.
func asStored(p Policy) Policy {
	if len(p.Rules) == 0 {
		p.Rules = nil
	}
	return p
}

func (f *storeState) CreatePolicy(_ context.Context, p Policy) error {
	if f.beforeCreate != nil {
		f.beforeCreate()
	}
	if f.failNext != nil {
		err := f.failNext
		f.failNext = nil
		return err
	}
	if _, existing := f.policies[p.ID]; !existing {
		f.order = append(f.order, p.ID)
	}
	f.policies[p.ID] = asStored(p)
	return nil
}

// GetPolicy returns one policy by id.
func (f *storeState) GetPolicy(_ context.Context, id string) (Policy, error) {
	p, ok := f.policies[id]
	if !ok {
		return Policy{}, errPolicyNotFound
	}
	return p, nil
}

// GetPolicyByInitiator returns the one policy an organization unit holds for a resource.
func (f *storeState) GetPolicyByInitiator(
	_ context.Context, rt ResourceType, resourceID, initiatingOUID string,
) (Policy, error) {
	for _, p := range f.policies {
		if p.ResourceType == rt && p.ResourceID == resourceID && p.InitiatingOUID == initiatingOUID {
			return p, nil
		}
	}
	return Policy{}, errPolicyNotFound
}

// ListAllPoliciesForResource returns the whole ordered set, as the unbounded query does.
func (f *storeState) ListAllPoliciesForResource(
	_ context.Context, rt ResourceType, resourceID string,
) ([]Policy, error) {
	if f.failNext != nil {
		err := f.failNext
		f.failNext = nil
		return nil, err
	}
	return f.forResource(rt, resourceID), nil
}

// ListPoliciesForResource cuts a page from the ordered set, which is what LIMIT and OFFSET do to
// the query's ORDER BY.
func (f *storeState) ListPoliciesForResource(
	_ context.Context, rt ResourceType, resourceID string, limit, offset int,
) ([]Policy, error) {
	if f.failNext != nil {
		err := f.failNext
		f.failNext = nil
		return nil, err
	}
	all := f.forResource(rt, resourceID)
	if offset >= len(all) {
		return nil, nil
	}
	return all[offset:min(offset+limit, len(all))], nil
}

// CountPoliciesForResource mirrors the COUNT the database answers with.
func (f *storeState) CountPoliciesForResource(
	_ context.Context, rt ResourceType, resourceID string,
) (int, error) {
	if f.failNext != nil {
		err := f.failNext
		f.failNext = nil
		return 0, err
	}
	return len(f.forResource(rt, resourceID)), nil
}

// forResource returns a resource's policies in insertion order, as ORDER BY CREATED_AT, ID does.
func (f *storeState) forResource(rt ResourceType, resourceID string) []Policy {
	out := make([]Policy, 0, len(f.policies))
	for _, id := range f.order {
		if p := f.policies[id]; p.ResourceType == rt && p.ResourceID == resourceID {
			out = append(out, p)
		}
	}
	return out
}

// ListPoliciesRelevantToChain mirrors the SQL predicate rather than returning everything, so a test
// that depends on the narrowed fetch being correct actually exercises it. A policy reaches the
// chain through a blanket scope or through a target anchored on one of its members; whether it
// really covers the asking unit is decided by the engine afterwards, exactly as in the database.
func (f *storeState) ListPoliciesRelevantToChain(
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
			if t.Scope == targetScopeAllOUs || t.Scope == targetScopeAllRoots || anchored {
				out = append(out, p)
				break
			}
		}
	}
	return out, nil
}

// ReplacePolicyContents rewrites a policy, refusing the write when the expected version is stale.
func (f *storeState) ReplacePolicyContents(_ context.Context, p Policy, expectedVersion int) error {
	current, ok := f.policies[p.ID]
	if !ok || current.Version != expectedVersion {
		return errPolicyNotFound
	}
	f.replaced = append(f.replaced, p.ID)
	p.Version = current.Version + 1
	f.policies[p.ID] = asStored(p)
	return nil
}

// DeletePolicy removes a policy.
func (f *storeState) DeletePolicy(_ context.Context, id string) error {
	// No cascade: PARENT_POLICY_ID carries no foreign key, so dependent policies are removed by the
	// service rather than by the database.
	delete(f.policies, id)
	f.order = slices.DeleteFunc(f.order, func(existing string) bool { return existing == id })
	return nil
}

// GetOverlayValues returns one organization unit's own values for a resource, by field.
func (f *storeState) GetOverlayValues(
	_ context.Context, rt ResourceType, resourceID, ouID string,
) (map[string][]string, error) {
	return f.values[string(rt)+resourceID+ouID], nil
}

// SetOverlayValue records one organization unit's value for one field.
func (f *storeState) SetOverlayValue(
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
func (f *storeState) DeleteOverlayValue(
	_ context.Context, rt ResourceType, resourceID, ouID, fieldKey string,
) error {
	delete(f.values[string(rt)+resourceID+ouID], fieldKey)
	return nil
}

// DeleteOverlayValuesForOU removes every value one organization unit holds for a resource.
func (f *storeState) DeleteOverlayValuesForOU(
	_ context.Context, rt ResourceType, resourceID, ouID string,
) error {
	delete(f.values, string(rt)+resourceID+ouID)
	return nil
}

// cleanerDeclaration keeps its organization units' values somewhere of its own, which is what
// OverlayCleaner exists for, and records what it was asked to delete.
type cleanerDeclaration struct {
	testDeclaration
	cleaned []string
}

// DeleteOverlayValues makes the declaration an OverlayCleaner.
func (d *cleanerDeclaration) DeleteOverlayValues(_ context.Context, _, ouID string) error {
	d.cleaned = append(d.cleaned, ouID)
	return nil
}

// unresolvedOwnerDeclaration resolves no owner until resolvable is set, which is the window the
// ownership-before-cache ordering exists to survive.
type unresolvedOwnerDeclaration struct {
	testDeclaration
	resolvable bool
}

// OwningOUID reports the owner as unknown until the resource type can answer.
func (d *unresolvedOwnerDeclaration) OwningOUID(
	_ context.Context, _ string,
) (string, *tidcommon.ServiceError) {
	if !d.resolvable {
		return "", nil
	}
	return ownerOU, nil
}

// ouTree is the fixed hierarchy behind the generated resolver mocks. The mocks supply the two
// interfaces, so a change to either breaks compilation here; this supplies the behavior, because
// the service walks the same tree upwards and downwards and the two directions have to agree.
type ouTree struct{ ancestors map[string][]string }

// GetAncestorOUIDs returns the fixed ancestor chain for an organization unit.
func (f *ouTree) GetAncestorOUIDs(_ context.Context, ouID string) ([]string, *tidcommon.ServiceError) {
	return f.ancestors[ouID], nil
}

// IsAncestor reports whether one organization unit sits above another in the fixed tree.
func (f *ouTree) IsAncestor(
	_ context.Context, ancestorOUID, descendantOUID string,
) (bool, *tidcommon.ServiceError) {
	for _, id := range f.ancestors[descendantOUID] {
		if id == ancestorOUID {
			return true, nil
		}
	}
	return false, nil
}

// DescendantOUIDs and AllOUIDs answer the enumerator half, deriving the downward view by
// inverting the same ancestor table the upward walks use.
func (f *ouTree) DescendantOUIDs(_ context.Context, ouID string) ([]string, *tidcommon.ServiceError) {
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
func (f *ouTree) AllOUIDs(_ context.Context) ([]string, *tidcommon.ServiceError) {
	out := make([]string, 0, len(f.ancestors))
	for id := range f.ancestors {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// inlineTx returns the generated transaction mock, set to run the work it is handed. Nothing here
// has anything to roll back: the store is in memory, so commit-on-success is the whole behavior.
func inlineTx(t interface {
	mock.TestingT
	Cleanup(func())
}) *transactionmock.TransactionerMock {
	tx := transactionmock.NewTransactionerMock(t)
	tx.EXPECT().Transact(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Maybe()
	return tx
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

// hierarchyWithoutDelimiter declares a hierarchical field and is not a FieldDelimiterResolver at
// all. It deliberately embeds nothing, because embedding a declaration that resolves one would make
// this a resolver too and there would be nothing left to test.
type hierarchyWithoutDelimiter struct{}

func (d *hierarchyWithoutDelimiter) ResourceType() ResourceType { return testType }

func (d *hierarchyWithoutDelimiter) Fields() []FieldDeclaration {
	locked := OverlayRule{Editable: false}
	return []FieldDeclaration{{Key: "permissions", Kind: FieldHierarchy, Default: &locked}}
}

func (d *hierarchyWithoutDelimiter) OwningOUID(
	_ context.Context, _ string,
) (string, *tidcommon.ServiceError) {
	return ownerOU, nil
}

// emptyDelimiterDeclaration resolves a separator, but an empty one, which leaves the field compared
// by raw string prefixing exactly as if it had resolved none.
type emptyDelimiterDeclaration struct{ hierarchyWithoutDelimiter }

func (d *emptyDelimiterDeclaration) FieldDelimiter(
	_ context.Context, _, _ string,
) (string, *tidcommon.ServiceError) {
	return "", nil
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
func (d *testDeclaration) OnVisibilityLost(_ context.Context, resourceID, ouID string) error {
	d.lostCalls = append(d.lostCalls, lostVisibilityCall{resourceID, ouID})
	return nil
}

type ServiceTestSuite struct {
	suite.Suite
	// storeMock is the generated mock itself, kept so a test can assert which store method a code
	// path reached rather than only what it returned.
	storeMock *sharingPolicyStoreInterfaceMock
	store     *storeState
	declStore *fileBasedStore
	decl      *testDeclaration
	svc       SharingServiceInterface
}

// TestServiceTestSuite runs the sharing service suite.
func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

// SetupTest rebuilds the store, declaration and service, so each test starts from a clean slate.
func (s *ServiceTestSuite) SetupTest() {
	storeMock, storeState := newMockStore(s.T())
	s.storeMock = storeMock
	s.store = storeState
	// The service carries a declarative store, because a declared policy is now only ever held in
	// memory: it never reaches the database, so a test that needs one has to seed it here.
	s.declStore = newFileBasedStore()
	s.decl = &testDeclaration{}
	hierarchy, enumerator := testResolver(s.T())
	s.svc = newSharingService(storeMock, s.declStore, hierarchy, enumerator,
		inlineTx(s.T()), nil, nil, false)
	s.svc.RegisterResourceType(s.decl)
	s.svc.RegisterResourceType(&fieldlessDeclaration{})
}

// testResolver returns the generated hierarchy and enumerator mocks over the fixture tree: two
// roots, one of which has a child and a grandchild, plus a non-root organization unit that owns a
// resource of its own. Both are wired as Maybe, since no test walks the tree in both directions.
func testResolver(t interface {
	mock.TestingT
	Cleanup(func())
}) (*sysauthzmock.OUHierarchyResolverMock, *oumock.HierarchyEnumeratorInterfaceMock) {
	tree := &ouTree{ancestors: map[string][]string{
		rootOU:        {},
		childOU:       {rootOU},
		grandOU:       {childOU, rootOU},
		otherOU:       {},
		ownerOU:       {},
		nestedOwnerOU: {rootOU},
		greatOU:       {grandOU, childOU, rootOU},
	}}
	a := mock.Anything
	hierarchy := sysauthzmock.NewOUHierarchyResolverMock(t)
	hierarchy.EXPECT().GetAncestorOUIDs(a, a).RunAndReturn(tree.GetAncestorOUIDs).Maybe()
	hierarchy.EXPECT().IsAncestor(a, a, a).RunAndReturn(tree.IsAncestor).Maybe()
	enumerator := oumock.NewHierarchyEnumeratorInterfaceMock(t)
	enumerator.EXPECT().DescendantOUIDs(a, a).RunAndReturn(tree.DescendantOUIDs).Maybe()
	enumerator.EXPECT().AllOUIDs(a).RunAndReturn(tree.AllOUIDs).Maybe()
	return hierarchy, enumerator
}

// declarative returns a service backed by the suite's own store and declaration, with a declarative
// policy store attached. The deployment-wide scopes are only reachable through this path, so a test
// that needs one has to come in through here.
func (s *ServiceTestSuite) declarative() SharingServiceInterface {
	return s.svc
}

// storedAllChildren issues the one blanket policy an API can create: the owner reaching its own
// subtree. A deployment-wide policy is declarative-only and never editable, so this is what an edit
// test on a blanket policy has to start from.
func (s *ServiceTestSuite) storedAllChildren(rules map[string]OverlayRule) Policy {
	p, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, rootOU,
		PolicyRequest{TargetOUScope: TargetOUScope{AllChildren: true}, OverlayRules: rules})
	s.Require().Nil(svcErr)
	return p
}

// declaredBlanket declares a deployment-wide policy, which is the only way one can be created.
//
// It stays in memory: a declared policy never reaches the database, so nothing is written to the
// store here and the policy cannot be edited or deleted through the API.
func (s *ServiceTestSuite) declaredBlanket(scope TargetOUScope) Policy {
	p, svcErr := s.svc.CreateDeclarativePolicy(context.Background(), testType, testResource,
		ownerOU, PolicyRequest{ID: declaredID, TargetOUScope: scope})
	s.Require().Nil(svcErr)
	return p
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
		// declared routes the row through the declarative path, which is the only way a request
		// carrying a deployment-wide scope reaches mode validation at all.
		declared bool
	}{
		{"no mode", TargetOUScope{}, false},
		{"two modes", TargetOUScope{RootOUIDs: []string{rootOU}, AllChildren: true}, false},
		{"three modes", TargetOUScope{AllOUs: true, AllRoots: true, AllChildren: true}, true},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			// The id matters only on the declarative path, and is ignored on the other.
			req := PolicyRequest{ID: declaredID, TargetOUScope: tt.scope}
			create := s.svc.CreatePolicy
			if tt.declared {
				create = s.declarative().CreateDeclarativePolicy
			}

			_, svcErr := create(context.Background(), testType, testResource, ownerOU, req)

			s.Require().NotNil(svcErr)
			s.Equal(ErrorInvalidRequestFormat.Code, svcErr.Code)
		})
	}
}

// Deployment-wide reach is not bounded by where the initiator sits, and every organization unit
// owns what it creates, so owner-only is no bound at all for these two. Declaring one in a resource
// file is reviewed; issuing one over the API is not.
func (s *ServiceTestSuite) TestCreateRejectsDeploymentWideScopesFromTheAPI() {
	tests := []struct {
		name   string
		scope  TargetOUScope
		detail string
	}{
		{"every organization unit", TargetOUScope{AllOUs: true}, "allOus"},
		{"every root", TargetOUScope{AllRoots: true}, "allRoots"},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
				PolicyRequest{TargetOUScope: tt.scope})

			s.Require().NotNil(svcErr)
			s.Equal(ErrorDeploymentWideScopeDeclarativeOnly.Code, svcErr.Code)
			s.Contains(svcErr.ErrorDescription.DefaultValue, tt.detail,
				"the refusal names which scope was asked for")
		})
	}
}

// The same two scopes are exactly what a resource file is allowed to declare.
func (s *ServiceTestSuite) TestCreateDeclarativeAllowsDeploymentWideScopes() {
	for _, scope := range []TargetOUScope{{AllOUs: true}, {AllRoots: true}} {
		s.SetupTest()

		p, svcErr := s.declarative().CreateDeclarativePolicy(context.Background(), testType,
			testResource, ownerOU, PolicyRequest{ID: declaredID, TargetOUScope: scope})

		s.Require().Nil(svcErr)
		s.Require().Len(p.Targets, 1)
	}
}

// The restriction covers the two standing grants and nothing else. Naming roots individually is a
// snapshot of the roots that exist today, and it still answers to the cross-tree gate.
func (s *ServiceTestSuite) TestCreateStillAllowsNamedRootsFromTheAPI() {
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU, otherOU}}})

	s.Require().Nil(svcErr)
}

// An edit cannot smuggle in what a create refuses. Every route into a deployment-wide scope is shut
// by the scope-family and blanket rules, so the create-time check needs no counterpart here.
func (s *ServiceTestSuite) TestUpdateCannotIntroduceADeploymentWideScope() {
	// The policy being edited names one root, so it is refused for changing how broadly it reaches.
	// Telling its author that a blanket policy may only be narrowed would describe a policy they are
	// not editing.
	s.Run("from a selective policy", func() {
		s.SetupTest()
		p := s.share(nil)

		_, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
			TargetOUScope: TargetOUScope{AllOUs: true},
			Version:       p.Version,
		})

		s.Require().NotNil(svcErr)
		s.Equal(ErrorScopeFamilyChange.Code, svcErr.Code)
	})

	// Owner-issued, so the edit gets past the owner-only guard and is stopped by the blanket rule
	// itself rather than before it. Both scopes are blanket, so this is the narrowing rule rather
	// than a change of family.
	s.Run("from a blanket policy of another kind", func() {
		s.SetupTest()
		p, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
			PolicyRequest{TargetOUScope: TargetOUScope{AllChildren: true}})
		s.Require().Nil(svcErr)

		_, svcErr = s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
			TargetOUScope: TargetOUScope{AllRoots: true},
			Version:       p.Version,
		})

		s.Require().NotNil(svcErr)
		s.Equal(ErrorBlanketNarrowOnly.Code, svcErr.Code)
	})
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

	// Declaratively, because that is the only path these scopes travel now. The owner-only rule
	// lives in buildTargets and applies to both paths alike, so the claim is unchanged.
	for _, scope := range []TargetOUScope{{AllOUs: true}, {AllRoots: true}} {
		_, svcErr := s.declarative().CreateDeclarativePolicy(context.Background(), testType, testResource,
			ownerOU, PolicyRequest{ID: declaredID, InitiatingOUID: rootOU, TargetOUScope: scope})

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
	s.Equal(stageReshare, reshared.Stage)
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

// One policy differentiates between the children it names: the overlay rules ride on the target, not
// on the policy, so an initiator sharing to two children on different terms needs one policy rather
// than two. A child brought with its subtree hands its own terms down; a child named alone does not.
//
// This is what the one-policy-per-organization-unit rule rests on. Were per-target rules to store
// with no target, every child of the policy would silently get whichever rule was written last.
func (s *ServiceTestSuite) TestEachNamedChildGetsItsOwnRules() {
	ctx := context.Background()
	policy, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, rootOU, PolicyRequest{
		TargetOUScope: TargetOUScope{ChildOUIDs: []TargetEntry{
			{OUID: childOU, AllChildren: true, OverlayRules: map[string]OverlayRule{
				"assignments": {Editable: true, AllowedValues: members("a", "b")},
			}},
			{OUID: nestedOwnerOU, OverlayRules: map[string]OverlayRule{
				"assignments": {Editable: false, Value: members("c")},
			}},
		}},
	})
	s.Require().Nil(svcErr)

	// Each rule is stored against the target that asked for it, not against the policy.
	byTarget := make(map[string]string, len(policy.Targets))
	for _, t := range policy.Targets {
		byTarget[t.ID] = t.OUID
	}
	governed := make(map[string]OverlayRule, len(policy.Rules))
	for _, r := range policy.Rules {
		s.Require().NotEmpty(r.TargetID, "a rule named by a target must not store as a policy-level one")
		governed[byTarget[r.TargetID]] = r.Resolved
	}
	s.Require().Len(governed, 2)
	s.True(governed[childOU].Editable)
	s.ElementsMatch([]string{"a", "b"}, *governed[childOU].AllowedValues)
	s.False(governed[nestedOwnerOU].Editable)
	s.ElementsMatch([]string{"c"}, *governed[nestedOwnerOU].Value)

	// And resolution hands each organization unit its own terms.
	for _, tc := range []struct {
		ouID     string
		editable bool
	}{
		{childOU, true},
		{grandOU, true}, // beneath childOU, reached through its subtree
		{nestedOwnerOU, false},
	} {
		resolved, svcErr := s.svc.ResolveOverlayRules(ctx, testType, testResource, tc.ouID)
		s.Require().Nil(svcErr)
		s.True(resolved.Visible, tc.ouID+" is reached by the policy")
		s.Equal(tc.editable, resolved.Rules["assignments"].Editable,
			tc.ouID+" holds the terms of the target that named it")
	}
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
	p := s.storedAllChildren(nil)

	_, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  TargetOUScope{ChildOUIDs: []TargetEntry{{OUID: childOU}}},
		Version:        p.Version,
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorScopeFamilyChange.Code, svcErr.Code, "the refusal is the change of family, both ways")
}

// Both directions of a family change are refused, and both say the same thing: what is wrong is the
// conversion, not which side of it the policy started on.
func (s *ServiceTestSuite) TestAFamilyChangeReadsTheSameWhicheverWayItGoes() {
	ctx := context.Background()

	selective := s.share(nil)
	_, widening := s.svc.UpdatePolicy(ctx, selective.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{AllChildren: true},
		Version:       selective.Version,
	})

	s.SetupTest()
	blanket := s.storedAllChildren(nil)
	_, narrowing := s.svc.UpdatePolicy(ctx, blanket.ID, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  TargetOUScope{ChildOUIDs: []TargetEntry{{OUID: childOU}}},
		Version:        blanket.Version,
	})

	s.Require().NotNil(widening)
	s.Require().NotNil(narrowing)
	s.Equal(ErrorScopeFamilyChange.Code, widening.Code)
	s.Equal(*widening, *narrowing, "one conversion rule, one answer")
	s.NotEqual(ErrorBlanketNarrowOnly.Code, widening.Code,
		"a selective policy is not refused for what blanket policies may do")
}

// Exclusions are the only thing that narrows a blanket policy, so an edit may add them.
func (s *ServiceTestSuite) TestUpdateEditsExclusionsOnABlanketPolicyBothWays() {
	ctx := context.Background()
	p := s.storedAllChildren(nil)

	added, svcErr := s.svc.UpdatePolicy(ctx, p.ID, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  TargetOUScope{AllChildren: true, ExcludedOUIDs: []string{childOU}},
		Version:        p.Version,
	})
	s.Require().Nil(svcErr)
	s.Equal([]string{childOU}, added.ExcludedOUIDs)

	// Dropping it again hands the resource back, which is the issuer reversing its own decision.
	dropped, svcErr := s.svc.UpdatePolicy(ctx, p.ID, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  TargetOUScope{AllChildren: true},
		Version:        added.Version,
	})
	s.Require().Nil(svcErr, "an exclusion may be removed as well as added")
	s.Empty(dropped.ExcludedOUIDs)

	visible, svcErr := s.svc.IsVisible(ctx, testType, testResource, childOU)
	s.Require().Nil(svcErr)
	s.True(visible, "childOU can see the resource again")
}

// Deleting one of two covering policies must not wipe state the surviving one still authorizes,
// which is why the cleanup is driven by visibility after the delete rather than by target lists.
func (s *ServiceTestSuite) TestDeleteOnlyCleansUpOUsThatActuallyLoseVisibility() {
	first := s.share(map[string]OverlayRule{"assignments": {Editable: true}})
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  TargetOUScope{ChildOUIDs: []TargetEntry{{OUID: childOU}}},
	})
	s.Require().Nil(svcErr)

	// Removing the owner's policy cuts the root off, and everything under it with it.
	s.Require().Nil(s.svc.DeletePolicy(context.Background(), first.ID))

	notified := make([]string, 0, len(s.decl.lostCalls))
	for _, c := range s.decl.lostCalls {
		notified = append(notified, c.ouID)
		s.Equal(testResource, c.resourceID, "the hook names the resource, not a field of it")
	}
	s.ElementsMatch([]string{rootOU, childOU, grandOU, greatOU, nestedOwnerOU}, notified)
}

// The candidate set is what a removed target could have reached; the hook only fires for those that
// no longer see the resource once the delete has happened.
func (s *ServiceTestSuite) TestReshareRefusedWhenCoveringPolicyReachesBelow() {
	cases := []struct {
		name  string
		setUp func()
	}{
		{
			name:  "deployment wide",
			setUp: func() { s.declaredBlanket(TargetOUScope{AllOUs: true}) },
		},
		{
			name: "a frontier unit's whole subtree",
			setUp: func() {
				s.share(nil)
				_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
					PolicyRequest{
						InitiatingOUID: rootOU,
						TargetOUScope:  TargetOUScope{AllChildren: true},
					})
				s.Require().Nil(svcErr)
			},
		},
		{
			name: "a named unit brought with its subtree",
			setUp: func() {
				s.share(nil)
				_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
					PolicyRequest{
						InitiatingOUID: rootOU,
						TargetOUScope: TargetOUScope{
							ChildOUIDs: []TargetEntry{{OUID: childOU, AllChildren: true}},
						},
					})
				s.Require().Nil(svcErr)
			},
		},
	}

	for _, tt := range cases {
		s.Run(tt.name, func() {
			s.SetupTest()
			tt.setUp()

			_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
				PolicyRequest{InitiatingOUID: childOU, TargetOUScope: entry(grandOU)})

			s.Require().NotNil(svcErr, "the covering policy already reaches grandOU")
			s.Equal(ErrorReshareNotPermitted.Code, svcErr.Code)
		})
	}
}

// Naming a child alongside allChildren used to be accepted, and the child's own overlay rule then
// stored with no target: the terms meant for that one child governed every unit in the subtree. The
// pair is refused rather than resolved, because either reading of it discards half the request.
func (s *ServiceTestSuite) TestAllChildrenWithANamedChildIsRefused() {
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, rootOU, PolicyRequest{
		TargetOUScope: TargetOUScope{
			AllChildren: true,
			ChildOUIDs: []TargetEntry{{
				OUID: childOU,
				OverlayRules: map[string]OverlayRule{
					"assignments": {Editable: true, AllowedValues: members("only-for-the-named-child")},
				},
			}},
		},
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidRequestFormat.Code, svcErr.Code)
}

// A unit the covering policy named and stopped at holds reach of its own, and may hand it on.
func (s *ServiceTestSuite) TestReshareAllowedFromAFrontierUnit() {
	s.share(nil)

	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{InitiatingOUID: rootOU, TargetOUScope: entry(childOU)})
	s.Require().Nil(svcErr, "rootOU was named by a root target, which stops at it")

	_, svcErr = s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{InitiatingOUID: childOU, TargetOUScope: entry(grandOU)})
	s.Require().Nil(svcErr, "childOU was named alone in turn")

	visible, covering, svcErr := s.svc.(*sharingService).resolveVisibility(context.Background(), testType,
		testResource, ownerOU, grandOU)
	s.Require().Nil(svcErr)
	s.True(visible)
	s.Len(covering, 1, "handing reach on one unit at a time leaves exactly one policy covering each")
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

// A declared policy owns its id, so the export has to carry it: without it the replay is refused for
// having no id, and a reshare beneath it would point at a parent that no longer exists.
func (s *ServiceTestSuite) TestExportRoundTripsADeclaredPolicyID() {
	s.declaredBlanket(TargetOUScope{AllOUs: true})

	exported, svcErr := s.svc.ExportPolicies(context.Background(), testType, testResource)
	s.Require().Nil(svcErr)
	require.Len(s.T(), exported, 1)
	s.Equal(declaredID, exported[0].Request.ID)

	replayed, svcErr := s.declarative().CreateDeclarativePolicy(context.Background(), testType,
		testResource, ownerOU, exported[0].Request)
	s.Require().Nil(svcErr, "replaying an export has to recreate the policy it came from")
	s.Equal(declaredID, replayed.ID)
}

// The file owns whether the policy exists, so an unedited declared policy cannot be deleted.
func (s *ServiceTestSuite) TestAnUneditedDeclaredPolicyCannotBeDeleted() {
	declStore := newFileBasedStore()
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(s.store, declStore, hierarchy, enumerator, inlineTx(s.T()), nil, nil, false)
	svc.RegisterResourceType(s.decl)
	ctx := context.Background()

	declared, svcErr := svc.CreateDeclarativePolicy(ctx, testType, testResource, ownerOU,
		PolicyRequest{ID: declaredID, TargetOUScope: TargetOUScope{AllOUs: true}})
	s.Require().Nil(svcErr)

	svcErr = svc.DeletePolicy(ctx, declared.ID)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorPolicyDeclared.Code, svcErr.Code)
}

// Re-loading a file replaces its policy rather than appending, which is what stops restarts
// accumulating duplicates.
func (s *ServiceTestSuite) TestDeclarativeReplayIsIdempotent() {
	declStore := newFileBasedStore()
	first := Policy{ID: "a", ResourceType: testType, ResourceID: testResource, InitiatingOUID: ownerOU}
	second := Policy{ID: "b", ResourceType: testType, ResourceID: testResource, InitiatingOUID: ownerOU}

	declStore.seed(first)
	declStore.seed(second)

	s.Len(declStore.listForResource(testType, testResource), 1)
}

// entry is a named target organization unit within a children-mode scope.
func entry(ouID string) TargetOUScope {
	return TargetOUScope{ChildOUIDs: []TargetEntry{{OUID: ouID}}}
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
			ChildOUIDs: []TargetEntry{{OUID: childOU, AllChildren: true}},
		}})

	s.Require().Nil(svcErr)
	s.Require().Len(p.Targets, 1)
	s.Equal(targetScopeOUSubtree, p.Targets[0].Scope)
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
				PolicyRequest{TargetOUScope: TargetOUScope{ChildOUIDs: tt.entries}})

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
		// declared routes the row through the declarative path. The cross-tree gate runs there
		// too, so a declared policy is still refused; it is simply the only way these two scopes
		// reach the gate at all.
		declared bool
	}{
		{"a named foreign root", TargetOUScope{RootOUIDs: []string{otherOU}}, false},
		{"every root", TargetOUScope{AllRoots: true}, true},
		{"every organization unit", TargetOUScope{AllOUs: true}, true},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			create := s.svc.CreatePolicy
			if tt.declared {
				create = s.declarative().CreateDeclarativePolicy
			}

			_, svcErr := create(context.Background(), testType, testResource,
				nestedOwnerOU, PolicyRequest{ID: declaredID, TargetOUScope: tt.scope})

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
	hierarchy, enumerator := testResolver(s.T())
	permissive := newSharingService(mustMockStore(s.T()), nil, hierarchy, enumerator,
		inlineTx(s.T()), nil, nil, true)
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
	hierarchy, enumerator := testResolver(s.T())
	declSvc := newSharingService(mustMockStore(s.T()), newFileBasedStore(), hierarchy, enumerator,
		inlineTx(s.T()), nil, nil, false)
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
				tt.owner, PolicyRequest{ID: declaredID, TargetOUScope: tt.scope})

			s.Require().NotNil(svcErr)
			s.Equal(tt.code, svcErr.Code)
		})
	}
}

// buildFrontierChain hands the resource down a branch one organization unit at a time.
//
// Every policy names the unit below it and stops there, which makes each named unit a frontier
// free to hand the resource on in turn. Each unit is covered by exactly one policy, so the bound
// at the bottom is whatever the tightest step above it allows.
func (s *ServiceTestSuite) buildFrontierChain() (upper, mid, reshare Policy) {
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

	upper, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  entry(childOU),
		OverlayRules:   inherit,
	})
	s.Require().Nil(svcErr)

	mid, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
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
		"the reshare starts at the owner's bound, inherited down the chain")
	return upper, mid, reshare
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

// Narrowing any policy on the chain has to reach the reshare beneath it, however many hops down.
func (s *ServiceTestSuite) TestUpdateRematerializesDownTheChain() {
	narrow := map[string]OverlayRule{"assignments": {Editable: true, AllowedValues: members("a")}}

	s.Run("narrowing the policy two hops up", func() {
		s.SetupTest()
		upper, _, reshare := s.buildFrontierChain()

		_, svcErr := s.svc.UpdatePolicy(context.Background(), upper.ID, PolicyRequest{
			InitiatingOUID: rootOU,
			TargetOUScope:  entry(childOU),
			OverlayRules:   narrow,
			Version:        upper.Version,
		})
		s.Require().Nil(svcErr)

		s.Equal([]string{"a"}, s.resolvedBound(reshare.ID))
	})

	s.Run("narrowing the policy one hop up", func() {
		s.SetupTest()
		_, mid, reshare := s.buildFrontierChain()

		_, svcErr := s.svc.UpdatePolicy(context.Background(), mid.ID, PolicyRequest{
			InitiatingOUID: childOU,
			TargetOUScope:  entry(grandOU),
			OverlayRules:   narrow,
			Version:        mid.Version,
		})
		s.Require().Nil(svcErr)

		s.Equal([]string{"a"}, s.resolvedBound(reshare.ID))
	})
}

// Recomputing every reshare must not rewrite the ones that did not change, or an unrelated edit
// would bump their version and fail a concurrent editor holding a perfectly current read.
func (s *ServiceTestSuite) TestUpdateLeavesUnaffectedPoliciesAtTheirVersion() {
	upper, _, reshare := s.buildFrontierChain()
	narrowed := PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  entry(childOU),
		OverlayRules:   map[string]OverlayRule{"assignments": {Editable: true, AllowedValues: members("a")}},
	}

	narrowed.Version = upper.Version
	updated, svcErr := s.svc.UpdatePolicy(context.Background(), upper.ID, narrowed)
	s.Require().Nil(svcErr)

	afterChange, svcErr := s.svc.GetPolicy(context.Background(), reshare.ID)
	s.Require().Nil(svcErr)
	s.Greater(afterChange.Version, reshare.Version, "a reshare whose ceiling moved is rewritten")

	// The same edit again resolves to the same rules, so nothing beneath it should be touched.
	narrowed.Version = updated.Version
	_, svcErr = s.svc.UpdatePolicy(context.Background(), upper.ID, narrowed)
	s.Require().Nil(svcErr)

	afterNoChange, svcErr := s.svc.GetPolicy(context.Background(), reshare.ID)
	s.Require().Nil(svcErr)
	s.Equal(afterChange.Version, afterNoChange.Version, "an unchanged reshare keeps its version")
}

// A declared policy and a stored one can only collide on id through a uuid collision, but the
// export has to stay total if they ever do: one must not silently stand in for the other.
func (s *ServiceTestSuite) TestExportKeepsBothPoliciesWhenIDsCollide() {
	declStore := newFileBasedStore()
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(mustMockStore(s.T()), declStore, hierarchy, enumerator,
		inlineTx(s.T()), nil, nil, false)
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
		Stage:          stageShare,
		Declared:       true,
		Targets:        []Target{{ID: "target-1", Scope: targetScopeRoot, OUID: otherOU}},
	})

	exported, svcErr := svc.ExportPolicies(context.Background(), testType, testResource)
	s.Require().Nil(svcErr)

	s.Require().Len(exported, 2)
	s.ElementsMatch([]string{ownerOU, otherOU},
		[]string{exported[0].InitiatingOUID, exported[1].InitiatingOUID},
		"one policy stood in for the other instead of both being exported")
}

// cacheState is the in-memory state behind a generated cache mock, along with the read and write
// counts a test needs to tell a recomputed answer from a served one. The mock supplies the
// interface; this supplies the behavior, because a read that populates the cache has to be served
// from it the next time.
type cacheState[T any] struct {
	values map[string]T
	gets   int
	sets   int
}

// newMockCache returns a generated cache mock backed by fresh state, along with that state so a
// test can inspect it. Every method is wired as Maybe, since no test needs all eight.
func newMockCache[T any](t interface {
	mock.TestingT
	Cleanup(func())
}) (*cachemock.CacheInterfaceMock[T], *cacheState[T]) {
	st := &cacheState[T]{values: map[string]T{}}
	m := cachemock.NewCacheInterfaceMock[T](t)
	e := m.EXPECT()
	a := mock.Anything
	e.GetName().Return("mock-cache").Maybe()
	e.Get(a, a).RunAndReturn(st.Get).Maybe()
	e.Set(a, a, a).RunAndReturn(st.Set).Maybe()
	e.Delete(a, a).RunAndReturn(st.Delete).Maybe()
	e.Clear(a).RunAndReturn(st.Clear).Maybe()
	e.IsEnabled().Return(true).Maybe()
	e.GetStats().Return(cache.CacheStat{}).Maybe()
	e.CleanupExpired().Return().Maybe()
	return m, st
}

// Get reads a cached value, counting the read.
func (c *cacheState[T]) Get(_ context.Context, key cache.CacheKey) (T, bool) {
	c.gets++
	v, ok := c.values[key.Key]
	return v, ok
}

// Set writes a cached value, counting the write.
func (c *cacheState[T]) Set(_ context.Context, key cache.CacheKey, value T) error {
	c.sets++
	c.values[key.Key] = value
	return nil
}

// Delete drops one cached value.
func (c *cacheState[T]) Delete(_ context.Context, key cache.CacheKey) error {
	delete(c.values, key.Key)
	return nil
}

// Clear drops every cached value.
func (c *cacheState[T]) Clear(_ context.Context) error {
	c.values = map[string]T{}
	return nil
}

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
	// blanketWith creates the owner's all-children policy, bounded to a and b.
	blanketWith := func() Policy {
		s.SetupTest()
		return s.storedAllChildren(map[string]OverlayRule{
			"assignments": {Editable: true, AllowedValues: members("a", "b")},
		})
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
				InitiatingOUID: rootOU,
				TargetOUScope:  TargetOUScope{AllChildren: true},
				OverlayRules:   tt.rules,
				Version:        p.Version,
			})

			s.Require().NotNil(svcErr, tt.claim)
			s.Equal(ErrorBlanketNarrowOnly.Code, svcErr.Code)
		})
	}

	s.Run("narrowing the bound is still allowed", func() {
		p := blanketWith()

		updated, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
			InitiatingOUID: rootOU,
			TargetOUScope:  TargetOUScope{AllChildren: true},
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
			InitiatingOUID: rootOU,
			TargetOUScope:  TargetOUScope{AllChildren: true},
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
	visibilityMock, visibility := newMockCache[bool](s.T())
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(mustMockStore(s.T()), nil, hierarchy, enumerator,
		inlineTx(s.T()), visibilityMock, nil, false)
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
	visibilityMock, visibility := newMockCache[bool](s.T())
	decl := &lateOwnerDeclaration{testDeclaration: &testDeclaration{}}
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(mustMockStore(s.T()), nil, hierarchy, enumerator, inlineTx(s.T()),
		visibilityMock, nil, false)
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
		Stage:          stageShare,
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
	p := s.storedAllChildren(map[string]OverlayRule{"assignments": {Editable: true}})

	s.Require().Nil(s.svc.DeletePolicy(context.Background(), p.ID))

	notified := make([]string, 0, len(s.decl.lostCalls))
	for _, c := range s.decl.lostCalls {
		notified = append(notified, c.ouID)
	}
	s.ElementsMatch([]string{childOU, grandOU, greatOU, nestedOwnerOU}, notified,
		"an all-children target reaches the issuer's whole subtree, the issuer itself excluded")
}

// A subtree target names its anchor, but reaches everything under it too.
func (s *ServiceTestSuite) TestDeleteCleansUpBeneathASubtreeTarget() {
	s.share(nil)
	p, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{
			InitiatingOUID: rootOU,
			TargetOUScope:  TargetOUScope{ChildOUIDs: []TargetEntry{{OUID: childOU, AllChildren: true}}},
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

// Resolution inside a write must not read the cache, or it applies the ceiling as it stood before
// the edit, and must not populate it, or a rolled-back transaction leaves its state behind.
func (s *ServiceTestSuite) TestWritesDoNotConsultTheOverlayCache() {
	overlayMock, overlay := newMockCache[ResolvedOverlay](s.T())
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(s.store, nil, hierarchy, enumerator, inlineTx(s.T()), nil, overlayMock, false)
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
	overlayMock, overlay := newMockCache[ResolvedOverlay](s.T())
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(s.store, nil, hierarchy, enumerator, inlineTx(s.T()), nil, overlayMock, false)
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

// A type declaring a hierarchical field has to resolve a separator for it. Both ways of failing to
// are refused as an internal failure rather than defaulted, because comparing such a field with no
// separator is raw string prefixing: a policy bounded to "billing" would hand over "billingx".
//
// Refusing is what makes the capability required in practice. A resource type that skipped it while
// the framework defaulted would ship a policy that quietly grants more than it names.
func (s *ServiceTestSuite) TestAHierarchicalFieldWithoutAResolvedDelimiterIsRefused() {
	for _, tc := range []struct {
		name string
		decl ResourceOverlayFieldDeclaration
	}{
		{"the capability is not implemented", &hierarchyWithoutDelimiter{}},
		{"the capability resolves an empty separator", &emptyDelimiterDeclaration{}},
	} {
		s.Run(tc.name, func() {
			hierarchy, enumerator := testResolver(s.T())
			svc := newSharingService(mustMockStore(s.T()), newFileBasedStore(), hierarchy, enumerator,
				inlineTx(s.T()), nil, nil, false)
			svc.RegisterResourceType(tc.decl)

			_, svcErr := svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
				PolicyRequest{
					TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
					OverlayRules: map[string]OverlayRule{
						"permissions": {Editable: false, Value: members("billing")},
					},
				})

			s.Require().NotNil(svcErr, "the field cannot be compared, so the policy cannot be written")
			s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
		})
	}
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

// An edit narrows reach as much as a delete does, so a unit an exclusion carves out has to be
// cleaned up exactly as one whose policy was removed.
func (s *ServiceTestSuite) TestUpdateCleansUpAfterAnExclusionRemovesReach() {
	p := s.storedAllChildren(map[string]OverlayRule{"assignments": {Editable: true}})
	s.Require().NoError(s.store.SetOverlayValue(
		context.Background(), testType, testResource, childOU, "assignments", []string{"a"}))

	// The rule is carried through unchanged: leaving it out would be a widening, which a blanket
	// policy refuses, and this test is about the exclusion rather than the rules.
	_, svcErr := s.svc.UpdatePolicy(context.Background(), p.ID, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  TargetOUScope{AllChildren: true, ExcludedOUIDs: []string{childOU}},
		OverlayRules:   map[string]OverlayRule{"assignments": {Editable: true}},
		Version:        p.Version,
	})
	s.Require().Nil(svcErr)

	notified := make([]string, 0, len(s.decl.lostCalls))
	for _, c := range s.decl.lostCalls {
		notified = append(notified, c.ouID)
	}
	s.Contains(notified, childOU, "the excluded unit lost the resource and must be cleaned up")
	s.Empty(s.store.values[string(testType)+testResource+childOU],
		"its own values for the resource go with it")
}

// A unit that goes dark cannot hold up what it shared onward, so the loss has to follow the
// reshares it issued rather than stopping at the edited policy's own targets.
func (s *ServiceTestSuite) TestCleanUpCascadesThroughReshares() {
	owner := s.share(map[string]OverlayRule{"assignments": {Editable: true}})
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{
			InitiatingOUID: rootOU,
			TargetOUScope:  TargetOUScope{ChildOUIDs: []TargetEntry{{OUID: childOU}}},
			OverlayRules:   map[string]OverlayRule{"assignments": {Editable: true}},
		})
	s.Require().Nil(svcErr)
	s.decl.lostCalls = nil

	// Removing the owner's policy takes rootOU's visibility, and childOU only ever had the
	// resource through the policy rootOU issued.
	s.Require().Nil(s.svc.DeletePolicy(context.Background(), owner.ID))

	notified := make([]string, 0, len(s.decl.lostCalls))
	for _, c := range s.decl.lostCalls {
		notified = append(notified, c.ouID)
	}
	s.Contains(notified, rootOU, "the unit the removed policy named lost the resource")
	s.Contains(notified, childOU, "and so did the unit reached only through that unit's reshare")
}

// A unit can be reached twice by one cascade: once because the removed policy's target covered it,
// and again because a reshare issued beneath that target named it. The hook is called once per unit
// all the same, which is what its contract promises a resource type. Calling it twice would run a
// type's cleanup a second time against state the first call already dropped.
func (s *ServiceTestSuite) TestAUnitReachedTwiceByOneCascadeIsNotifiedOnce() {
	ctx := context.Background()
	// The owner's target carries rootOU's whole subtree, so childOU is covered by it directly.
	owner, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules:  map[string]OverlayRule{"assignments": {Editable: true}},
	})
	s.Require().Nil(svcErr)
	// And rootOU names childOU again in a reshare of its own, so the cascade reaches it a second
	// time when rootOU goes dark.
	_, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  TargetOUScope{ChildOUIDs: []TargetEntry{{OUID: childOU, AllChildren: true}}},
		OverlayRules:   map[string]OverlayRule{"assignments": {Editable: true}},
	})
	s.Require().Nil(svcErr)
	s.decl.lostCalls = nil

	s.Require().Nil(s.svc.DeletePolicy(ctx, owner.ID))

	perUnit := map[string]int{}
	for _, c := range s.decl.lostCalls {
		perUnit[c.ouID]++
	}
	s.Require().Contains(perUnit, childOU, "childOU lost the resource by both routes")
	for ouID, calls := range perUnit {
		s.Equal(1, calls, "the hook fires once per unit, and "+ouID+" was notified "+
			"more than once")
	}
}

// A resource type that stores its units' values elsewhere is called instead of the framework's own
// table, since the framework holds nothing for it to delete.
func (s *ServiceTestSuite) TestOverlayCleanupIsOverridablePerResourceType() {
	storeMock, store := newMockStore(s.T())
	decl := &cleanerDeclaration{}
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(storeMock, nil, hierarchy, enumerator, inlineTx(s.T()), nil, nil, false)
	svc.RegisterResourceType(decl)

	p, svcErr := svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}}})
	s.Require().Nil(svcErr)
	s.Require().NoError(store.SetOverlayValue(
		context.Background(), testType, testResource, rootOU, "assignments", []string{"a"}))

	s.Require().Nil(svc.DeletePolicy(context.Background(), p.ID))

	s.Contains(decl.cleaned, rootOU, "the type's own cleaner was called for the unit that lost it")
	s.NotEmpty(store.values[string(testType)+testResource+rootOU],
		"and the framework left its own table alone, since this type does not use it")
}

// ownerlessDeclaration is a resource type that never implements OwnerResolver, which the framework
// cannot work without: ownership is asked of the type and read nowhere else.
type ownerlessDeclaration struct {
	fields []FieldDeclaration
}

func (d *ownerlessDeclaration) ResourceType() ResourceType { return testType }
func (d *ownerlessDeclaration) Fields() []FieldDeclaration { return d.fields }

// A type that cannot answer who owns a resource is refused at registration rather than onboarded.
//
// Onboarding it would answer its owner with a denial: the chain-scoped fetch returns the policies
// bearing on the asking organization unit, and an owner's own policies point at other units, so the
// owner's question fetches nothing and nothing else knows who the owner is. Refusing is louder,
// because the first call then says the type is not registered.
func (s *ServiceTestSuite) TestATypeThatCannotResolveOwnershipIsNotRegistered() {
	ctx := context.Background()
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(mustMockStore(s.T()), newFileBasedStore(), hierarchy, enumerator,
		inlineTx(s.T()), nil, nil, false)

	svc.RegisterResourceType(&ownerlessDeclaration{})

	_, svcErr := svc.CreatePolicy(ctx, testType, testResource, rootOU, PolicyRequest{
		TargetOUScope: entry(childOU),
	})
	s.Require().NotNil(svcErr, "the type never registered, so nothing can be shared for it")
	s.Equal(ErrorResourceTypeNotRegistered.Code, svcErr.Code)
}

// The overlay cache must not hand the owner a denial that was computed while the resource type
// could not yet say who the owner was. Only a policy write clears that cache, so the owner would be
// told it holds nothing about its own resource for as long as the entry survived.
func (s *ServiceTestSuite) TestResolveOverlayRulesNeverServesTheOwnerAStaleDenial() {
	decl := &unresolvedOwnerDeclaration{}
	hierarchy, enumerator := testResolver(s.T())
	overlayMock, overlay := newMockCache[ResolvedOverlay](s.T())
	svc := newSharingService(mustMockStore(s.T()), nil, hierarchy, enumerator, inlineTx(s.T()), nil, overlayMock, false)
	svc.RegisterResourceType(decl)

	// Ownership is unresolvable, so the owner's own answer is a denial, and it lands in the cache.
	first, svcErr := svc.ResolveOverlayRules(context.Background(), testType, testResource, ownerOU)
	s.Require().Nil(svcErr)
	s.Require().False(first.Visible)
	s.Require().NotEmpty(overlay.values, "the denial was cached under the owner's own key")

	decl.resolvable = true

	second, svcErr := svc.ResolveOverlayRules(context.Background(), testType, testResource, ownerOU)
	s.Require().Nil(svcErr)
	s.True(second.Owned, "the owner is recognized once the resource type can answer")
	s.True(second.Visible, "and is not served the denial cached while ownership was unknown")
}

// Everyone else keeps their cached answer, because it never depended on ownership being resolvable.
func (s *ServiceTestSuite) TestResolveOverlayRulesStillServesTheCacheToNonOwners() {
	hierarchy, enumerator := testResolver(s.T())
	overlayMock, overlay := newMockCache[ResolvedOverlay](s.T())
	svc := newSharingService(s.store, nil, hierarchy, enumerator, inlineTx(s.T()), nil, overlayMock, false)
	svc.RegisterResourceType(s.decl)

	_, svcErr := svc.ResolveOverlayRules(context.Background(), testType, testResource, childOU)
	s.Require().Nil(svcErr)
	setsAfterFirst := overlay.sets

	_, svcErr = svc.ResolveOverlayRules(context.Background(), testType, testResource, childOU)
	s.Require().Nil(svcErr)

	s.Equal(setsAfterFirst, overlay.sets, "the second read was served from the cache")
}

// A reshare that carries no overlay rules must come out of re-materialization untouched. Rewriting
// it would bump a version nothing changed, and the next concurrent editor holding a current read of
// that reshare would fail on a mismatch it had no part in.
func (s *ServiceTestSuite) TestRematerializeLeavesARuleLessReshareAlone() {
	owner := s.share(map[string]OverlayRule{"assignments": {Editable: true}})

	reshare, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{
			InitiatingOUID: rootOU,
			TargetOUScope:  TargetOUScope{ChildOUIDs: []TargetEntry{{OUID: childOU}}},
		})
	s.Require().Nil(svcErr)
	s.Require().Empty(reshare.Rules, "this reshare carries no overlay rules at all")
	versionBefore := s.store.policies[reshare.ID].Version

	// Edit the owner's policy, which re-materializes every reshare of the resource.
	_, svcErr = s.svc.UpdatePolicy(context.Background(), owner.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules:  map[string]OverlayRule{"assignments": {Editable: true}},
		Version:       owner.Version,
	})
	s.Require().Nil(svcErr)

	s.Equal(versionBefore, s.store.policies[reshare.ID].Version,
		"a reshare with nothing to rebuild must not be rewritten")
}

// An exclusion only means something inside the reach of the policy carrying it. Naming a unit no
// target reaches is refused rather than stored, so it cannot read as a withholding that never
// happened.
func (s *ServiceTestSuite) TestExclusionMustBeInThePolicysOwnReach() {
	cases := []struct {
		name    string
		scope   TargetOUScope
		refused bool
	}{
		{
			name: "a descendant of a subtree target",
			scope: TargetOUScope{
				ChildOUIDs:    []TargetEntry{{OUID: childOU, AllChildren: true}},
				ExcludedOUIDs: []string{grandOU},
			},
		},
		{
			name: "the named unit of a subtree target",
			scope: TargetOUScope{
				ChildOUIDs:    []TargetEntry{{OUID: childOU, AllChildren: true}},
				ExcludedOUIDs: []string{childOU},
			},
		},
		{
			name:  "a descendant, under an all children target",
			scope: TargetOUScope{AllChildren: true, ExcludedOUIDs: []string{grandOU}},
		},
		{
			name: "a grandchild, where the target stops at the child",
			scope: TargetOUScope{
				ChildOUIDs:    []TargetEntry{{OUID: childOU}},
				ExcludedOUIDs: []string{grandOU},
			},
			refused: true,
		},
		{
			name: "a unit in another tree",
			scope: TargetOUScope{
				ChildOUIDs:    []TargetEntry{{OUID: childOU, AllChildren: true}},
				ExcludedOUIDs: []string{otherOU},
			},
			refused: true,
		},
	}

	for _, tt := range cases {
		s.Run(tt.name, func() {
			s.SetupTest()
			s.share(nil)

			_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
				PolicyRequest{InitiatingOUID: rootOU, TargetOUScope: tt.scope})

			if !tt.refused {
				s.Require().Nil(svcErr)
				return
			}
			s.Require().NotNil(svcErr)
			s.Equal(ErrorExclusionOutOfReach.Code, svcErr.Code)
		})
	}
}

// A deployment-wide policy reaches everything, so any unit at all is a valid thing for it to carve out.
func (s *ServiceTestSuite) TestDeploymentWidePolicyMayExcludeAnyUnit() {
	_, svcErr := s.declarative().CreateDeclarativePolicy(context.Background(), testType, testResource,
		ownerOU, PolicyRequest{
			ID:            declaredID,
			TargetOUScope: TargetOUScope{AllOUs: true, ExcludedOUIDs: []string{otherOU, grandOU}},
		})

	s.Require().Nil(svcErr)
}

// A unit that has handed the resource on cannot have the reach above it widened over its head:
// everything it granted would then be covered twice.
func (s *ServiceTestSuite) TestEditCannotReachPastAUnitThatSharedOn() {
	ctx := context.Background()
	s.share(nil)

	upper, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU,
		PolicyRequest{InitiatingOUID: rootOU, TargetOUScope: entry(childOU)})
	s.Require().Nil(svcErr)

	_, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, ownerOU,
		PolicyRequest{InitiatingOUID: childOU, TargetOUScope: entry(grandOU)})
	s.Require().Nil(svcErr)

	_, svcErr = s.svc.UpdatePolicy(ctx, upper.ID, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  TargetOUScope{ChildOUIDs: []TargetEntry{{OUID: childOU, AllChildren: true}}},
		Version:        upper.Version,
	})

	s.Require().NotNil(svcErr, "childOU has already shared the resource on")
	s.Equal(ErrorReshareNotPermitted.Code, svcErr.Code)

	_, covering, svcErr := s.svc.(*sharingService).resolveVisibility(ctx, testType, testResource, ownerOU, grandOU)
	s.Require().Nil(svcErr)
	s.Len(covering, 1, "the refusal is what keeps exactly one policy covering each unit")
}

// The same widening is fine while nothing below has been handed on.
func (s *ServiceTestSuite) TestEditMayWidenWhenNothingWasSharedOn() {
	ctx := context.Background()
	s.share(nil)

	upper, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU,
		PolicyRequest{InitiatingOUID: rootOU, TargetOUScope: entry(childOU)})
	s.Require().Nil(svcErr)

	_, svcErr = s.svc.UpdatePolicy(ctx, upper.ID, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  TargetOUScope{ChildOUIDs: []TargetEntry{{OUID: childOU, AllChildren: true}}},
		Version:        upper.Version,
	})

	s.Require().Nil(svcErr)
}

// memberValidatingDeclaration is a resource type that vets the members a rule names, refusing the
// ones in refuse with whatever reason it is given for them.
type memberValidatingDeclaration struct {
	testDeclaration
	refuse map[string]*tidcommon.ServiceError
}

func (d *memberValidatingDeclaration) ValidateMembers(
	_ context.Context, _, _, _ string, members []string,
) *tidcommon.ServiceError {
	for _, m := range members {
		if svcErr, ok := d.refuse[m]; ok {
			return svcErr
		}
	}
	return nil
}

// memberRefusal is a resource type's own client-side rejection of a member, which is the only error
// class that means the members were actually refused.
func memberRefusal(reason string) *tidcommon.ServiceError {
	return &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "TST-0001",
		Error: tidcommon.I18nMessage{DefaultValue: reason},
	}
}

// A refusal must not say why. "No such member" and "that member is not yours to name" have to be
// indistinguishable, or a caller can enumerate what exists in OUs it cannot see by reading the
// difference. The member itself is never echoed back either.
func (s *ServiceTestSuite) TestMemberRefusalRevealsNothingAboutTheMember() {
	decl := &memberValidatingDeclaration{refuse: map[string]*tidcommon.ServiceError{
		"ghost":  memberRefusal("no permission with that name exists"),
		"hidden": memberRefusal("permission exists but belongs to another organization unit"),
	}}
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(mustMockStore(s.T()), newFileBasedStore(), hierarchy, enumerator,
		inlineTx(s.T()), nil, nil, false)
	svc.RegisterResourceType(decl)

	refuse := func(member string) *tidcommon.ServiceError {
		_, svcErr := svc.CreatePolicy(context.Background(), testType, testResource+member, ownerOU,
			PolicyRequest{
				TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
				OverlayRules: map[string]OverlayRule{
					"assignments": {Editable: true, AllowedValues: members(member)},
				},
			})
		s.Require().NotNil(svcErr, member+" is refused by the resource type")
		return svcErr
	}

	ghost, hidden := refuse("ghost"), refuse("hidden")

	s.Equal(ErrorMemberNotVisible.Code, ghost.Code)
	s.Equal(*ghost, *hidden, "the two refusals must be byte-identical")

	for _, svcErr := range []*tidcommon.ServiceError{ghost, hidden} {
		body := svcErr.Error.DefaultValue + " " + svcErr.ErrorDescription.DefaultValue
		s.NotContains(body, "ghost", "the refusal must not echo the member back")
		s.NotContains(body, "hidden")
		s.NotContains(body, "exists", "the refusal must not hint at existence either way")
		s.Contains(body, "assignments", "naming the field is what makes the error actionable")
	}
}

// A validator that could not run has refused nothing. Reporting its failure as "member not visible"
// would send the caller to fix a request that was fine, and hide a broken dependency behind a 400.
func (s *ServiceTestSuite) TestAFailedMemberCheckIsNotReportedAsARefusal() {
	decl := &memberValidatingDeclaration{refuse: map[string]*tidcommon.ServiceError{
		"unreachable": &tidcommon.InternalServerError,
	}}
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(mustMockStore(s.T()), newFileBasedStore(), hierarchy, enumerator,
		inlineTx(s.T()), nil, nil, false)
	svc.RegisterResourceType(decl)

	_, svcErr := svc.CreatePolicy(context.Background(), testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules: map[string]OverlayRule{
			"assignments": {Editable: true, AllowedValues: members("unreachable")},
		},
	})

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.NotEqual(ErrorMemberNotVisible.Code, svcErr.Code)
}

// Deleting a policy cascades to the reshares beneath it, so cleanup has to know what those reshares
// reached. A policy naming one organization unit reaches only that unit, so the units below it are
// known only from the reshares, and reading the policy list after the delete sees none of them.
func (s *ServiceTestSuite) TestDeleteCleansUpOUsReachedOnlyThroughACascadedReshare() {
	ctx := context.Background()
	// rootOU owns the resource, so its policy can name childOU alone: reach stops at childOU.
	owner, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, rootOU,
		PolicyRequest{TargetOUScope: entry(childOU)})
	s.Require().Nil(svcErr)
	_, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, rootOU,
		PolicyRequest{InitiatingOUID: childOU, TargetOUScope: entry(grandOU)})
	s.Require().Nil(svcErr)

	// grandOU holds a value of its own, which is what cleanup exists to remove.
	s.Require().NoError(s.store.SetOverlayValue(ctx, testType, testResource, grandOU, "assignments",
		[]string{"a"}))

	s.Require().Nil(s.svc.DeletePolicy(ctx, owner.ID))

	visible, _, svcErr := s.svc.(*sharingService).resolveVisibility(ctx, testType, testResource, rootOU, grandOU)
	s.Require().Nil(svcErr)
	s.Require().False(visible, "the chain above grandOU is gone, so it cannot see the resource")

	values, err := s.store.GetOverlayValues(ctx, testType, testResource, grandOU)
	s.Require().NoError(err)
	s.Empty(values, "grandOU's values must be cleared, not left behind a policy that no longer exists")

	cleaned := make([]string, 0, len(s.decl.lostCalls))
	for _, c := range s.decl.lostCalls {
		cleaned = append(cleaned, c.ouID)
	}
	s.Contains(cleaned, grandOU, "the type's cleanup hook has to fire for grandOU too")
}

// A field the policy never named can still hold state: the type declares an editable default, so an
// organization unit may write the field with no policy mentioning it. Cleanup has to cover that too,
// which is why the hook names the resource rather than a list of fields derived from the policy.
func (s *ServiceTestSuite) TestCleanupCoversFieldsNoPolicyNamed() {
	ctx := context.Background()
	// The owner's policy names one field only. "permissions" is declared with a default and is
	// never mentioned by any policy here.
	owner := s.share(map[string]OverlayRule{"assignments": {Editable: true}})

	s.Require().NoError(s.store.SetOverlayValue(ctx, testType, testResource, rootOU, "assignments",
		[]string{"a"}))
	s.Require().NoError(s.store.SetOverlayValue(ctx, testType, testResource, rootOU, "permissions",
		[]string{"billing"}))

	s.Require().Nil(s.svc.DeletePolicy(ctx, owner.ID))

	values, err := s.store.GetOverlayValues(ctx, testType, testResource, rootOU)
	s.Require().NoError(err)
	s.Empty(values, "every field goes, including the one no policy named")

	cleaned := make([]string, 0, len(s.decl.lostCalls))
	for _, c := range s.decl.lostCalls {
		cleaned = append(cleaned, c.ouID)
		s.Equal(testResource, c.resourceID,
			"the hook is told the resource, so the type clears whatever it holds for it")
	}
	s.Contains(cleaned, rootOU)
}

// Evaluation must read through the unbounded method. Routing it at the paged one would answer a
// coverage question from a page, which reads as a resource an organization unit has lost access to
// rather than as a truncated list.
func (s *ServiceTestSuite) TestEvaluationNeverReadsThroughThePagedListing() {
	ctx := context.Background()
	owner := s.share(nil)

	// Every path that asks a coverage question: the create's frontier check, the edit's, the
	// re-materialization an edit and a delete each trigger, and a plain visibility read.
	reshare, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU,
		PolicyRequest{InitiatingOUID: rootOU, TargetOUScope: entry(childOU)})
	s.Require().Nil(svcErr)
	visible, svcErr := s.svc.IsVisible(ctx, testType, testResource, childOU)
	s.Require().Nil(svcErr)
	s.True(visible)
	_, svcErr = s.svc.UpdatePolicy(ctx, owner.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}}, Version: owner.Version,
	})
	s.Require().Nil(svcErr)
	s.Require().Nil(s.svc.DeletePolicy(ctx, reshare.ID))

	s.storeMock.AssertNotCalled(s.T(), "ListPoliciesForResource",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A management API serves a page at a time, so the listing is the one read in the framework that
// may answer with part of a resource's policies. These cover what a page has to get right; every
// evaluation read goes through ListPolicies and is asserted on elsewhere.

// Paging walks the whole set once: each page carries its own slice, the total stays the resource's
// total rather than the page's, and no policy is dropped or repeated across the boundary.
func (s *ServiceTestSuite) TestPagingWalksEveryPolicyExactlyOnce() {
	ctx := context.Background()
	// Three policies, because one organization unit holds only one: the owner reaches rootOU, which
	// reshares to childOU, which reshares in turn.
	s.share(nil)
	for _, reshare := range []struct{ initiator, target string }{
		{rootOU, childOU},
		{childOU, grandOU},
	} {
		_, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
			InitiatingOUID: reshare.initiator, TargetOUScope: entry(reshare.target),
		})
		s.Require().Nil(svcErr)
	}

	seen := make([]string, 0, 3)
	for offset := 0; offset < 3; offset += 2 {
		page, svcErr := s.svc.GetPolicyList(ctx, testType, testResource, 2, offset)
		s.Require().Nil(svcErr)
		s.Equal(3, page.TotalResults, "the total counts the resource's policies, not the page's")
		s.Equal(offset+1, page.StartIndex)
		s.Equal(len(page.Policies), page.Count)
		for _, p := range page.Policies {
			seen = append(seen, p.ID)
		}
	}

	s.Len(seen, 3)
	s.Len(slices.Compact(slices.Sorted(slices.Values(seen))), 3, "no policy is returned twice")
}

// A page past the end is empty rather than an error: a listing whose last page was deleted between
// two requests is a race, not a bad request.
func (s *ServiceTestSuite) TestAPageBeyondTheEndIsEmpty() {
	s.share(nil)

	page, svcErr := s.svc.GetPolicyList(context.Background(), testType, testResource, 10, 50)

	s.Require().Nil(svcErr)
	s.Empty(page.Policies)
	s.Equal(1, page.TotalResults, "the resource still has its policy; this page just starts past it")
	s.Equal(0, page.Count)
}

// A resource nobody has shared lists as empty, not as missing. The framework stores no resource
// rows, so it cannot tell a resource with no policies from one that does not exist, and answering
// "not found" would be a claim it has no basis for.
func (s *ServiceTestSuite) TestAResourceWithNoPoliciesListsEmpty() {
	page, svcErr := s.svc.GetPolicyList(context.Background(), testType, "never-shared", 10, 0)

	s.Require().Nil(svcErr)
	s.Equal(0, page.TotalResults)
	s.Empty(page.Policies)
	s.Equal(1, page.StartIndex, "the first index is one even when there is nothing at it")
}

// The page parameters are checked before the store is asked, so a bad request is a client error
// rather than a query.
func (s *ServiceTestSuite) TestAnUnusablePageIsRefused() {
	tests := []struct {
		name          string
		limit, offset int
		expected      string
	}{
		{"a page of nothing", 0, 0, ErrorInvalidLimit.Code},
		{"a negative page", -1, 0, ErrorInvalidLimit.Code},
		{"a page larger than the maximum", serverconst.MaxPageSize + 1, 0, ErrorInvalidLimit.Code},
		{"starting before the first result", 10, -1, ErrorInvalidOffset.Code},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			_, svcErr := s.svc.GetPolicyList(context.Background(), testType, testResource, tt.limit, tt.offset)

			s.Require().NotNil(svcErr)
			s.Equal(tt.expected, svcErr.Code)
			s.Equal(tidcommon.ClientErrorType, svcErr.Type)
		})
	}
}

// A resource with more policies than the two stores can be merged across is the caller's problem,
// not the operator's: the answer names the limit so the listing can be narrowed, rather than
// reporting an internal failure nobody can act on.
func (s *ServiceTestSuite) TestTheMergeLimitIsReportedAsSuch() {
	s.share(nil)
	s.store.failNext = errResultLimitExceededInCompositeMode

	_, svcErr := s.svc.GetPolicyList(context.Background(), testType, testResource, 10, 0)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorResultLimitExceededInCompositeMode.Code, svcErr.Code)
	s.Equal(tidcommon.ClientErrorType, svcErr.Type)
}

// The count can succeed and the page read still fail, so that half has its own answer. Returning an
// empty page there would read as a resource nobody shares, while the total above it said otherwise.
func (s *ServiceTestSuite) TestAFailedPageReadIsReportedSeparatelyFromTheCount() {
	hierarchy, enumerator := testResolver(s.T())
	storeMock := newSharingPolicyStoreInterfaceMock(s.T())
	storeMock.EXPECT().CountPoliciesForResource(mock.Anything, testType, testResource).
		Return(3, nil).Once()
	storeMock.EXPECT().
		ListPoliciesForResource(mock.Anything, testType, testResource, mock.Anything, mock.Anything).
		Return(nil, errors.New("connection refused")).Once()
	svc := newSharingService(storeMock, nil, hierarchy, enumerator, inlineTx(s.T()), nil, nil, false)
	svc.RegisterResourceType(&testDeclaration{})

	_, svcErr := svc.GetPolicyList(context.Background(), testType, testResource, 10, 0)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// The unbounded read every evaluation depends on fails loudly too. Answering with no policies would
// resolve as a resource nobody can see, which is an outage wearing the shape of an empty list.
func (s *ServiceTestSuite) TestAFailedWholeSetReadIsNotAnEmptyList() {
	s.share(nil)
	s.store.failNext = errors.New("connection refused")

	_, svcErr := s.svc.ListPolicies(context.Background(), testType, testResource)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// A store that cannot be read is an internal failure, not an empty page. A listing that answered
// with nothing here would read as a resource nobody shares.
func (s *ServiceTestSuite) TestAFailedListingIsNotAnEmptyPage() {
	s.share(nil)
	s.store.failNext = errors.New("connection refused")

	_, svcErr := s.svc.GetPolicyList(context.Background(), testType, testResource, 10, 0)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// An owner tightening its own resource must succeed even when a reshare below asked for more than
// the new ceiling allows. The reshare is cut back to fit; refusing the edit would let a sharee's old
// request veto the owner, which AC9.4 does not permit.
func (s *ServiceTestSuite) TestOwnerMayTightenPastAReshareThatAskedForMore() {
	ctx := context.Background()
	owner := s.share(map[string]OverlayRule{
		"assignments": {Editable: true, AllowedValues: members("a", "b", "c")},
	})
	reshare, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  entry(childOU),
		OverlayRules:   map[string]OverlayRule{"assignments": {Editable: true, AllowedValues: members("b", "c")}},
	})
	s.Require().Nil(svcErr)
	s.Require().Equal([]string{"b", "c"}, s.resolvedBound(reshare.ID))

	updated, svcErr := s.svc.UpdatePolicy(ctx, owner.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules:  map[string]OverlayRule{"assignments": {Editable: true, AllowedValues: members("a", "b")}},
		Version:       owner.Version,
	})
	s.Require().Nil(svcErr, "the owner narrowing its own resource must not be refused")
	s.Equal([]string{"b"}, s.resolvedBound(reshare.ID),
		"the reshare keeps only what both the old request and the new ceiling allow")

	// Widening back restores what the clamp took, which is why the request is stored alongside it.
	_, svcErr = s.svc.UpdatePolicy(ctx, owner.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules:  map[string]OverlayRule{"assignments": {Editable: true, AllowedValues: members("a", "b", "c")}},
		Version:       updated.Version,
	})
	s.Require().Nil(svcErr)
	s.Equal([]string{"b", "c"}, s.resolvedBound(reshare.ID),
		"the original request is replayed, so widening restores rather than ratcheting down")
}

// A declared policy is its file. Editing it through the API is refused, so it never acquires a
// stored row, and what an organization unit it reached may do instead is issue a policy of its own.
func (s *ServiceTestSuite) TestADeclaredPolicyCannotBeEditedThroughTheAPI() {
	ctx := context.Background()
	declared := s.declaredBlanket(TargetOUScope{AllOUs: true, ExcludedOUIDs: []string{childOU}})

	_, svcErr := s.svc.UpdatePolicy(ctx, declared.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{AllOUs: true},
		Version:       declared.Version,
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorPolicyDeclared.Code, svcErr.Code)

	stored, err := s.store.ListAllPoliciesForResource(ctx, testType, testResource)
	s.Require().NoError(err)
	s.Empty(stored, "a refused edit must not have written the policy to the database")
}

// Re-applying a declaration replaces what it declared, which is how an exclusion is removed from a
// blanket policy: the file is the source of truth, so the same policy comes back without it.
func (s *ServiceTestSuite) TestReapplyingADeclarationDropsAnExclusion() {
	ctx := context.Background()
	s.declaredBlanket(TargetOUScope{AllOUs: true, ExcludedOUIDs: []string{childOU}})

	visible, svcErr := s.svc.IsVisible(ctx, testType, testResource, childOU)
	s.Require().Nil(svcErr)
	s.Require().False(visible, "childOU is excluded to begin with")

	again, svcErr := s.svc.CreateDeclarativePolicy(ctx, testType, testResource, ownerOU,
		PolicyRequest{ID: declaredID, TargetOUScope: TargetOUScope{AllOUs: true}})
	s.Require().Nil(svcErr, "re-applying the file must replace what it declared, not collide with it")
	s.Empty(again.ExcludedOUIDs)

	visible, svcErr = s.svc.IsVisible(ctx, testType, testResource, childOU)
	s.Require().Nil(svcErr)
	s.True(visible, "dropping the exclusion hands the resource back to childOU")

	stored, err := s.store.ListAllPoliciesForResource(ctx, testType, testResource)
	s.Require().NoError(err)
	s.Empty(stored, "a declared policy never reaches the database")
}

// A declaration carries its own id, so a reshare beneath it records which policy it depends on.
// An id minted here would differ on every startup and the reference would be stale immediately.
func (s *ServiceTestSuite) TestAReshareRecordsADeclaredParent() {
	ctx := context.Background()
	// rootOU owns the resource and its declaration names childOU alone, which makes childOU a
	// frontier free to reshare.
	declared, svcErr := s.svc.CreateDeclarativePolicy(ctx, testType, testResource, rootOU,
		PolicyRequest{ID: declaredID, TargetOUScope: entry(childOU)})
	s.Require().Nil(svcErr)
	s.Require().Equal(declaredID, declared.ID, "the declaration's own id is what the policy carries")

	reshare, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, rootOU,
		PolicyRequest{InitiatingOUID: childOU, TargetOUScope: entry(grandOU)})
	s.Require().Nil(svcErr)

	s.Equal(declaredID, reshare.ParentPolicyID,
		"a declared parent is recorded like any other, as a plain id rather than a foreign key")
}

// The id has to come from the file, because nothing else survives a restart.
func (s *ServiceTestSuite) TestADeclarationWithoutAnIDIsRefused() {
	_, svcErr := s.svc.CreateDeclarativePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{AllOUs: true}})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorDeclaredPolicyIDRequired.Code, svcErr.Code)
}

// Two declarations sharing one id would re-parent each other's reshares.
func (s *ServiceTestSuite) TestTwoDeclarationsCannotShareAnID() {
	ctx := context.Background()
	s.declaredBlanket(TargetOUScope{AllOUs: true})

	_, svcErr := s.svc.CreateDeclarativePolicy(ctx, testType, "another-resource", ownerOU,
		PolicyRequest{ID: declaredID, TargetOUScope: TargetOUScope{AllOUs: true}})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorDeclaredPolicyIDConflict.Code, svcErr.Code)
}

// Re-applying the same declaration keeps its id, which is what makes it an upsert rather than a
// conflict, and what keeps the reshares beneath it pointing at the same policy.
func (s *ServiceTestSuite) TestReapplyingADeclarationKeepsItsID() {
	ctx := context.Background()
	first := s.declaredBlanket(TargetOUScope{AllOUs: true, ExcludedOUIDs: []string{childOU}})

	again, svcErr := s.svc.CreateDeclarativePolicy(ctx, testType, testResource, ownerOU,
		PolicyRequest{ID: declaredID, TargetOUScope: TargetOUScope{AllOUs: true}})

	s.Require().Nil(svcErr)
	s.Equal(first.ID, again.ID)
}

// With no foreign key there is no cascade, so the service deletes the policies a unit issued once
// that unit can no longer see the resource. Leaving them would strand rows pointing at nothing.
func (s *ServiceTestSuite) TestDeleteRemovesTheReshareRowsBeneathIt() {
	ctx := context.Background()
	owner, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, rootOU,
		PolicyRequest{TargetOUScope: entry(childOU)})
	s.Require().Nil(svcErr)
	reshare, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, rootOU,
		PolicyRequest{InitiatingOUID: childOU, TargetOUScope: entry(grandOU)})
	s.Require().Nil(svcErr)

	s.Require().Nil(s.svc.DeletePolicy(ctx, owner.ID))

	remaining, err := s.store.ListAllPoliciesForResource(ctx, testType, testResource)
	s.Require().NoError(err)
	s.Empty(remaining, "the reshare beneath the deleted policy has to go with it")
	_, err = s.store.GetPolicy(ctx, reshare.ID)
	s.ErrorIs(err, errPolicyNotFound)
}

// A declaration can disappear from its resource file, and no delete runs for that: a file that
// stopped mentioning a policy looks exactly like one that never did. The reshares beneath it are
// left pointing at a parent that is not there, and a grant nobody made must not still grant.
func (s *ServiceTestSuite) TestAnOrphanedReshareGrantsNothing() {
	ctx := context.Background()
	// rootOU's file declares a policy naming childOU, and childOU reshares to grandOU beneath it.
	_, svcErr := s.svc.CreateDeclarativePolicy(ctx, testType, testResource, rootOU,
		PolicyRequest{ID: declaredID, TargetOUScope: entry(childOU)})
	s.Require().Nil(svcErr)
	_, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, rootOU,
		PolicyRequest{InitiatingOUID: childOU, TargetOUScope: entry(grandOU)})
	s.Require().Nil(svcErr)

	visible, svcErr := s.svc.IsVisible(ctx, testType, testResource, grandOU)
	s.Require().Nil(svcErr)
	s.Require().True(visible, "grandOU sees the resource while the declaration stands")

	// The operator removes the declaration from the file, so the next startup never seeds it.
	s.declStore.policies = nil

	visible, svcErr = s.svc.IsVisible(ctx, testType, testResource, grandOU)
	s.Require().Nil(svcErr)
	s.False(visible, "the reshare's parent is gone, so the reshare grants nothing")

	visible, svcErr = s.svc.IsVisible(ctx, testType, testResource, childOU)
	s.Require().Nil(svcErr)
	s.False(visible, "childOU only ever saw it through the declaration")

	ids, svcErr := s.svc.ListVisibleResourceIDs(ctx, testType, grandOU)
	s.Require().Nil(svcErr)
	s.NotContains(ids, testResource, "the reverse lookup has to agree with the forward one")
}

// An organization unit holding only an orphaned reshare cannot pass the resource on: it cannot see
// the resource itself, so there is nothing for it to share.
func (s *ServiceTestSuite) TestAnOrphanedReshareCannotShareOnward() {
	ctx := context.Background()
	_, svcErr := s.svc.CreateDeclarativePolicy(ctx, testType, testResource, rootOU,
		PolicyRequest{ID: declaredID, TargetOUScope: entry(childOU)})
	s.Require().Nil(svcErr)
	_, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, rootOU,
		PolicyRequest{InitiatingOUID: childOU, TargetOUScope: entry(grandOU)})
	s.Require().Nil(svcErr)

	s.declStore.policies = nil

	_, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, rootOU,
		PolicyRequest{InitiatingOUID: grandOU, TargetOUScope: entry(greatOU)})

	s.Require().NotNil(svcErr, "grandOU's own visibility came through an orphan")
	s.Equal(ErrorNotShared.Code, svcErr.Code)
}

// Putting the declaration back restores the branch, which is why an orphan is ignored rather than
// deleted: the reshares beneath it were never wrong, only unsupported.
func (s *ServiceTestSuite) TestRestoringADeclarationRevivesTheBranch() {
	ctx := context.Background()
	declared := PolicyRequest{ID: declaredID, TargetOUScope: entry(childOU)}
	_, svcErr := s.svc.CreateDeclarativePolicy(ctx, testType, testResource, rootOU, declared)
	s.Require().Nil(svcErr)
	_, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, rootOU,
		PolicyRequest{InitiatingOUID: childOU, TargetOUScope: entry(grandOU)})
	s.Require().Nil(svcErr)

	s.declStore.policies = nil
	visible, svcErr := s.svc.IsVisible(ctx, testType, testResource, grandOU)
	s.Require().Nil(svcErr)
	s.Require().False(visible)

	// The same declaration, carrying the same id, is loaded again.
	_, svcErr = s.svc.CreateDeclarativePolicy(ctx, testType, testResource, rootOU, declared)
	s.Require().Nil(svcErr)

	visible, svcErr = s.svc.IsVisible(ctx, testType, testResource, grandOU)
	s.Require().Nil(svcErr)
	s.True(visible, "the branch comes back, because the reshare still points at the same id")
}

// A field that declares a coarser one is governed by it when no policy names the field itself.
// Otherwise locking the coarse field would leave the finer one answered by its own default, and
// naming the finer field would be a way around the lock.
func (s *ServiceTestSuite) TestALockedCoarseFieldLocksTheFieldThatFallsBackToIt() {
	ctx := context.Background()
	_, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules:  map[string]OverlayRule{"assignments": {Editable: false}},
	})
	s.Require().Nil(svcErr)

	eff, svcErr := s.svc.ResolveOverlayRules(ctx, testType, testResource, rootOU)
	s.Require().Nil(svcErr)

	s.False(eff.Rules["assignments"].Editable)
	s.False(eff.Rules["assignments.user"].Editable,
		"assignments.user declares assignments as its fallback, so the lock reaches it")
	s.Equal(SourcePolicy, eff.Sources["assignments.user"],
		"a policy decided it, through the coarser field, so it is not the declared default")
}

// A field naming its own rule is not overridden by the coarser one: the fallback applies only when
// nothing names the field itself.
func (s *ServiceTestSuite) TestAFieldNamedDirectlyBeatsItsFallback() {
	ctx := context.Background()
	_, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules: map[string]OverlayRule{
			"assignments":      {Editable: true, AllowedValues: members("a", "b")},
			"assignments.user": {Editable: true, AllowedValues: members("a")},
		},
	})
	s.Require().Nil(svcErr)

	eff, svcErr := s.svc.ResolveOverlayRules(ctx, testType, testResource, rootOU)
	s.Require().Nil(svcErr)

	s.Equal([]string{"a"}, *eff.Rules["assignments.user"].AllowedValues,
		"the field's own rule stands, not the coarser one")
}

// The same lock has to hold on the write path. An organization unit whose coarse field is locked
// cannot hand the finer one on as editable, which would launder the lock through a reshare.
func (s *ServiceTestSuite) TestALockedCoarseFieldCannotBeLaunderedThroughTheFinerOne() {
	ctx := context.Background()
	_, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules:  map[string]OverlayRule{"assignments": {Editable: false}},
	})
	s.Require().Nil(svcErr)

	_, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  entry(childOU),
		OverlayRules:   map[string]OverlayRule{"assignments.user": {Editable: true}},
	})

	s.Require().NotNil(svcErr, "rootOU holds assignments locked, so it holds assignments.user locked")
	s.Equal(ErrorRuleWidens.Code, svcErr.Code)
}

// An edit can take the resource away from an organization unit that had reshared it. Re-materializing
// runs before cleanup, so it meets that unit mid-edit with no ceiling left. Resolution answers an
// invisible unit with empty rules, and reading that as "no policy has an opinion" would clamp the
// dead policy against the type's defaults, which can be wider than the ceiling it actually held.
func (s *ServiceTestSuite) TestAnEditDoesNotRewriteAPolicyItIsAboutToKill() {
	ctx := context.Background()
	owner := s.share(map[string]OverlayRule{
		"assignments": {Editable: true, AllowedValues: members("a", "b", "c")},
	})
	reshare, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		InitiatingOUID: rootOU,
		TargetOUScope:  entry(childOU),
		OverlayRules: map[string]OverlayRule{
			"assignments": {Editable: true, AllowedValues: members("a", "b", "c")},
		},
	})
	s.Require().Nil(svcErr)

	// The owner narrows, so the reshare is clamped to [a] while its request still asks for [a b c].
	owner, svcErr = s.svc.UpdatePolicy(ctx, owner.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules:  map[string]OverlayRule{"assignments": {Editable: true, AllowedValues: members("a")}},
		Version:       owner.Version,
	})
	s.Require().Nil(svcErr)
	s.Require().Equal([]string{"a"}, s.resolvedBound(reshare.ID))

	// Only the next edit's writes matter, so the earlier legitimate rewrite is not counted.
	s.store.replaced = nil

	// Now the owner moves the target away, so rootOU loses the resource in this same edit.
	_, svcErr = s.svc.UpdatePolicy(ctx, owner.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{otherOU}},
		OverlayRules:  map[string]OverlayRule{"assignments": {Editable: true, AllowedValues: members("a")}},
		Version:       owner.Version,
	})
	s.Require().Nil(svcErr, "the owner may always move its own target")

	visible, svcErr := s.svc.IsVisible(ctx, testType, testResource, rootOU)
	s.Require().Nil(svcErr)
	s.False(visible)

	remaining, err := s.store.ListAllPoliciesForResource(ctx, testType, testResource)
	s.Require().NoError(err)
	for _, p := range remaining {
		s.NotEqual(rootOU, p.InitiatingOUID, "the reshare goes with the visibility it depended on")
	}
	s.NotContains(s.store.replaced, reshare.ID,
		"the dead reshare must not be rewritten on the way out, since the only ceiling left to "+
			"clamp it against is the type's default, which is wider than the one it held")
}

// shareWithRule shares the resource to rootOU under one rule for "assignments", which is the setup
// every overlay value test starts from.
func (s *ServiceTestSuite) shareWithRule(rule OverlayRule) {
	_, svcErr := s.svc.CreatePolicy(context.Background(), testType, testResource, ownerOU,
		PolicyRequest{
			TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
			OverlayRules:  map[string]OverlayRule{"assignments": rule},
		})
	s.Require().Nil(svcErr)
}

// A value the rule permits is stored and comes back as the organization unit's own, which is the
// whole point of an editable field: the unit holds something the owner did not choose for it.
func (s *ServiceTestSuite) TestAnOrganizationUnitChoosesItsOwnValue() {
	s.shareWithRule(OverlayRule{Editable: true, AllowedValues: members("a", "b", "c")})
	ctx := context.Background()

	s.Require().Nil(s.svc.SetOverlayValue(ctx, testType, testResource, rootOU, "assignments",
		[]string{"a", "c"}))

	resolved, svcErr := s.svc.ResolveOverlayValues(ctx, testType, testResource, rootOU)
	s.Require().Nil(svcErr)
	s.True(resolved.Visible)
	s.ElementsMatch([]string{"a", "c"}, resolved.Values["assignments"].Value)
	s.Equal(SourceOverlay, resolved.Values["assignments"].Source)
	s.True(resolved.Values["assignments"].Editable)
}

// A unit that has chosen nothing holds what the rule carries, not an empty set. The two are
// different answers: one is "the owner shared this", the other is "the unit cleared it".
func (s *ServiceTestSuite) TestAnUnchosenFieldHoldsTheRuleValue() {
	s.shareWithRule(OverlayRule{Editable: true, Value: members("a", "b")})

	resolved, svcErr := s.svc.ResolveOverlayValues(context.Background(), testType, testResource, rootOU)
	s.Require().Nil(svcErr)
	s.ElementsMatch([]string{"a", "b"}, resolved.Values["assignments"].Value)
	s.Equal(SourceRule, resolved.Values["assignments"].Source)
}

// The rule's own bound is enforced at write time rather than silently trimmed, so a unit is never
// left believing it holds a value that was never stored.
func (s *ServiceTestSuite) TestAValueOutsideTheAllowedSetIsRefused() {
	s.shareWithRule(OverlayRule{Editable: true, AllowedValues: members("a", "b")})

	svcErr := s.svc.SetOverlayValue(context.Background(), testType, testResource, rootOU,
		"assignments", []string{"a", "z"})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorValueNotPermitted.Code, svcErr.Code)
}

// Exclusions are subtracted last, so a member the rule carved out is refused even though the menu
// still covers it. Without this the carve-out would be advisory.
func (s *ServiceTestSuite) TestAnExcludedMemberIsRefused() {
	s.shareWithRule(OverlayRule{
		Editable:       true,
		AllowedValues:  members("a", "b", "c"),
		ExcludedValues: members("b"),
	})

	svcErr := s.svc.SetOverlayValue(context.Background(), testType, testResource, rootOU,
		"assignments", []string{"b"})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorValueNotPermitted.Code, svcErr.Code)
}

// An exclusion also has to survive resolution, which is where the rule's carve-outs are finally
// applied: the rule carries them rather than baking them into its value, so nothing removes them
// until a value is read.
func (s *ServiceTestSuite) TestAnExcludedMemberIsDroppedFromTheRuleValue() {
	s.shareWithRule(OverlayRule{
		Editable:       true,
		Value:          members("a", "b"),
		ExcludedValues: members("b"),
	})

	resolved, svcErr := s.svc.ResolveOverlayValues(context.Background(), testType, testResource, rootOU)
	s.Require().Nil(svcErr)
	s.Equal([]string{"a"}, resolved.Values["assignments"].Value,
		"the carved-out member survived into the value the unit was told it holds")
}

// A pinned field is the owner's decision, not an opening position, so there is nothing for the
// target to write.
func (s *ServiceTestSuite) TestAPinnedFieldCannotBeWritten() {
	s.shareWithRule(OverlayRule{Editable: false, Value: members("a")})

	svcErr := s.svc.SetOverlayValue(context.Background(), testType, testResource, rootOU,
		"assignments", []string{"a"})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorFieldNotEditable.Code, svcErr.Code)
}

// A unit the resource never reached holds nothing, and must not be able to write as though it did.
func (s *ServiceTestSuite) TestAnOrganizationUnitOutsideTheShareCannotWrite() {
	s.shareWithRule(OverlayRule{Editable: true})
	ctx := context.Background()

	svcErr := s.svc.SetOverlayValue(ctx, testType, testResource, otherOU, "assignments", []string{"a"})
	s.Require().NotNil(svcErr)
	s.Equal(ErrorNotShared.Code, svcErr.Code)

	resolved, svcErr := s.svc.ResolveOverlayValues(ctx, testType, testResource, otherOU)
	s.Require().Nil(svcErr)
	s.False(resolved.Visible)
	s.Empty(resolved.Values, "an invisible resource reports no values, the way it reports no rules")
}

// A field the resource type does not declare is rejected before any resolution, since there is no
// rule that could govern it.
func (s *ServiceTestSuite) TestAnUndeclaredFieldCannotBeWritten() {
	s.shareWithRule(OverlayRule{Editable: true})

	svcErr := s.svc.SetOverlayValue(context.Background(), testType, testResource, rootOU,
		"nonexistent", []string{"a"})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorUnknownFieldKey.Code, svcErr.Code)
}

// The read path clamps rather than refuses, which is what lets an owner narrow a field after a
// target has already chosen a value: the stored choice is cut back to the new bound instead of
// standing in the owner's way.
func (s *ServiceTestSuite) TestAStoredChoiceIsClampedWhenTheRuleNarrows() {
	ctx := context.Background()
	owner, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules: map[string]OverlayRule{
			"assignments": {Editable: true, AllowedValues: members("a", "b")},
		},
	})
	s.Require().Nil(svcErr)
	s.Require().Nil(s.svc.SetOverlayValue(ctx, testType, testResource, rootOU, "assignments",
		[]string{"a", "b"}))

	_, svcErr = s.svc.UpdatePolicy(ctx, owner.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules: map[string]OverlayRule{
			"assignments": {Editable: true, AllowedValues: members("a")},
		},
		Version: owner.Version,
	})
	s.Require().Nil(svcErr)

	resolved, svcErr := s.svc.ResolveOverlayValues(ctx, testType, testResource, rootOU)
	s.Require().Nil(svcErr)
	s.Equal([]string{"a"}, resolved.Values["assignments"].Value,
		"the half of the stored choice the owner withdrew is still being reported")
}

// A choice made before the field was pinned is not a choice the unit still holds, so the pin wins
// outright rather than the two being merged.
func (s *ServiceTestSuite) TestAStoredChoiceIsIgnoredOnceTheFieldIsPinned() {
	ctx := context.Background()
	owner, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules:  map[string]OverlayRule{"assignments": {Editable: true}},
	})
	s.Require().Nil(svcErr)
	s.Require().Nil(s.svc.SetOverlayValue(ctx, testType, testResource, rootOU, "assignments",
		[]string{"chosen"}))

	_, svcErr = s.svc.UpdatePolicy(ctx, owner.ID, PolicyRequest{
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
		OverlayRules:  map[string]OverlayRule{"assignments": {Editable: false, Value: members("pinned")}},
		Version:       owner.Version,
	})
	s.Require().Nil(svcErr)

	resolved, svcErr := s.svc.ResolveOverlayValues(ctx, testType, testResource, rootOU)
	s.Require().Nil(svcErr)
	s.Equal([]string{"pinned"}, resolved.Values["assignments"].Value)
	s.Equal(SourceRule, resolved.Values["assignments"].Source,
		"a value the unit can no longer change is not the unit's own")
	s.False(resolved.Values["assignments"].Editable)
}

// Deleting a choice returns the field to the rule, which is what makes the write reversible.
func (s *ServiceTestSuite) TestDeletingAChoiceReturnsTheFieldToTheRule() {
	s.shareWithRule(OverlayRule{Editable: true, Value: members("a"), AllowedValues: members("a", "b")})
	ctx := context.Background()
	s.Require().Nil(s.svc.SetOverlayValue(ctx, testType, testResource, rootOU, "assignments",
		[]string{"b"}))

	s.Require().Nil(s.svc.DeleteOverlayValue(ctx, testType, testResource, rootOU, "assignments"))

	resolved, svcErr := s.svc.ResolveOverlayValues(ctx, testType, testResource, rootOU)
	s.Require().Nil(svcErr)
	s.Equal([]string{"a"}, resolved.Values["assignments"].Value)
	s.Equal(SourceRule, resolved.Values["assignments"].Source)
}

// A finer field no policy names is governed by the coarser one it falls back to, and a value
// written to it is checked against that rule. Otherwise naming the finer key would be a way around
// the bound the coarse key carries.
func (s *ServiceTestSuite) TestAFallbackFieldIsWrittenAgainstTheCoarseRule() {
	s.shareWithRule(OverlayRule{Editable: true, AllowedValues: members("a")})
	ctx := context.Background()

	svcErr := s.svc.SetOverlayValue(ctx, testType, testResource, rootOU, "assignments.user",
		[]string{"z"})
	s.Require().NotNil(svcErr)
	s.Equal(ErrorValueNotPermitted.Code, svcErr.Code)

	s.Require().Nil(s.svc.SetOverlayValue(ctx, testType, testResource, rootOU, "assignments.user",
		[]string{"a"}), "the coarse rule permits this member, so the finer field takes it")
}

// The owner is answered too. It holds the resource without any policy reaching it, so a values
// lookup that only served sharees would report nothing for the one unit that owns the thing.
func (s *ServiceTestSuite) TestTheOwnerResolvesItsOwnValues() {
	ctx := context.Background()
	s.Require().Nil(s.svc.SetOverlayValue(ctx, testType, testResource, ownerOU, "assignments",
		[]string{"a"}))

	resolved, svcErr := s.svc.ResolveOverlayValues(ctx, testType, testResource, ownerOU)
	s.Require().Nil(svcErr)
	s.True(resolved.Owned)
	s.True(resolved.Visible)
	s.Equal([]string{"a"}, resolved.Values["assignments"].Value)
}

// A resource type keeping its own overlay values has opted out of the framework's table, and
// cleanup already defers to it. Writing there anyway would leave rows nothing ever deletes.
func (s *ServiceTestSuite) TestATypeHoldingItsOwnValuesIsNotServedFromTheFrameworkTable() {
	storeMock, store := newMockStore(s.T())
	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(storeMock, nil, hierarchy, enumerator, inlineTx(s.T()), nil, nil, false)
	svc.RegisterResourceType(&cleanerDeclaration{})
	ctx := context.Background()

	svcErr := svc.SetOverlayValue(ctx, testType, testResource, rootOU, "assignments", []string{"a"})
	s.Require().NotNil(svcErr)
	s.Empty(store.values, "a value was written to a table the type's own cleanup never visits")

	_, svcErr = svc.ResolveOverlayValues(ctx, testType, testResource, rootOU)
	s.Require().NotNil(svcErr, "reporting no choices for a unit that has made some would be wrong")
}

// Clearing a field is something the unit did, so the rule's value must not reappear under it. The
// difference rides on nil versus empty, and a cleared set round-trips through the store as null.
func (s *ServiceTestSuite) TestAClearedFieldStaysCleared() {
	s.shareWithRule(OverlayRule{Editable: true, Value: members("a")})
	ctx := context.Background()

	s.Require().Nil(s.svc.SetOverlayValue(ctx, testType, testResource, rootOU, "assignments", nil))

	resolved, svcErr := s.svc.ResolveOverlayValues(ctx, testType, testResource, rootOU)
	s.Require().Nil(svcErr)
	s.Empty(resolved.Values["assignments"].Value, "the rule's value came back under a cleared field")
	s.Equal(SourceOverlay, resolved.Values["assignments"].Source)
}

// A store that cannot be reached is reported as an internal failure rather than as an empty answer.
// Answering a failed read with no values would tell an organization unit it holds nothing, and
// answering a failed write with success would lose the value it just chose.
func (s *ServiceTestSuite) TestAStoreFailureIsNotReportedAsAnEmptyAnswer() {
	storeMock := newSharingPolicyStoreInterfaceMock(s.T())
	a := mock.Anything
	boom := errors.New("store unreachable")
	storeMock.EXPECT().ListPoliciesRelevantToChain(a, a, a, a).Return(nil, nil).Maybe()
	storeMock.EXPECT().GetOverlayValues(a, a, a, a).Return(nil, boom).Maybe()
	storeMock.EXPECT().SetOverlayValue(a, a, a, a, a, a).Return(boom).Maybe()
	storeMock.EXPECT().DeleteOverlayValue(a, a, a, a, a).Return(boom).Maybe()

	hierarchy, enumerator := testResolver(s.T())
	svc := newSharingService(storeMock, nil, hierarchy, enumerator, inlineTx(s.T()), nil, nil, false)
	svc.RegisterResourceType(&testDeclaration{})
	ctx := context.Background()

	// The owner is visible without any policy, so these reach the store with nothing else in the way.
	svcErr := svc.SetOverlayValue(ctx, testType, testResource, ownerOU, "assignments", []string{"a"})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	svcErr = svc.DeleteOverlayValue(ctx, testType, testResource, ownerOU, "assignments")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	_, svcErr = svc.ResolveOverlayValues(ctx, testType, testResource, ownerOU)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// A file declaring a policy for an organization unit the API already recorded one for is refused.
// Nothing folds the two stores together, so allowing it would leave the unit governed by a file and
// a database row at once, and the next startup would seed the declaration beside the row.
func (s *ServiceTestSuite) TestADeclarationCannotGovernAnOUAStoredPolicyAlreadyDoes() {
	ctx := context.Background()
	stored, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}}})
	s.Require().Nil(svcErr)

	_, svcErr = s.svc.CreateDeclarativePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		ID:            declaredID,
		TargetOUScope: TargetOUScope{RootOUIDs: []string{otherOU}},
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorDeclaredPolicyConflictsWithStored.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, stored.ID,
		"the rejection has to name the row standing in the way, or an operator cannot go remove it")
}

// The refusal has to survive a restart, which is the case the reviewer raised: the declaration is
// replayed against a database that already holds the row, and must still be refused rather than
// seeded beside it.
func (s *ServiceTestSuite) TestAReplayedDeclarationIsStillRefusedAgainstAStoredPolicy() {
	ctx := context.Background()
	_, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}}})
	s.Require().Nil(svcErr)

	// A restart replays the file against the same database, through a service holding no declared
	// state of its own.
	declStore := newFileBasedStore()
	hierarchy, enumerator := testResolver(s.T())
	restarted := newSharingService(s.store, declStore, hierarchy, enumerator,
		inlineTx(s.T()), nil, nil, false)
	restarted.RegisterResourceType(&testDeclaration{})

	_, svcErr = restarted.CreateDeclarativePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		ID:            declaredID,
		TargetOUScope: TargetOUScope{RootOUIDs: []string{otherOU}},
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorDeclaredPolicyConflictsWithStored.Code, svcErr.Code)
	s.Empty(declStore.listForResource(testType, testResource),
		"the refused declaration was seeded anyway, so the unit is governed by both stores")
}

// The other order is refused too, so the guard does not depend on which arrived first.
func (s *ServiceTestSuite) TestAStoredPolicyCannotGovernAnOUADeclarationAlreadyDoes() {
	ctx := context.Background()
	_, svcErr := s.svc.CreateDeclarativePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		ID:            declaredID,
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
	})
	s.Require().Nil(svcErr)

	_, svcErr = s.svc.CreatePolicy(ctx, testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{otherOU}}})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorPolicyExists.Code, svcErr.Code)
}

// Re-applying the same file is still an upsert. The guard has to separate a declaration colliding
// with a stored row from one replacing what it declared itself, or every restart would fail.
func (s *ServiceTestSuite) TestAReappliedDeclarationStillReplacesItsOwn() {
	ctx := context.Background()
	_, svcErr := s.svc.CreateDeclarativePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		ID:            declaredID,
		TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}},
	})
	s.Require().Nil(svcErr)

	again, svcErr := s.svc.CreateDeclarativePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		ID:            declaredID,
		TargetOUScope: TargetOUScope{RootOUIDs: []string{otherOU}},
	})

	s.Require().Nil(svcErr, "a file re-applying itself is an upsert, not a conflict")
	s.Equal(declaredID, again.ID)
	s.Len(s.declStore.listForResource(testType, testResource), 1,
		"the replay appended a second declared policy instead of replacing the first")
}

// A declaration for a different organization unit is unaffected: the rule is one policy per unit
// per resource, not one policy per resource.
func (s *ServiceTestSuite) TestADeclarationForAnotherOUIsUnaffectedByAStoredPolicy() {
	ctx := context.Background()
	_, svcErr := s.svc.CreatePolicy(ctx, testType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{RootOUIDs: []string{rootOU}}})
	s.Require().Nil(svcErr)

	_, svcErr = s.svc.CreateDeclarativePolicy(ctx, testType, testResource, ownerOU, PolicyRequest{
		ID:             declaredID,
		InitiatingOUID: rootOU,
		TargetOUScope:  entry(childOU),
	})

	s.Require().Nil(svcErr)
}

// A resource type that declares no overlay field is the second shape the framework has to serve.
// Sharing one hands over visibility and nothing else: there is no field to narrow, no value to
// choose and nothing to clean up. These cover that shape; everything above exercises a type whose
// fields are the point.

// declareFieldless issues the one kind of policy a fieldless type can carry: a reach, and no rules.
func (s *ServiceTestSuite) declareFieldless(id string, scope TargetOUScope) *tidcommon.ServiceError {
	_, svcErr := s.svc.CreateDeclarativePolicy(context.Background(), fieldlessType, testResource,
		ownerOU, PolicyRequest{ID: id, TargetOUScope: scope})
	return svcErr
}

// A blanket policy reaches every organization unit, which is all sharing a fieldless type means:
// those units may be named, and nothing about the resource changes.
func (s *ServiceTestSuite) TestABlanketPolicyOnAFieldlessTypeReachesEveryUnit() {
	s.Require().Nil(s.declareFieldless(declaredID, TargetOUScope{AllOUs: true}))

	for _, ouID := range []string{rootOU, childOU, grandOU, otherOU} {
		visible, svcErr := s.svc.IsVisible(context.Background(), fieldlessType, testResource, ouID)
		s.Require().Nil(svcErr)
		s.True(visible, ouID+" is reached by a deployment-wide policy")
	}
}

// A carve-out takes the branch beneath it, so a resource shared to everyone except one unit is not
// reachable by that unit's children either.
func (s *ServiceTestSuite) TestACarveOutOnAFieldlessTypeTakesTheBranchBeneathIt() {
	s.Require().Nil(s.declareFieldless(declaredID,
		TargetOUScope{AllOUs: true, ExcludedOUIDs: []string{childOU}}))

	for ouID, want := range map[string]bool{rootOU: true, childOU: false, grandOU: false, otherOU: true} {
		visible, svcErr := s.svc.IsVisible(context.Background(), fieldlessType, testResource, ouID)
		s.Require().Nil(svcErr)
		s.Equal(want, visible, ouID)
	}
}

// The owner needs no policy of its own, so a fieldless resource nobody has shared is still visible
// in the organization unit that owns it.
func (s *ServiceTestSuite) TestTheOwnerOfAFieldlessTypeNeedsNoPolicy() {
	visible, svcErr := s.svc.IsVisible(context.Background(), fieldlessType, testResource, ownerOU)

	s.Require().Nil(svcErr)
	s.True(visible)
}

// Being reached hands over no field. This is what "a sharee may not edit anything" means in the
// framework: a policy can only speak about fields the type declares, and this type declares none.
func (s *ServiceTestSuite) TestBeingReachedHandsOverNoFieldWhenNoneAreDeclared() {
	s.Require().Nil(s.declareFieldless(declaredID, TargetOUScope{AllOUs: true}))

	resolved, svcErr := s.svc.ResolveOverlayRules(context.Background(), fieldlessType, testResource, childOU)

	s.Require().Nil(svcErr)
	s.True(resolved.Visible)
	s.Empty(resolved.Rules, "there is no field to hold a rule")
	s.Empty(resolved.Sources)
}

// A rule naming a field the type never declared is refused rather than stored and ignored, so a file
// that tries to hand part of the resource over fails at startup instead of taking no effect.
func (s *ServiceTestSuite) TestARuleOnAFieldlessTypeIsRefusedHavingNoFieldToName() {
	_, svcErr := s.svc.CreateDeclarativePolicy(context.Background(), fieldlessType, testResource, ownerOU,
		PolicyRequest{
			ID:            declaredID,
			TargetOUScope: TargetOUScope{AllOUs: true},
			OverlayRules:  map[string]OverlayRule{"anything": {Editable: true}},
		})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorUnknownFieldKey.Code, svcErr.Code)
}

// The file owns the policy, so it cannot be deleted through the API: a resource shared by a file
// stays shared until that file says otherwise.
func (s *ServiceTestSuite) TestADeclaredPolicyOnAFieldlessTypeCannotBeDeleted() {
	s.Require().Nil(s.declareFieldless(declaredID, TargetOUScope{AllOUs: true}))

	svcErr := s.svc.DeletePolicy(context.Background(), declaredID)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorPolicyDeclared.Code, svcErr.Code)
}

// Re-declaring for the same organization unit replaces what was declared rather than adding to it,
// which is what makes replaying a file on every startup idempotent instead of accumulating.
//
// The replacement is keyed on the resource and the initiating organization unit, not on the policy
// id, so a file that rewrites its policy under a new id still replaces the old one. The consequence
// worth knowing: two separate documents declaring policies for one resource and one initiator
// collapse to whichever loaded last, rather than being reported as a conflict.
func (s *ServiceTestSuite) TestRedeclaringForTheSameInitiatorReplacesIt() {
	ctx := context.Background()
	s.Require().Nil(s.declareFieldless(declaredID, TargetOUScope{AllOUs: true}))
	reachedBefore, svcErr := s.svc.IsVisible(ctx, fieldlessType, testResource, otherOU)
	s.Require().Nil(svcErr)
	s.Require().True(reachedBefore, "a deployment-wide policy reaches another tree")

	s.Require().Nil(s.declareFieldless("a-second-policy-id", TargetOUScope{AllChildren: true}))

	held, svcErr := s.svc.ListPolicies(ctx, fieldlessType, testResource)
	s.Require().Nil(svcErr)
	s.Require().Len(held, 1, "one organization unit holds one policy, however often it is declared")
	s.Equal("a-second-policy-id", held[0].ID, "the latest application of the file is what stands")

	reachedAfter, svcErr := s.svc.IsVisible(ctx, fieldlessType, testResource, otherOU)
	s.Require().Nil(svcErr)
	s.False(reachedAfter, "and the reach it replaced is gone with it")
}
