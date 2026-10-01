// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
)

// FileBasedStoreTestSuite covers the store that holds the policies a resource file declares. It is
// the source of truth for those policies for the life of the process, so what it replaces and what
// it appends is the whole of declarative replay behavior.
type FileBasedStoreTestSuite struct {
	suite.Suite
	store *fileBasedStore
}

func TestFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedStoreTestSuite))
}

func (s *FileBasedStoreTestSuite) SetupTest() {
	s.store = newFileBasedStore()
}

// declaredPolicy builds a declared policy for one (resource, initiating organization unit) pair.
func declaredPolicy(id, resourceID, initiatingOUID string) Policy {
	return Policy{
		ID:             id,
		ResourceType:   testType,
		ResourceID:     resourceID,
		OwningOUID:     ownerOU,
		InitiatingOUID: initiatingOUID,
		Stage:          stageShare,
		Declared:       true,
		Targets:        []Target{{ID: id + "-t", Scope: targetScopeRoot, OUID: rootOU}},
	}
}

// A file re-applied on the next startup must replace what it declared rather than append to it,
// which is what keeps replay idempotent instead of accumulating a duplicate per restart.
func (s *FileBasedStoreTestSuite) TestSeedingTheSamePairReplacesIt() {
	s.store.seed(declaredPolicy("p1", testResource, rootOU))
	s.store.seed(declaredPolicy("p1-again", testResource, rootOU))

	held := s.store.listForResource(testType, testResource)
	s.Require().Len(held, 1, "the pair is declared once, however many times the file is applied")
	s.Equal("p1-again", held[0].ID, "the latest application of the file is what stands")
}

// The pair is the identity, so a second organization unit declaring against the same resource is a
// separate policy rather than a replacement.
func (s *FileBasedStoreTestSuite) TestSeedingADifferentPairAppends() {
	s.store.seed(declaredPolicy("p1", testResource, rootOU))
	s.store.seed(declaredPolicy("p2", testResource, otherOU))
	s.store.seed(declaredPolicy("p3", "resource-2", rootOU))

	s.Len(s.store.listForResource(testType, testResource), 2)
	s.Len(s.store.listForResource(testType, "resource-2"), 1)
	s.Len(s.store.listForType(testType), 3)
}

// Reads keep declaration order, which is the order the file listed its policies in. A reshare
// declared beneath another policy is only replayable after it, so the order is load-bearing.
func (s *FileBasedStoreTestSuite) TestReadsPreserveDeclarationOrder() {
	for _, id := range []string{"first", "second", "third"} {
		s.store.seed(declaredPolicy(id, testResource, id+"-ou"))
	}

	held := s.store.listForResource(testType, testResource)
	ids := make([]string, 0, len(held))
	for _, p := range held {
		ids = append(ids, p.ID)
	}
	s.Equal([]string{"first", "second", "third"}, ids)
}

// A store that holds nothing for a resource answers empty rather than failing: a resource with no
// declared policy is the ordinary case, not an error.
func (s *FileBasedStoreTestSuite) TestAnUndeclaredResourceReadsEmpty() {
	s.store.seed(declaredPolicy("p1", testResource, rootOU))

	s.Empty(s.store.listForResource(testType, "resource-unknown"))
	s.Empty(s.store.listForType(ResourceType("application")))
}

// The store answers the interface the way a store does, so a composite can hold it and the database
// one behind one type: a hit is a policy, a miss is errPolicyNotFound rather than a zero value.
func (s *FileBasedStoreTestSuite) TestInterfaceReadsReportAMissAsNotFound() {
	ctx := context.Background()
	s.store.seed(declaredPolicy("p1", testResource, rootOU))

	s.Run("by id", func() {
		got, err := s.store.GetPolicy(ctx, "p1")
		s.Require().NoError(err)
		s.Equal("p1", got.ID)

		_, err = s.store.GetPolicy(ctx, "no-such-policy")
		s.ErrorIs(err, errPolicyNotFound)
	})

	s.Run("by initiator", func() {
		got, err := s.store.GetPolicyByInitiator(ctx, testType, testResource, rootOU)
		s.Require().NoError(err)
		s.Equal("p1", got.ID)

		_, err = s.store.GetPolicyByInitiator(ctx, testType, testResource, otherOU)
		s.ErrorIs(err, errPolicyNotFound)

		_, err = s.store.GetPolicyByInitiator(ctx, ResourceType("application"), testResource, rootOU)
		s.ErrorIs(err, errPolicyNotFound, "the resource type is part of the identity")
	})

	s.Run("for a resource", func() {
		held, err := s.store.ListPoliciesForResource(ctx, testType, testResource)
		s.Require().NoError(err)
		s.Len(held, 1)
	})
}

// The chain-scoped read is what the database store narrows by organization unit. A declaration set
// is held whole in memory, so this store answers by resource instead, and by type when no resource
// is named, which is the reverse lookup's fetch.
func (s *FileBasedStoreTestSuite) TestTheChainScopedReadIgnoresTheChain() {
	ctx := context.Background()
	s.store.seed(declaredPolicy("p1", testResource, rootOU))
	s.store.seed(declaredPolicy("p2", "resource-2", rootOU))

	forResource, err := s.store.ListPoliciesRelevantToChain(ctx, testType, testResource, []string{rootOU})
	s.Require().NoError(err)
	s.Len(forResource, 1, "a named resource narrows to that resource")

	forType, err := s.store.ListPoliciesRelevantToChain(ctx, testType, "", nil)
	s.Require().NoError(err)
	s.Len(forType, 2, "no resource named spans the type")

	unreached, err := s.store.ListPoliciesRelevantToChain(ctx, testType, testResource, []string{otherOU})
	s.Require().NoError(err)
	s.Len(unreached, 1, "coverage is decided by the caller, so the chain does not filter here")
}

// A declaration is its file. Nothing writes here, and an organization unit's own overlay values are
// database rows whichever store the governing policy came from. The composite routes all of these
// to the database, so a call arriving here is a wiring mistake and says so.
func (s *FileBasedStoreTestSuite) TestEveryWriteAndEveryOverlayValueIsRefused() {
	ctx := context.Background()

	s.Run("writes", func() {
		s.ErrorIs(s.store.CreatePolicy(ctx, declaredPolicy("p1", testResource, rootOU)), errFileStoreReadOnly)
		s.ErrorIs(s.store.ReplacePolicyContents(ctx, declaredPolicy("p1", testResource, rootOU), 1),
			errFileStoreReadOnly)
		s.ErrorIs(s.store.DeletePolicy(ctx, "p1"), errFileStoreReadOnly)
	})

	s.Run("overlay values", func() {
		_, err := s.store.GetOverlayValues(ctx, testType, testResource, childOU)
		s.ErrorIs(err, errFileStoreReadOnly)
		s.ErrorIs(s.store.SetOverlayValue(ctx, testType, testResource, childOU, "assignments", nil),
			errFileStoreReadOnly)
		s.ErrorIs(s.store.DeleteOverlayValue(ctx, testType, testResource, childOU, "assignments"),
			errFileStoreReadOnly)
		s.ErrorIs(s.store.DeleteOverlayValuesForOU(ctx, testType, testResource, childOU), errFileStoreReadOnly)
	})
}

// Declarative resources load concurrently with requests already being served, so seeding and
// reading overlap. The store carries a lock for exactly that; this is what makes it observable.
func (s *FileBasedStoreTestSuite) TestSeedsAndReadsAreSafeTogether() {
	const declarations = 40
	var wg sync.WaitGroup
	for i := range declarations {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			s.store.seed(declaredPolicy(string(rune('a'+i%26))+string(rune('0'+i/26)), testResource,
				string(rune('A'+i%26))+string(rune('0'+i/26))))
		}(i)
		go func() {
			defer wg.Done()
			s.store.listForResource(testType, testResource)
		}()
	}
	wg.Wait()

	s.Len(s.store.listForResource(testType, testResource), declarations,
		"every declaration landed, and none replaced another")
}
