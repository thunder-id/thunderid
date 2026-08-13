// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import "github.com/thunder-id/thunderid/tests/integration/testutils"

// B1: external claims are published under their configured local names on the provisioned user.
func (s *FederatedMappingSuite) TestCustomAttributeMappingsApplied() {
	sub := s.nextSubject()
	user := s.baseUser(sub)

	attributes := s.register(mapping(fedPersonType.Name,
		pair("given_name", "firstName"),
		pair("family_name", "lastName"),
		pair("email", "username"),
	), user)

	s.Equal("Federated", attributes["firstName"])
	s.Equal("User", attributes["lastName"])
	s.Equal(user.Email, attributes["username"], "the email claim should be published as the username")
}

// B2: a dot-notation external path reads through a nested claim object.
func (s *FederatedMappingSuite) TestNestedClaimPathMapped() {
	sub := s.nextSubject()
	user := s.baseUser(sub)
	user.Custom["address"] = map[string]interface{}{"locality": "Colombo", "region": "Western"}

	attributes := s.register(mapping(fedPersonType.Name,
		pair("email", "username"),
		pair("address.locality", "city"),
	), user)

	s.Equal("Colombo", attributes["city"], "the nested claim should be read through the dotted path")
}

// B3: one external claim can feed two local attributes. Mappings copy rather than rename, so both
// targets receive the value.
func (s *FederatedMappingSuite) TestOneClaimMappedToTwoAttributes() {
	sub := s.nextSubject()
	user := s.baseUser(sub)

	attributes := s.register(mapping(fedPersonType.Name,
		pair("email", "username"),
		pair("given_name", "firstName"),
		pair("given_name", "lastName"),
	), user)

	s.Equal("Federated", attributes["firstName"])
	s.Equal("Federated", attributes["lastName"], "a second target receives the same source claim")
}

// B4: mapping copies rather than consumes. The source claim survives under its own name, and claims
// with no mapping at all are still available to provisioning.
func (s *FederatedMappingSuite) TestUnmappedClaimsPassThroughAndSourcesSurvive() {
	sub := s.nextSubject()
	user := s.baseUser(sub)

	attributes := s.register(mapping(fedPersonType.Name,
		pair("email", "username"),
		pair("given_name", "firstName"),
	), user)

	s.Equal("Federated", attributes["firstName"], "the mapped target is published")
	s.Equal(sub, attributes["sub"],
		"sub is not mapped anywhere yet still reaches the provisioned user")
	s.Equal(user.Email, attributes["email"],
		"the email claim survives under its own name even though it also feeds username")
}

// B5: the resolved user type exists but has no entry in the mappings list, so no mapping applies.
func (s *FederatedMappingSuite) TestResolvedUserTypeWithoutMappingEntry() {
	user := s.baseUser(s.nextSubject())

	step := s.registerExpectingPrompt(&testutils.AttributeConfiguration{
		// Resolution names fed_person, but the only profile is keyed to fed_contractor.
		UserTypeResolution: &testutils.UserTypeResolution{Default: fedPersonType.Name},
		UserTypeAttributeMappings: []testutils.UserTypeAttributeMapping{{
			UserType:   fedContractorType.Name,
			Attributes: []testutils.AttributeMapping{pair("given_name", "firstName")},
		}},
	}, user)

	// No profile matches the resolved type, so nothing is mapped and the required username has no
	// source: provisioning stops and asks for it.
	s.assertPromptsFor(step, "username")
}

// BR1: the resolution default names a user type that does not exist. Nothing rejects that at
// configuration time (G4), so the failure is silent at runtime: no profile matches and no mapping is
// applied. This differs from B5 only in why the lookup misses.
func (s *FederatedMappingSuite) TestResolutionToNonexistentUserTypeAppliesNoMappings() {
	user := s.baseUser(s.nextSubject())

	step := s.registerExpectingPrompt(&testutils.AttributeConfiguration{
		UserTypeResolution: &testutils.UserTypeResolution{Default: "no_such_user_type"},
		UserTypeAttributeMappings: []testutils.UserTypeAttributeMapping{{
			UserType:   fedPersonType.Name,
			Attributes: []testutils.AttributeMapping{pair("given_name", "firstName")},
		}},
	}, user)

	// The misconfiguration surfaces only here, as an unexpected prompt during sign-up.
	s.assertPromptsFor(step, "username")
}

// BR2: claim-driven resolution with no value mapping uses the claim value directly as the profile key,
// so a claim naming an unknown type also silently applies nothing. Nothing constrains that value to a
// known user type (G5).
func (s *FederatedMappingSuite) TestDirectResolutionClaimNamingUnknownTypeAppliesNoMappings() {
	user := s.baseUser(s.nextSubject())
	user.Custom["profile_type"] = "no_such_user_type"

	step := s.registerExpectingPrompt(&testutils.AttributeConfiguration{
		UserTypeResolution: &testutils.UserTypeResolution{
			Default:           fedPersonType.Name,
			ExternalAttribute: "profile_type",
		},
		UserTypeAttributeMappings: []testutils.UserTypeAttributeMapping{{
			UserType:   fedPersonType.Name,
			Attributes: []testutils.AttributeMapping{pair("given_name", "firstName")},
		}},
	}, user)

	// The claim value is the profile key verbatim, so an unknown type matches no profile and the
	// configured default is never consulted as a fallback for the mappings.
	s.assertPromptsFor(step, "username")
}

// BR13: a dotted path whose intermediate segment is a scalar cannot be traversed, so the target is
// simply not published rather than the lookup failing loudly.
func (s *FederatedMappingSuite) TestNestedPathThroughScalarPublishesNothing() {
	sub := s.nextSubject()
	user := s.baseUser(sub)

	attributes := s.register(mapping(fedPersonType.Name,
		pair("email", "username"),
		// email is a string, so email.locality cannot resolve.
		pair("email.locality", "city"),
	), user)

	_, published := attributes["city"]
	s.False(published, "a path through a scalar should publish nothing")
	s.Equal(user.Email, attributes["username"], "the other mappings are unaffected")
}

// BR14: a dotted path whose final segment is absent behaves the same way.
func (s *FederatedMappingSuite) TestNestedPathWithMissingSegmentPublishesNothing() {
	sub := s.nextSubject()
	user := s.baseUser(sub)
	user.Custom["address"] = map[string]interface{}{"locality": "Colombo"}

	attributes := s.register(mapping(fedPersonType.Name,
		pair("email", "username"),
		pair("address.region", "city"),
	), user)

	_, published := attributes["city"]
	s.False(published, "a missing final segment should publish nothing")
}

// BR16: claim and attribute names are matched exactly. A source whose casing differs does not resolve,
// and a target is published under exactly the configured spelling.
func (s *FederatedMappingSuite) TestAttributeNamesAreCaseSensitive() {
	sub := s.nextSubject()
	user := s.baseUser(sub)

	attributes := s.register(mapping(fedPersonType.Name,
		pair("email", "username"),
		// The claim is given_name; Given_Name must not resolve.
		pair("Given_Name", "firstName"),
		pair("family_name", "lastName"),
	), user)

	_, mismatched := attributes["firstName"]
	s.False(mismatched, "a source claim whose casing differs should not resolve")
	s.Equal("User", attributes["lastName"], "the correctly cased mapping still applies")
}

// BR15: a claim that is present but null contributes nothing to the provisioned user.
//
// The two layers disagree, which is the point of testing this end to end rather than at the mapping
// function alone. ApplyAttributeMappings does publish the target, because the claim key exists and only
// a *missing* key is skipped. By the time the identity is provisioned the null has been dropped, so the
// attribute is indistinguishable from one that was never mapped. A null claim therefore cannot be used
// to satisfy an attribute, however the mapping is configured.
func (s *FederatedMappingSuite) TestNullClaimValueContributesNothing() {
	user := s.baseUser(s.nextSubject())
	user.Custom["work_city"] = nil

	attributes := s.register(mapping(fedPersonType.Name,
		pair("email", "username"),
		pair("work_city", "city"),
	), user)

	_, published := attributes["city"]
	s.False(published, "a null claim leaves the target absent on the provisioned user")
}

// BR3: a non-string claim mapped into a string-typed schema attribute. JSON numbers arrive as float64,
// so this is the shape a provider actually sends.
func (s *FederatedMappingSuite) TestNonStringClaimMappedIntoTypedAttribute() {
	user := s.baseUser(s.nextSubject())
	user.Custom["cost_centre"] = 4200

	attributes := s.register(mapping(fedPersonType.Name,
		pair("email", "username"),
		pair("cost_centre", "costCenter"),
	), user)

	s.Equal("4200", attributes["costCenter"],
		"a numeric claim should reach a string attribute in its decimal form")
}

// BR4: a mapping that targets a unique attribute whose value already belongs to another user. The
// conflict surfaces during provisioning rather than being silently overwritten.
func (s *FederatedMappingSuite) TestMappingOntoUniqueAttributeThatCollides() {
	existingEmail := s.nextSubject() + "@example.com"
	existing, err := testutils.CreateUser(testutils.User{
		Type: fedPersonType.Name,
		OUID: s.ouID,
		Attributes: mustJSON(map[string]interface{}{
			"username": existingEmail,
			"email":    existingEmail,
		}),
	})
	s.Require().NoError(err, "failed to create the colliding user")
	defer func() {
		if err := testutils.DeleteUser(existing); err != nil {
			s.T().Logf("failed to delete the colliding user: %v", err)
		}
	}()

	user := s.baseUser(s.nextSubject())
	user.Email = existingEmail

	step := s.registerExpectingPrompt(mapping(fedPersonType.Name,
		pair("email", "username"),
	), user)

	// Provisioning cannot create a second user with that unique username, so the flow does not complete.
	s.NotEqual("COMPLETE", step.FlowStatus,
		"a unique-attribute collision must not silently provision a duplicate: %+v", step)
}

// BR5: the profile is matched but the claim it reads is absent, so the required local attribute has no
// source and provisioning stops to collect it.
func (s *FederatedMappingSuite) TestRequiredAttributeLeftAbsentByMissingClaim() {
	user := s.baseUser(s.nextSubject())

	step := s.registerExpectingPrompt(mapping(fedPersonType.Name,
		// The identity carries no work_email claim.
		pair("work_email", "username"),
	), user)

	s.assertPromptsFor(step, "username")
}

// BR6: the claim is present but empty. An empty value is not a value for a required attribute, so this
// behaves as if it were absent.
func (s *FederatedMappingSuite) TestEmptyClaimValueForRequiredAttribute() {
	user := s.baseUser(s.nextSubject())
	user.Custom["work_email"] = ""

	step := s.registerExpectingPrompt(mapping(fedPersonType.Name,
		pair("work_email", "username"),
	), user)

	s.assertPromptsFor(step, "username")
}
