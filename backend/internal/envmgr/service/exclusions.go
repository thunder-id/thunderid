/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package service

import (
	"context"
	"sort"

	"github.com/thunder-id/thunderid/internal/envmgr/bundle"
	"github.com/thunder-id/thunderid/internal/envmgr/diff"
	"github.com/thunder-id/thunderid/internal/envmgr/model"
)

// Holding a resource back is a standing decision, not a per-run one. A user who deselects something
// before promoting means "not this resource, here", and asking again on every run would make them
// repeat the decision until they eventually forgot and let it through. So the choice is recorded on
// the target environment and reapplied by default, and selecting the resource again clears it.

// changedKeys returns the resource keys the diff reports as actually changing.
func changedKeys(d diff.Diff) []string {
	keys := make([]string, 0, len(d.Changes))
	for _, c := range d.Changes {
		if c.Change != diff.Unchanged {
			keys = append(keys, c.Key)
		}
	}
	return keys
}

// defaultSelection is what runs when the caller expresses no preference: everything that changed,
// minus what was held back before.
func defaultSelection(d diff.Diff, excluded []string) []string {
	held := toSet(excluded)
	var selection []string
	for _, key := range changedKeys(d) {
		if !held[key] {
			selection = append(selection, key)
		}
	}
	return selection
}

// nextExclusions folds an explicit selection into the remembered ones.
//
// Only the resources on offer in this run are reconsidered: a key that did not change is neither
// held back nor released, so a decision made about it earlier survives a run that never showed it.
func nextExclusions(existing []string, d diff.Diff, selection []string) []string {
	held := toSet(existing)
	selected := toSet(selection)

	for _, key := range changedKeys(d) {
		if selected[key] {
			// Deliberately selected again, so it promotes from here on.
			delete(held, key)
		} else {
			held[key] = true
		}
	}

	out := make([]string, 0, len(held))
	for key := range held {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// withoutExcluded drops the held back resources from a bundle, so an apply pushes the same set the
// promotion agreed on.
func withoutExcluded(resources []bundle.Resource, excluded []string) []bundle.Resource {
	if len(excluded) == 0 {
		return resources
	}
	held := toSet(excluded)
	out := make([]bundle.Resource, 0, len(resources))
	for _, r := range resources {
		if !held[r.Key()] {
			out = append(out, r)
		}
	}
	return out
}

// rememberSelection stores the environment's exclusions when they changed.
func (s *Service) rememberSelection(ctx context.Context, env model.Environment, d diff.Diff,
	selection []string) error {
	next := nextExclusions(env.Excluded, d, selection)
	if sameKeys(env.Excluded, next) {
		return nil
	}
	env.Excluded = next
	env.UpdatedAt = s.now().UTC()
	return s.store.SaveEnvironment(ctx, env)
}

func toSet(keys []string) map[string]bool {
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	return set
}

func sameKeys(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
