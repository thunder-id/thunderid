// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// DeclarativeResourceTestSuite covers what the loader does with the sharing half of a resource's
// document: every policy it carries is declared, and a refusal stops startup saying which one.
type DeclarativeResourceTestSuite struct {
	suite.Suite
	svc   *declaringServiceStub
	seeds *policySeeder
}

func TestDeclarativeResourceTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeResourceTestSuite))
}

func (s *DeclarativeResourceTestSuite) SetupTest() {
	s.svc = &declaringServiceStub{}
	s.seeds = &policySeeder{ctx: context.Background(), svc: s.svc, rt: testType}
}

// declaringServiceStub records what was declared, and can refuse on command. The service interface
// is large and only one method is exercised here, so the stub embeds it rather than implementing
// every method: a call to any other panics, which is the right answer if one ever appears.
type declaringServiceStub struct {
	SharingServiceInterface
	declared []PolicyRequest
	refuse   *tidcommon.ServiceError
}

func (d *declaringServiceStub) CreateDeclarativePolicy(
	_ context.Context, _ ResourceType, _, _ string, req PolicyRequest,
) (Policy, *tidcommon.ServiceError) {
	if d.refuse != nil {
		return Policy{}, d.refuse
	}
	d.declared = append(d.declared, req)
	return Policy{ID: req.ID}, nil
}

// declaredDocument is the sharing half of one document, carrying two policies: the owner's, and a
// reshare issued by an organization unit it reaches. That is the only shape a document carrying two
// policies can take, since one organization unit holds one policy per resource.
func declaredDocument() *DeclaredResourcePolicies {
	return &DeclaredResourcePolicies{
		ResourceID:   testResource,
		ResourceName: "the-resource",
		OwningOUID:   ownerOU,
		Policies: []providers.SharingPolicy{
			{ID: declaredID, TargetOuScope: providers.SharingTargetOUScope{AllOUs: true}},
			{
				ID:             "a-second-policy",
				InitiatingOuID: rootOU,
				TargetOuScope:  providers.SharingTargetOUScope{AllChildren: true},
			},
		},
	}
}

// Every policy a document carries is declared, in the order the document lists them, because a
// reshare may name the policy above it and would not find one that had not been declared yet.
func (s *DeclarativeResourceTestSuite) TestEveryPolicyInADocumentIsDeclared() {
	s.Require().NoError(s.seeds.Create(testResource, declaredDocument()))

	s.Require().Len(s.svc.declared, 2)
	s.Equal(declaredID, s.svc.declared[0].ID)
	s.True(s.svc.declared[0].TargetOUScope.AllOUs)
	s.Equal("a-second-policy", s.svc.declared[1].ID)
	s.True(s.svc.declared[1].TargetOUScope.AllChildren)
}

// A refused declaration stops startup, and says which resource and which policy, so the operator is
// not left searching the directory for the document that is wrong.
func (s *DeclarativeResourceTestSuite) TestARefusalNamesTheResourceAndThePolicy() {
	s.svc.refuse = &tidcommon.InternalServerError

	err := s.seeds.Create(testResource, declaredDocument())

	s.Require().Error(err)
	s.Contains(err.Error(), "the-resource", "the failure names the resource")
	s.Contains(err.Error(), "policy 1", "and which of its policies")
	s.Contains(err.Error(), ownerOU, "and the organization unit the policy was issued by")
	s.Contains(err.Error(), tidcommon.InternalServerError.Code, "and what the framework said")
}

// A policy naming its own initiator is reported against that unit rather than against the owner,
// which is what tells two policies on one document apart.
func (s *DeclarativeResourceTestSuite) TestARefusalNamesTheInitiatorWhenOneIsGiven() {
	s.svc.refuse = &tidcommon.InternalServerError
	doc := declaredDocument()
	doc.Policies = doc.Policies[:1]
	doc.Policies[0].InitiatingOuID = rootOU

	err := s.seeds.Create(testResource, doc)

	s.Require().Error(err)
	s.Contains(err.Error(), rootOU)
}

// Two policies for one organization unit are refused before either is declared, because seeding
// would otherwise replace the first with the second and say nothing.
func (s *DeclarativeResourceTestSuite) TestTwoPoliciesForOneInitiatorAreRefused() {
	doc := declaredDocument()
	doc.Policies[1].InitiatingOuID = ""

	err := s.seeds.Create(testResource, doc)

	s.Require().Error(err)
	s.Contains(err.Error(), "the-resource", "the failure names the document")
	s.Contains(err.Error(), "policies 1 and 2", "and which two policies collide")
	s.Contains(err.Error(), ownerOU, "and the organization unit they both govern")
	s.Empty(s.svc.declared, "nothing is declared from a document that is refused")
}

// The same applies when the duplicate is spelled out rather than left to default to the owner.
func (s *DeclarativeResourceTestSuite) TestADuplicateInitiatorIsCaughtWhenNamedExplicitly() {
	doc := declaredDocument()
	doc.Policies[0].InitiatingOuID = rootOU

	err := s.seeds.Create(testResource, doc)

	s.Require().Error(err)
	s.Contains(err.Error(), rootOU)
	s.Empty(s.svc.declared)
}

// A document whose policies name different organization units is the reshare case and is untouched.
func (s *DeclarativeResourceTestSuite) TestPoliciesForDifferentInitiatorsAreDeclared() {
	s.Require().NoError(s.seeds.Create(testResource, declaredDocument()))

	s.Len(s.svc.declared, 2)
}

// A document declaring nothing is the ordinary case and must not reach the framework at all.
func (s *DeclarativeResourceTestSuite) TestADocumentDeclaringNothingIsNotAFailure() {
	s.Require().NoError(s.seeds.Create(testResource, &DeclaredResourcePolicies{ResourceID: testResource}))
	s.Require().NoError(s.seeds.Create(testResource, (*DeclaredResourcePolicies)(nil)))

	s.Empty(s.svc.declared)
}

// Anything that is not a policy bundle is a programming error in the consumer's parser, and is
// reported as one rather than silently skipped.
func (s *DeclarativeResourceTestSuite) TestAnUnexpectedTypeIsReported() {
	err := s.seeds.Create(testResource, "not a bundle")

	s.Require().Error(err)
	s.Contains(err.Error(), "unexpected data type")
}

// A deployment that loads no declarative resources has no file store, and asking it to load is a
// no-op rather than a failure, so a consumer may call it without checking first.
func (s *DeclarativeResourceTestSuite) TestLoadingWithoutAFileStoreIsANoOp() {
	err := loadDeclarativeResources(context.Background(), s.svc, nil, DeclarativeLoaderConfig{
		ResourceType: testType, DirectoryName: "whatever",
	})

	s.Require().NoError(err)
	s.Empty(s.svc.declared)
}

// The declarative and framework shapes are separate types, so the conversion between them is where
// a declared policy can quietly lose a field. These cover the two parts of it that carry structure
// rather than a scalar.

// Each named organization unit decides for itself whether its subtree comes too, so the flag has to
// survive per entry rather than per policy.
func (s *DeclarativeResourceTestSuite) TestChildEntriesKeepTheirOwnSubtreeFlag() {
	req := requestFromDeclaration(providers.SharingPolicy{
		ID: declaredID,
		TargetOuScope: providers.SharingTargetOUScope{
			ChildOUIDs: []providers.SharingTargetOUEntry{
				{OUID: childOU, AllChildren: true},
				{OUID: otherOU},
			},
		},
	})

	s.Equal(declaredID, req.ID)
	s.Require().Len(req.TargetOUScope.ChildOUIDs, 2)
	s.Equal(TargetEntry{OUID: childOU, AllChildren: true}, req.TargetOUScope.ChildOUIDs[0])
	s.Equal(TargetEntry{OUID: otherOU}, req.TargetOUScope.ChildOUIDs[1])
}

// Overlay rules carry three list fields whose absent and empty forms mean opposite things, so the
// conversion keeps the pointers rather than copying the lists behind them.
func (s *DeclarativeResourceTestSuite) TestOverlayRulesAreCarriedThrough() {
	value := []string{"read"}
	excluded := []string{}

	req := requestFromDeclaration(providers.SharingPolicy{
		ID:            declaredID,
		TargetOuScope: providers.SharingTargetOUScope{AllChildren: true},
		OverlayRules: map[string]providers.OverlayRule{
			"permissions": {Editable: false, Value: &value, ExcludedValues: &excluded},
		},
	})

	s.Require().Len(req.OverlayRules, 1)
	rule := req.OverlayRules["permissions"]
	s.False(rule.Editable)
	s.Equal(&value, rule.Value)
	s.Nil(rule.AllowedValues, "an omitted bound stays omitted")
	s.Require().NotNil(rule.ExcludedValues)
	s.Empty(*rule.ExcludedValues, "an empty exclusion list is not the same as none")
}

// A policy carrying neither leaves both nil, so the framework can tell "declared nothing" from
// "declared an empty list".
func (s *DeclarativeResourceTestSuite) TestAPolicyWithoutEntriesOrRulesCarriesNeither() {
	req := requestFromDeclaration(providers.SharingPolicy{
		ID:            declaredID,
		TargetOuScope: providers.SharingTargetOUScope{AllOUs: true},
	})

	s.Nil(req.TargetOUScope.ChildOUIDs)
	s.Nil(req.OverlayRules)
	s.True(req.TargetOUScope.AllOUs)
}

// assertParseFailure is what a consumer's parser returns when a document is not readable.
var assertParseFailure = errors.New("unreadable document")

// testResourceDirectory is the resource type's own directory, the one the loader is pointed at.
// The helper writes there and the configs read from there, so one name keeps the two in step.
const testResourceDirectory = "applications"

// writeDocument puts one declarative document where the loader looks for a resource type, and
// points the server runtime at the directory holding it.
func (s *DeclarativeResourceTestSuite) writeDocument(name, body string) {
	// The runtime initializes once per process and other suites in this package have already done
	// it, so point it at a directory of this test's own and put it back afterwards.
	home := s.T().TempDir()
	config.ResetServerRuntime()
	s.T().Cleanup(config.ResetServerRuntime)
	s.Require().NoError(config.InitializeServerRuntime(home, &config.Config{
		Server: engineconfig.ServerConfig{Hostname: "localhost", Port: 8080},
	}))

	resourceDir := filepath.Join(home, "config", "resources", testResourceDirectory)
	s.Require().NoError(os.MkdirAll(resourceDir, 0o750))
	s.Require().NoError(os.WriteFile(filepath.Join(resourceDir, name), []byte(body), 0o600))
}

// The loader reads a resource type's own directory, because a policy has no document of its own: it
// is carried inside the resource that declares it.
func (s *DeclarativeResourceTestSuite) TestLoadingDeclaresWhatTheDocumentsCarry() {
	s.writeDocument("billing.yaml", "resource_type: application\nid: billing\n")

	err := loadDeclarativeResources(context.Background(), s.svc, newFileBasedStore(), DeclarativeLoaderConfig{
		ResourceType:  testType,
		DirectoryName: testResourceDirectory,
		Parser: func([]byte) (*DeclaredResourcePolicies, error) {
			return declaredDocument(), nil
		},
	})

	s.Require().NoError(err)
	s.Require().Len(s.svc.declared, 2, "both of the document's policies are declared")
	s.Equal(declaredID, s.svc.declared[0].ID)
}

// A document the parser cannot read stops the load, and the failure names the resource type so the
// operator knows which directory to look in.
func (s *DeclarativeResourceTestSuite) TestAParserFailureStopsTheLoad() {
	s.writeDocument("billing.yaml", "resource_type: application\nid: billing\n")

	err := loadDeclarativeResources(context.Background(), s.svc, newFileBasedStore(), DeclarativeLoaderConfig{
		ResourceType:  testType,
		DirectoryName: testResourceDirectory,
		Parser: func([]byte) (*DeclaredResourcePolicies, error) {
			return nil, assertParseFailure
		},
	})

	s.Require().Error(err)
	s.Contains(err.Error(), string(testType))
	s.Empty(s.svc.declared)
}

// A directory with no documents is the ordinary case for a resource type nobody has shared, and is
// not a failure.
func (s *DeclarativeResourceTestSuite) TestLoadingAnEmptyDirectoryIsNotAFailure() {
	s.writeDocument(".keep", "")

	err := loadDeclarativeResources(context.Background(), s.svc, newFileBasedStore(), DeclarativeLoaderConfig{
		ResourceType:  testType,
		DirectoryName: "empty_dir",
		Parser: func([]byte) (*DeclaredResourcePolicies, error) {
			return declaredDocument(), nil
		},
	})

	s.Require().NoError(err)
	s.Empty(s.svc.declared)
}

// A parser that reads a document and finds nothing of its own returns no bundle at all. The loader
// still asks for an id to log against, so that answer has to be a blank rather than a panic.
func (s *DeclarativeResourceTestSuite) TestADocumentThatParsesToNothingIsSkipped() {
	s.writeDocument("billing.yaml", "resource_type: application\nid: billing\n")

	err := loadDeclarativeResources(context.Background(), s.svc, newFileBasedStore(), DeclarativeLoaderConfig{
		ResourceType:  testType,
		DirectoryName: testResourceDirectory,
		Parser: func([]byte) (*DeclaredResourcePolicies, error) {
			return nil, nil
		},
	})

	s.Require().NoError(err)
	s.Empty(s.svc.declared)
}
