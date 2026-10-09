// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// DeclarativeResourceTestSuite covers what the loader does with the sharing half of a resource's
// document: every policy it carries is declared, and a refusal stops startup saying which one.
type DeclarativeResourceTestSuite struct {
	suite.Suite
	svc      *SharingServiceInterfaceMock
	seeds    *policySeeder
	declared []PolicyRequest
}

func TestDeclarativeResourceTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeResourceTestSuite))
}

func (s *DeclarativeResourceTestSuite) SetupTest() {
	s.svc = NewSharingServiceInterfaceMock(s.T())
	s.seeds = &policySeeder{ctx: context.Background(), svc: s.svc, rt: testType}
	s.declared = nil
}

// acceptDeclarations lets every declaration through, recording each one in the order it was made.
//
// A test expecting nothing to be declared sets up neither this nor a refusal: the mock then fails
// on the first call, which says more than finding an empty record afterwards would, and says it at
// the point the unwanted declaration happens.
func (s *DeclarativeResourceTestSuite) acceptDeclarations() {
	s.svc.EXPECT().
		CreateDeclarativePolicy(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			_ context.Context, _ ResourceType, _, _ string, req PolicyRequest,
		) (Policy, *tidcommon.ServiceError) {
			s.declared = append(s.declared, req)
			return Policy{ID: req.ID}, nil
		})
}

// refuseDeclarations answers the first declaration with the given failure. It is expected exactly
// once, because a refused document stops there rather than carrying on to its remaining policies.
func (s *DeclarativeResourceTestSuite) refuseDeclarations(svcErr *tidcommon.ServiceError) {
	s.svc.EXPECT().
		CreateDeclarativePolicy(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(Policy{}, svcErr).Once()
}

// declaredDocument is the sharing half of one document, carrying two policies: the owner's, and a
// reshare issued by an organization unit it reaches. That is the only shape a document carrying two
// policies can take, since one organization unit holds one policy per resource.
func declaredDocument() *DeclaredResourcePolicies {
	return &DeclaredResourcePolicies{
		ResourceID:   testResource,
		ResourceName: "the-resource",
		OwningOUID:   ownerOU,
		Policies: []PolicyRequest{
			{ID: declaredID, Targets: []TargetRequest{{Scope: ScopeAllOUs}}},
			{
				ID:             "a-second-policy",
				InitiatingOUID: rootOU,
				Targets:        []TargetRequest{{Scope: ScopeAllChildren}},
			},
		},
	}
}

// Every policy a document carries is declared, in the order the document lists them, because a
// reshare may name the policy above it and would not find one that had not been declared yet.
func (s *DeclarativeResourceTestSuite) TestEveryPolicyInADocumentIsDeclared() {
	s.acceptDeclarations()

	s.Require().NoError(s.seeds.Create(testResource, declaredDocument()))

	s.Require().Len(s.declared, 2)
	s.Equal(declaredID, s.declared[0].ID)
	s.Equal(ScopeAllOUs, s.declared[0].Targets[0].Scope)
	s.Equal("a-second-policy", s.declared[1].ID)
	s.Equal(ScopeAllChildren, s.declared[1].Targets[0].Scope)
}

// A refused declaration stops startup, and says which resource and which policy, so the operator is
// not left searching the directory for the document that is wrong.
func (s *DeclarativeResourceTestSuite) TestARefusalNamesTheResourceAndThePolicy() {
	s.refuseDeclarations(&tidcommon.InternalServerError)

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
	s.refuseDeclarations(&tidcommon.InternalServerError)
	doc := declaredDocument()
	doc.Policies = doc.Policies[:1]
	doc.Policies[0].InitiatingOUID = rootOU

	err := s.seeds.Create(testResource, doc)

	s.Require().Error(err)
	s.Contains(err.Error(), rootOU)
}

// Two policies for one organization unit are refused before either is declared, because seeding
// would otherwise replace the first with the second and say nothing.
func (s *DeclarativeResourceTestSuite) TestTwoPoliciesForOneInitiatorAreRefused() {
	doc := declaredDocument()
	doc.Policies[1].InitiatingOUID = ""

	err := s.seeds.Create(testResource, doc)

	s.Require().Error(err)
	s.Contains(err.Error(), "the-resource", "the failure names the document")
	s.Contains(err.Error(), "policies 1 and 2", "and which two policies collide")
	s.Contains(err.Error(), ownerOU, "and the organization unit they both govern")
}

// The same applies when the duplicate is spelled out rather than left to default to the owner.
func (s *DeclarativeResourceTestSuite) TestADuplicateInitiatorIsCaughtWhenNamedExplicitly() {
	doc := declaredDocument()
	doc.Policies[0].InitiatingOUID = rootOU

	err := s.seeds.Create(testResource, doc)

	s.Require().Error(err)
	s.Contains(err.Error(), rootOU)
}

// A document whose policies name different organization units is the reshare case and is untouched.
func (s *DeclarativeResourceTestSuite) TestPoliciesForDifferentInitiatorsAreDeclared() {
	s.acceptDeclarations()

	s.Require().NoError(s.seeds.Create(testResource, declaredDocument()))

	s.Len(s.declared, 2)
}

// A document declaring nothing is the ordinary case and must not reach the framework at all.
func (s *DeclarativeResourceTestSuite) TestADocumentDeclaringNothingIsNotAFailure() {
	s.Require().NoError(s.seeds.Create(testResource, &DeclaredResourcePolicies{ResourceID: testResource}))
	s.Require().NoError(s.seeds.Create(testResource, (*DeclaredResourcePolicies)(nil)))
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
}

// A declared policy is read straight into the framework's own request shape, so what a document
// says and what the framework acts on cannot drift apart. What can still go wrong is the reading
// itself: the overlay list fields are pointers because absent and empty mean opposite things, and
// a document has to be able to say each.
func (s *DeclarativeResourceTestSuite) TestADocumentSaysAbsentAndEmptyApart() {
	var policy PolicyRequest
	s.Require().NoError(yaml.Unmarshal([]byte(`
id: `+declaredID+`
targets:
  - scope: allChildren
    overlayRules:
      permissions:
        editable: false
        value: [ read ]
        excludedValues: []
`), &policy))

	s.Equal(declaredID, policy.ID)
	s.Require().Len(policy.Targets, 1)
	s.Equal(ScopeAllChildren, policy.Targets[0].Scope)

	s.Require().Len(policy.Targets[0].OverlayRules, 1)
	rule := policy.Targets[0].OverlayRules["permissions"]
	s.False(rule.Editable)
	s.Require().NotNil(rule.Value)
	s.Equal([]string{"read"}, *rule.Value)
	s.Nil(rule.AllowedValues, "a bound the document omits stays omitted")
	s.Require().NotNil(rule.ExcludedValues)
	s.Empty(*rule.ExcludedValues, "an empty exclusion list is not the same as none")
}

// Each target names its own breadth and carries its own terms, so a document says both per entry
// rather than once for the policy.
func (s *DeclarativeResourceTestSuite) TestEachTargetKeepsItsOwnScopeAndTerms() {
	var policy PolicyRequest
	s.Require().NoError(yaml.Unmarshal([]byte(`
id: `+declaredID+`
targets:
  - scope: childSubtree
    ouId: `+childOU+`
    excludedOuIds: [ `+otherOU+` ]
  - scope: child
    ouId: `+otherOU+`
    overlayRules:
      permissions:
        editable: true
`), &policy))

	s.Require().Len(policy.Targets, 2)
	s.Equal(TargetRequest{
		Scope:         ScopeChildSubtree,
		OUID:          childOU,
		ExcludedOUIDs: []string{otherOU},
	}, policy.Targets[0])
	s.Equal(TargetRequest{
		Scope:        ScopeChild,
		OUID:         otherOU,
		OverlayRules: map[string]OverlayRule{"permissions": {Editable: true}},
	}, policy.Targets[1])
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
	s.acceptDeclarations()
	s.writeDocument("billing.yaml", "resource_type: application\nid: billing\n")

	err := loadDeclarativeResources(context.Background(), s.svc, newFileBasedStore(), DeclarativeLoaderConfig{
		ResourceType:  testType,
		DirectoryName: testResourceDirectory,
		Parser: func([]byte) (*DeclaredResourcePolicies, error) {
			return declaredDocument(), nil
		},
	})

	s.Require().NoError(err)
	s.Require().Len(s.declared, 2, "both of the document's policies are declared")
	s.Equal(declaredID, s.declared[0].ID)
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
}

// shapeRefusingDeclaration refuses the rule shapes a resource server refuses: a menu, an editable
// rule, or naming both what is shared and what is withheld.
type shapeRefusingDeclaration struct {
	testDeclaration
}

func (d *shapeRefusingDeclaration) ValidateOverlayRule(
	_ context.Context, _, _ string, r OverlayRule,
) *tidcommon.ServiceError {
	refuse := func(reason string) *tidcommon.ServiceError {
		return &tidcommon.ServiceError{
			Type:             tidcommon.ClientErrorType,
			Code:             "TST-0003",
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: reason},
		}
	}
	switch {
	case r.AllowedValues != nil:
		return refuse("allowedValues is not supported")
	case r.Editable:
		return refuse("the field cannot be editable")
	case r.Value != nil && r.ExcludedValues != nil:
		return refuse("value and excludedValues cannot be combined")
	default:
		return nil
	}
}

// loadDeclaredRules declares one document's policies through the real service and loader, the way a
// resource type does at startup, and returns what the load reports.
func (s *DeclarativeResourceTestSuite) loadDeclaredRules(document string) error {
	s.writeDocument("orders.yaml", document)
	hierarchy, enumerator := testResolver(s.T())
	fileStore := newFileBasedStore()
	svc := newSharingService(mustMockStore(s.T()), fileStore, hierarchy, enumerator,
		inlineTx(s.T()), nil, nil, false)
	svc.RegisterResourceType(&shapeRefusingDeclaration{testDeclaration{owner: rootOU}})

	return loadDeclarativeResources(context.Background(), svc, fileStore, DeclarativeLoaderConfig{
		ResourceType:  testType,
		DirectoryName: testResourceDirectory,
		Parser: func(data []byte) (*DeclaredResourcePolicies, error) {
			var doc struct {
				SharingPolicies []PolicyRequest `yaml:"sharingPolicies"`
			}
			if err := yaml.Unmarshal(data, &doc); err != nil {
				return nil, err
			}
			return &DeclaredResourcePolicies{
				ResourceID: testResource, ResourceName: "Orders API", OwningOUID: rootOU,
				Policies: doc.SharingPolicies,
			}, nil
		},
	})
}

// A rule the resource type refuses stops the load from a resource file just as it refuses an API
// request, whichever target carries it, and the failure carries the type's own reason and names the
// resource. Declaring a policy is never a way around the type's validation.
func (s *DeclarativeResourceTestSuite) TestADeclaredRuleTheTypeRefusesStopsTheLoad() {
	cases := []struct {
		name   string
		rule   string
		reason string
	}{
		{"allowed values", "allowedValues: [orders]", "allowedValues is not supported"},
		{"editable", "editable: true", "the field cannot be editable"},
		{"value with excluded values", "value: [orders]\n          excludedValues: [orders:delete]",
			"value and excludedValues cannot be combined"},
	}
	targets := map[string]string{
		"child target":       "      - scope: child\n        ouId: " + childOU + "\n",
		"allChildren target": "      - scope: allChildren\n",
	}
	for _, tc := range cases {
		for targetName, target := range targets {
			s.Run(targetName+" with "+tc.name, func() {
				err := s.loadDeclaredRules("sharingPolicies:\n" +
					"  - id: orders-policy\n" +
					"    targets:\n" +
					target +
					"        overlayRules:\n" +
					"          permissions:\n" +
					"            " + strings.ReplaceAll(tc.rule, "\n          ", "\n            ") + "\n")

				s.Require().Error(err)
				s.Contains(err.Error(), tc.reason)
				s.Contains(err.Error(), "Orders API")
			})
		}
	}
}

// A rule the type accepts loads, so the refusals above are about the rules and not the document.
func (s *DeclarativeResourceTestSuite) TestADeclaredRuleTheTypeAcceptsLoads() {
	err := s.loadDeclaredRules("sharingPolicies:\n" +
		"  - id: orders-policy\n" +
		"    targets:\n" +
		"      - scope: child\n" +
		"        ouId: " + childOU + "\n" +
		"        overlayRules:\n" +
		"          permissions:\n" +
		"            excludedValues: [orders:delete]\n")

	s.Require().NoError(err)
}
