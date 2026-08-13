// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import "github.com/thunder-id/thunderid/tests/integration/testutils"

/*
User-type resolution selects which attribute-mapping profile applies. It does NOT decide which user type
the identity provisions into — GetMappedUserType has a single consumer, GetAttributeMappings, and neither
the provisioning executor nor the user-type resolver executor reads the connection at all. That is
recorded as G17, because the naming invites the opposite reading.

Every scenario here therefore observes *which profile was applied*, by keying the two profiles to the
same local target with different sources: the person profile fills firstName from given_name, the
contractor profile fills it from family_name. The value of firstName on the provisioned user names the
profile that won.
*/

// twoProfiles builds a configuration carrying one profile per user type, distinguishable by the value
// each writes into firstName. Both also map the username, so provisioning completes either way and a
// failure to provision means no profile matched at all.
func (s *FederatedMappingSuite) twoProfiles(
	resolution *testutils.UserTypeResolution) *testutils.AttributeConfiguration {
	return &testutils.AttributeConfiguration{
		UserTypeResolution: resolution,
		UserTypeAttributeMappings: []testutils.UserTypeAttributeMapping{
			{UserType: fedPersonType.Name, Attributes: []testutils.AttributeMapping{
				pair("email", "username"), pair("given_name", "firstName"),
			}},
			{UserType: fedContractorType.Name, Attributes: []testutils.AttributeMapping{
				pair("email", "username"), pair("family_name", "firstName"),
			}},
		},
	}
}

const (
	// The claim values the two profiles write into firstName, used to identify which one applied.
	personProfileMarker     = "Federated" // given_name
	contractorProfileMarker = "User"      // family_name
)

// B6: a value mapping whose key matches the incoming claim selects the profile it names.
func (s *FederatedMappingSuite) TestValueMappingHitSelectsThatProfile() {
	user := s.baseUser(s.nextSubject())
	user.Custom["employment"] = "staff"

	attributes := s.register(s.twoProfiles(&testutils.UserTypeResolution{
		Default:           fedPersonType.Name,
		ExternalAttribute: "employment",
		ValueMapping:      map[string]string{"staff": fedContractorType.Name},
	}), user)

	s.Equal(contractorProfileMarker, attributes["firstName"],
		"the mapped profile should have been applied, not the default one")
}

// B7: a claim value absent from the value mapping falls back to the default profile.
func (s *FederatedMappingSuite) TestValueMappingMissFallsBackToDefaultProfile() {
	user := s.baseUser(s.nextSubject())
	user.Custom["employment"] = "visitor"

	attributes := s.register(s.twoProfiles(&testutils.UserTypeResolution{
		Default:           fedPersonType.Name,
		ExternalAttribute: "employment",
		ValueMapping:      map[string]string{"staff": fedContractorType.Name},
	}), user)

	s.Equal(personProfileMarker, attributes["firstName"],
		"an unmapped claim value should fall back to the default profile")
}

// B8: with no value mapping configured, the claim value is used directly as the profile key.
func (s *FederatedMappingSuite) TestDirectClaimValueSelectsProfile() {
	user := s.baseUser(s.nextSubject())
	user.Custom["profile_type"] = fedContractorType.Name

	attributes := s.register(s.twoProfiles(&testutils.UserTypeResolution{
		Default:           fedPersonType.Name,
		ExternalAttribute: "profile_type",
	}), user)

	s.Equal(contractorProfileMarker, attributes["firstName"],
		"the claim value should be used directly as the profile key")
}

// B9: the external attribute may be a dot-notation path into a nested claim.
func (s *FederatedMappingSuite) TestNestedResolutionClaimSelectsProfile() {
	user := s.baseUser(s.nextSubject())
	user.Custom["profile"] = map[string]interface{}{"tier": "staff"}

	attributes := s.register(s.twoProfiles(&testutils.UserTypeResolution{
		Default:           fedPersonType.Name,
		ExternalAttribute: "profile.tier",
		ValueMapping:      map[string]string{"staff": fedContractorType.Name},
	}), user)

	s.Equal(contractorProfileMarker, attributes["firstName"],
		"resolution should traverse the nested claim path")
}

// BR17: the two resolution branches treat whitespace differently. Direct resolution trims the claim
// before using it as the profile key; a value-mapping lookup uses the raw string, so a padded value
// misses and falls back to the default.
func (s *FederatedMappingSuite) TestResolutionWhitespaceHandlingDiffersByBranch() {
	direct := s.baseUser(s.nextSubject())
	direct.Custom["profile_type"] = "  " + fedContractorType.Name + "  "

	attributes := s.register(s.twoProfiles(&testutils.UserTypeResolution{
		Default:           fedPersonType.Name,
		ExternalAttribute: "profile_type",
	}), direct)
	s.Equal(contractorProfileMarker, attributes["firstName"],
		"direct resolution trims the claim value before using it")

	mapped := s.baseUser(s.nextSubject())
	mapped.Custom["employment"] = "  staff  "

	attributes = s.register(s.twoProfiles(&testutils.UserTypeResolution{
		Default:           fedPersonType.Name,
		ExternalAttribute: "employment",
		ValueMapping:      map[string]string{"staff": fedContractorType.Name},
	}), mapped)
	s.Equal(personProfileMarker, attributes["firstName"],
		"a value-mapping lookup does not trim, so a padded value misses and falls back")
}

// BR18: neither branch case-folds. A value-mapping key must match exactly, and a directly used claim
// value that differs in case names no profile at all — which surfaces as a prompt, because no mapping
// then supplies the required username.
func (s *FederatedMappingSuite) TestResolutionIsCaseSensitive() {
	mapped := s.baseUser(s.nextSubject())
	mapped.Custom["employment"] = "STAFF"

	attributes := s.register(s.twoProfiles(&testutils.UserTypeResolution{
		Default:           fedPersonType.Name,
		ExternalAttribute: "employment",
		ValueMapping:      map[string]string{"staff": fedContractorType.Name},
	}), mapped)
	s.Equal(personProfileMarker, attributes["firstName"],
		"a value-mapping key is matched exactly, so different casing misses")

	direct := s.baseUser(s.nextSubject())
	direct.Custom["profile_type"] = "FED_CONTRACTOR"

	step := s.registerExpectingPrompt(s.twoProfiles(&testutils.UserTypeResolution{
		Default:           fedPersonType.Name,
		ExternalAttribute: "profile_type",
	}), direct)
	s.assertPromptsFor(step, "username")
}

// BR19: a non-string claim is stringified before it is used as the profile key, so numeric and boolean
// claims resolve rather than being ignored. JSON numbers arrive as float64, which is why that is the
// case worth pinning.
func (s *FederatedMappingSuite) TestNonStringResolutionClaimIsStringified() {
	numeric := s.baseUser(s.nextSubject())
	numeric.Custom["level"] = 3

	attributes := s.register(s.twoProfiles(&testutils.UserTypeResolution{
		Default:           fedPersonType.Name,
		ExternalAttribute: "level",
		ValueMapping:      map[string]string{"3": fedContractorType.Name},
	}), numeric)
	s.Equal(contractorProfileMarker, attributes["firstName"],
		"a numeric claim should stringify to its decimal form and match the mapping key")

	boolean := s.baseUser(s.nextSubject())
	boolean.Custom["contractor"] = true

	attributes = s.register(s.twoProfiles(&testutils.UserTypeResolution{
		Default:           fedPersonType.Name,
		ExternalAttribute: "contractor",
		ValueMapping:      map[string]string{"true": fedContractorType.Name},
	}), boolean)
	s.Equal(contractorProfileMarker, attributes["firstName"],
		"a boolean claim should stringify to true/false")
}
