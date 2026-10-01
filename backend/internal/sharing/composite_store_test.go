// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// CompositeStoreTestSuite covers the store that puts the declared policies and the stored ones
// behind one interface. What it decides is which half answers, which is the whole of the merge the
// service used to do by hand.
type CompositeStoreTestSuite struct {
	suite.Suite
	db    *sharingPolicyStoreInterfaceMock
	file  *fileBasedStore
	store sharingPolicyStoreInterface
}

func TestCompositeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeStoreTestSuite))
}

func (s *CompositeStoreTestSuite) SetupTest() {
	s.db = newSharingPolicyStoreInterfaceMock(s.T())
	s.file = newFileBasedStore()
	s.store = newCompositeStore(s.file, s.db)
}

// storedPolicy builds a database-backed policy for the test resource, held by one organization unit.
// The id is fixed, because what these tests read off an answer is which half it came from.
func storedPolicy(initiatingOUID string) Policy {
	p := declaredPolicy("stored", testResource, initiatingOUID)
	p.Declared = false
	return p
}

// A deployment that declares nothing gets the database store itself, so the fallbacks cost nothing
// where there is nothing to fall back to. Wrapping a nil half would also make every read ask a
// store that cannot answer.
func (s *CompositeStoreTestSuite) TestNoDeclarationsMeansNoWrapper() {
	s.Same(s.db, newCompositeStore(nil, s.db), "the database store is returned unwrapped")
}

// The database is asked first and answers alone when it can. The order matters for a policy whose
// declaration has been withdrawn from its file while the row it left behind survives: that row is
// what still applies.
func (s *CompositeStoreTestSuite) TestAStoredPolicyAnswersBeforeADeclaredOne() {
	ctx := context.Background()
	s.file.seed(declaredPolicy("declared", testResource, rootOU))
	s.db.EXPECT().GetPolicy(ctx, "shared-id").Return(storedPolicy(rootOU), nil).Once()

	got, err := s.store.GetPolicy(ctx, "shared-id")

	s.Require().NoError(err)
	s.Equal("stored", got.ID)
	s.False(got.Declared)
}

// Absent from the database means "not stored", not "does not exist": the declarations are asked
// next, and a declared policy is returned as the ordinary answer.
func (s *CompositeStoreTestSuite) TestADeclaredPolicyAnswersWhenNothingIsStored() {
	ctx := context.Background()
	s.file.seed(declaredPolicy("declared", testResource, rootOU))
	s.db.EXPECT().GetPolicy(ctx, "declared").Return(Policy{}, errPolicyNotFound).Once()

	got, err := s.store.GetPolicy(ctx, "declared")

	s.Require().NoError(err)
	s.Equal("declared", got.ID)
	s.True(got.Declared)
}

// A store that could not be read has not reported an absence, so the declarations are not consulted
// and the failure travels. Falling back here would answer from the file whenever the database was
// down, which reads as a policy that exists rather than as a deployment that is broken.
func (s *CompositeStoreTestSuite) TestADatabaseFailureIsNotAFallback() {
	ctx := context.Background()
	unreachable := errors.New("connection refused")
	s.file.seed(declaredPolicy("declared", testResource, rootOU))
	s.db.EXPECT().GetPolicy(ctx, "declared").Return(Policy{}, unreachable).Once()

	_, err := s.store.GetPolicy(ctx, "declared")

	s.ErrorIs(err, unreachable)
}

// The same three cases for the lookup that backs the one-policy-per-organization-unit rule.
func (s *CompositeStoreTestSuite) TestTheInitiatorLookupFallsBackTheSameWay() {
	ctx := context.Background()
	s.file.seed(declaredPolicy("declared", testResource, otherOU))

	s.Run("stored answers first", func() {
		s.db.EXPECT().GetPolicyByInitiator(ctx, testType, testResource, rootOU).
			Return(storedPolicy(rootOU), nil).Once()

		got, err := s.store.GetPolicyByInitiator(ctx, testType, testResource, rootOU)
		s.Require().NoError(err)
		s.Equal("stored", got.ID)
	})

	s.Run("declared answers when nothing is stored", func() {
		s.db.EXPECT().GetPolicyByInitiator(ctx, testType, testResource, otherOU).
			Return(Policy{}, errPolicyNotFound).Once()

		got, err := s.store.GetPolicyByInitiator(ctx, testType, testResource, otherOU)
		s.Require().NoError(err)
		s.Equal("declared", got.ID)
	})

	s.Run("neither half holds one", func() {
		s.db.EXPECT().GetPolicyByInitiator(ctx, testType, testResource, childOU).
			Return(Policy{}, errPolicyNotFound).Once()

		_, err := s.store.GetPolicyByInitiator(ctx, testType, testResource, childOU)
		s.ErrorIs(err, errPolicyNotFound)
	})
}

// A resource's policies are the union of both halves, which is what makes a declared policy visible
// to a listing at all.
func (s *CompositeStoreTestSuite) TestListingAResourceReturnsBothHalves() {
	ctx := context.Background()
	s.file.seed(declaredPolicy("declared", testResource, otherOU))
	s.db.EXPECT().ListPoliciesForResource(ctx, testType, testResource).
		Return([]Policy{storedPolicy(rootOU)}, nil).Once()

	held, err := s.store.ListPoliciesForResource(ctx, testType, testResource)

	s.Require().NoError(err)
	s.Require().Len(held, 2)
	s.Equal("stored", held[0].ID, "the stored half comes first, as the database returned it")
	s.Equal("declared", held[1].ID)
}

// Two policies for one organization unit is what every reader here treats as impossible, and
// requireSoleGovernor is what makes it so. The filter keeps that a fact rather than an assumption:
// were a row and a declaration ever to name the same unit, the row is the one that applies.
func (s *CompositeStoreTestSuite) TestADeclaredPolicyIsDroppedWhenARowGovernsTheSameUnit() {
	ctx := context.Background()
	s.file.seed(declaredPolicy("declared", testResource, rootOU))
	s.db.EXPECT().ListPoliciesForResource(ctx, testType, testResource).
		Return([]Policy{storedPolicy(rootOU)}, nil).Once()

	held, err := s.store.ListPoliciesForResource(ctx, testType, testResource)

	s.Require().NoError(err)
	s.Require().Len(held, 1, "one organization unit is answered with one policy")
	s.Equal("stored", held[0].ID)
}

// The chain-scoped read merges the same way, and carries the chain through to the database, which is
// the half that can narrow by it.
func (s *CompositeStoreTestSuite) TestTheChainScopedReadMergesAndPassesTheChainDown() {
	ctx := context.Background()
	chain := []string{rootOU, childOU}
	s.file.seed(declaredPolicy("declared", testResource, otherOU))
	s.db.EXPECT().ListPoliciesRelevantToChain(ctx, testType, testResource, chain).
		Return([]Policy{storedPolicy(childOU)}, nil).Once()

	held, err := s.store.ListPoliciesRelevantToChain(ctx, testType, testResource, chain)

	s.Require().NoError(err)
	s.Len(held, 2)
}

// A read that fails in the database fails outright, rather than answering with the declared half
// alone: a partial policy set resolves to a narrower answer than the truth, which reads as a
// resource somebody has lost access to.
func (s *CompositeStoreTestSuite) TestAFailedListIsNotAnsweredFromDeclarationsAlone() {
	ctx := context.Background()
	unreachable := errors.New("connection refused")
	s.file.seed(declaredPolicy("declared", testResource, otherOU))
	s.db.EXPECT().ListPoliciesForResource(ctx, testType, testResource).Return(nil, unreachable).Once()

	held, err := s.store.ListPoliciesForResource(ctx, testType, testResource)

	s.ErrorIs(err, unreachable)
	s.Nil(held)
}

// Writes go to the database alone. The file store refuses them, so routing one to the wrong half
// would surface here rather than silently succeeding.
func (s *CompositeStoreTestSuite) TestEveryWriteGoesToTheDatabase() {
	ctx := context.Background()
	p := storedPolicy(rootOU)
	s.db.EXPECT().CreatePolicy(ctx, p).Return(nil).Once()
	s.db.EXPECT().ReplacePolicyContents(ctx, p, 2).Return(nil).Once()
	s.db.EXPECT().DeletePolicy(ctx, "stored").Return(nil).Once()

	s.NoError(s.store.CreatePolicy(ctx, p))
	s.NoError(s.store.ReplacePolicyContents(ctx, p, 2))
	s.NoError(s.store.DeletePolicy(ctx, "stored"))
}

// An organization unit's own value for a field is a database row whichever store the policy
// governing it came from, so these never consult the declarations.
func (s *CompositeStoreTestSuite) TestOverlayValuesNeverConsultTheDeclarations() {
	ctx := context.Background()
	s.file.seed(declaredPolicy("declared", testResource, rootOU))
	s.db.EXPECT().GetOverlayValues(ctx, testType, testResource, childOU).
		Return(map[string][]string{"assignments": {"a"}}, nil).Once()
	s.db.EXPECT().SetOverlayValue(ctx, testType, testResource, childOU, "assignments", []string{"a"}).
		Return(nil).Once()
	s.db.EXPECT().DeleteOverlayValue(ctx, testType, testResource, childOU, "assignments").Return(nil).Once()
	s.db.EXPECT().DeleteOverlayValuesForOU(ctx, testType, testResource, childOU).Return(nil).Once()

	values, err := s.store.GetOverlayValues(ctx, testType, testResource, childOU)
	s.Require().NoError(err)
	s.Equal(map[string][]string{"assignments": {"a"}}, values)
	s.NoError(s.store.SetOverlayValue(ctx, testType, testResource, childOU, "assignments", []string{"a"}))
	s.NoError(s.store.DeleteOverlayValue(ctx, testType, testResource, childOU, "assignments"))
	s.NoError(s.store.DeleteOverlayValuesForOU(ctx, testType, testResource, childOU))
}

// The declared half is asked for a read the database could not answer, and only then. A composite
// that consulted it on every read would spend a scan of the declarations per request.
func (s *CompositeStoreTestSuite) TestTheDeclaredHalfIsOnlyAskedWhenItHasTo() {
	ctx := context.Background()
	asked := newSharingPolicyStoreInterfaceMock(s.T())
	asked.EXPECT().GetPolicy(mock.Anything, mock.Anything).Return(Policy{}, errPolicyNotFound).Maybe()
	store := newCompositeStore(asked, s.db)
	s.db.EXPECT().GetPolicy(ctx, "stored").Return(storedPolicy(rootOU), nil).Once()

	_, err := store.GetPolicy(ctx, "stored")

	s.Require().NoError(err)
	asked.AssertNotCalled(s.T(), "GetPolicy", ctx, "stored")
}
