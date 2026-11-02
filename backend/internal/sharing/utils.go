// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"

	"gopkg.in/yaml.v3"
)

// DecodePolicyBody reads a policy request body through the strict decoder, which refuses a field the
// body does not declare: a misspelled excludedOuIds would otherwise read as a section with no
// exclusions and share the resource with the organization unit the caller named to keep it from. On
// a malformed body it answers 400 with invalidFormat, the mounting resource type's own error, and
// reports false.
func DecodePolicyBody[T PolicyCreateBody | PolicyUpdateBody](
	ctx context.Context, w http.ResponseWriter, r *http.Request, invalidFormat tidcommon.ServiceError,
) (*T, bool) {
	body, err := sysutils.DecodeJSONBodyStrict[T](r)
	if err != nil {
		sysutils.WriteErrorResponse(ctx, w, http.StatusBadRequest, apierror.ErrorResponse{
			Code:        invalidFormat.Code,
			Message:     invalidFormat.Error,
			Description: invalidFormat.ErrorDescription,
		})
		return nil, false
	}
	return body, true
}

// nullableString maps an empty string to a SQL null, so an absent parent or named organization
// unit stores as null rather than as an empty string a foreign key would reject.
func nullableString(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

// copyStrings returns a copy of a member list, so a stored rule cannot be mutated through a
// pointer the caller still holds.
func copyStrings(in *[]string) *[]string {
	if in == nil {
		return nil
	}
	out := append([]string{}, *in...)
	return &out
}

// copyRule returns a rule whose member lists share nothing with the rules it was folded from.
//
// The algebra composes by taking whichever side won, so its result aliases its inputs: a resolved
// rule would otherwise point at the parent policy's own slices, and at a cached resolution's. The
// copy is taken where a folded rule escapes into a stored row or a cache, not on the way in.
func copyRule(r OverlayRule) OverlayRule {
	r.Value = copyStrings(r.Value)
	r.AllowedValues = copyStrings(r.AllowedValues)
	r.ExcludedValues = copyStrings(r.ExcludedValues)
	return r
}

// decodeYAMLStrict decodes a YAML mapping into out, refusing a key out's type does not declare: a
// misspelled excludedOuIds in a resource file or an imported document would otherwise read as a
// section with no exclusions. out points at a struct whose fields all carry yaml tags. A merge key
// is accepted, and the keys of the mappings it merges in are held to the same check.
func decodeYAMLStrict(node *yaml.Node, out interface{}) error {
	if node.Kind == yaml.MappingNode {
		known := yamlFieldNames(reflect.TypeOf(out).Elem())
		if err := checkYAMLKeys(node, known, map[*yaml.Node]bool{}); err != nil {
			return err
		}
	}
	return node.Decode(out)
}

// checkYAMLKeys refuses a key of a YAML mapping that known does not hold, following merge keys into
// the mappings they merge in. visited guards against an anchor that merges itself in.
func checkYAMLKeys(mapping *yaml.Node, known map[string]bool, visited map[*yaml.Node]bool) error {
	if visited[mapping] {
		return nil
	}
	visited[mapping] = true
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		if isYAMLMergeKey(key) {
			if err := checkYAMLMergedKeys(value, known, visited); err != nil {
				return err
			}
			continue
		}
		if !known[key.Value] {
			return fmt.Errorf("line %d: unknown sharing policy field %q", key.Line, key.Value)
		}
	}
	return nil
}

// checkYAMLMergedKeys checks the keys of what a merge key merges in: a mapping, an alias of one, or a
// sequence of them. Any other value is left for the decoder to refuse.
func checkYAMLMergedKeys(node *yaml.Node, known map[string]bool, visited map[*yaml.Node]bool) error {
	switch node.Kind {
	case yaml.AliasNode:
		return checkYAMLMergedKeys(node.Alias, known, visited)
	case yaml.MappingNode:
		return checkYAMLKeys(node, known, visited)
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if err := checkYAMLMergedKeys(item, known, visited); err != nil {
				return err
			}
		}
	}
	return nil
}

// isYAMLMergeKey reports whether a mapping key is the merge key, as the yaml.v3 decoder reads one.
func isYAMLMergeKey(key *yaml.Node) bool {
	return key.Kind == yaml.ScalarNode && key.Value == "<<" &&
		(key.Tag == "" || key.Tag == "!" || key.ShortTag() == "!!merge")
}

// yamlFieldNames returns the keys a struct's yaml tags declare.
func yamlFieldNames(t reflect.Type) map[string]bool {
	names := make(map[string]bool, t.NumField())
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
		if name != "" && name != "-" {
			names[name] = true
		}
	}
	return names
}
