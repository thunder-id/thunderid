// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/secretresolver"
	"github.com/thunder-id/thunderid/internal/system/valueref"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// misplacedSecretsIn finds every place a version's content refers to a secret from a field that
// holds no credential, the fields secretFields names for each resource type. A gateway refuses such
// a resource on import, since resolved there the secret would be stored as an ordinary value that
// the field's API returns or that the resource sends elsewhere; finding them here lets a capture
// refuse the configuration and a diff report a version captured before that check.
func misplacedSecretsIn(content string, secretFields map[string][]string) []ValueReference {
	var misplaced []ValueReference
	for _, resource := range parseBundle(content) {
		var root yaml.Node
		if err := yaml.Unmarshal([]byte(quotePlaceholders(resource.Content)), &root); err != nil ||
			len(root.Content) == 0 {
			continue
		}
		for _, secret := range secretresolver.MisplacedSecrets(root.Content[0], secretFields[resource.Type]) {
			misplaced = append(misplaced, ValueReference{
				Name:         valueref.ReferencedName(secret.Reference),
				Kind:         ReferenceSecret,
				ResourceType: resource.Type,
				ResourceID:   resource.ID,
				ResourceName: resource.Name,
				Field:        secret.Field,
			})
		}
	}
	return misplaced
}

// misplacedSecretError refuses a capture, naming each place a secret stands where it may not.
func misplacedSecretError(misplaced []ValueReference) *tidcommon.ServiceError {
	places := make([]string, 0, len(misplaced))
	for _, reference := range misplaced {
		name := reference.ResourceID
		if name == "" {
			name = reference.ResourceName
		}
		places = append(places, reference.ResourceType+"/"+name+" "+reference.Field)
	}
	refused := ErrorMisplacedSecret
	refused.ErrorDescription.DefaultValue += ": " + strings.Join(places, ", ")
	return &refused
}
