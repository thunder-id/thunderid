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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package service

import (
	"fmt"
	"sort"

	"github.com/thunder-id/thunderid/internal/envmgr/model"
)

// The promotion graph is a DAG: environments are nodes and an edge means "this environment can
// promote into that one". Modeling it as a graph rather than a line lets a single environment fan
// out (one dev feeding several regional prods) or fan in (a prod gated behind both qa and staging).
// Cycles are rejected, because a cycle would let configuration be promoted in a loop with no
// well-defined ordering.

// buildAdjacency maps every environment to its outgoing edges.
//
// The rank fallback is all-or-nothing: it only applies when no environment declares any edge, in
// which case the environments form a linear chain in rank order. As soon as one edge is declared the
// graph is taken literally, and an environment without edges is a sink. Falling back per environment
// instead would quietly invent edges that can close a cycle, for example a declared dev -> prod plus
// an invented prod -> dev.
func buildAdjacency(envs []model.Environment) map[string][]string {
	adjacency := make(map[string][]string, len(envs))

	declared := false
	for _, env := range envs {
		if len(env.PromotesTo) > 0 {
			declared = true
			break
		}
	}

	if declared {
		for _, env := range envs {
			adjacency[env.ID] = env.PromotesTo
		}
		return adjacency
	}

	ordered := make([]model.Environment, len(envs))
	copy(ordered, envs)
	sortEnvironments(ordered)
	for i, env := range ordered {
		if i+1 < len(ordered) {
			adjacency[env.ID] = []string{ordered[i+1].ID}
		} else {
			adjacency[env.ID] = nil
		}
	}
	return adjacency
}

// validateGraph checks that every edge points at a known environment, that nothing points at itself,
// and that the graph has no cycle.
func validateGraph(envs []model.Environment) error {
	known := make(map[string]bool, len(envs))
	for _, env := range envs {
		known[env.ID] = true
	}
	for _, env := range envs {
		for _, target := range env.PromotesTo {
			if target == env.ID {
				return fmt.Errorf("%w: %q cannot promote to itself", ErrValidation, env.Name)
			}
			if !known[target] {
				return fmt.Errorf("%w: %q promotes to unknown environment %q", ErrValidation, env.Name, target)
			}
		}
	}
	if cycle := findCycle(buildAdjacency(envs)); cycle != "" {
		return fmt.Errorf("%w: promotion path would form a cycle through %s", ErrValidation, cycle)
	}
	return nil
}

// findCycle reports a node involved in a cycle, or "" when the graph is acyclic. It is a depth-first
// search coloring nodes white (unvisited), grey (on the current path) and black (fully explored); an
// edge back to a grey node closes a cycle.
func findCycle(adjacency map[string][]string) string {
	const (
		white = 0
		grey  = 1
		black = 2
	)
	state := make(map[string]int, len(adjacency))

	ids := make([]string, 0, len(adjacency))
	for id := range adjacency {
		ids = append(ids, id)
	}
	sort.Strings(ids) // deterministic reporting

	var visit func(string) string
	visit = func(id string) string {
		switch state[id] {
		case grey:
			return id
		case black:
			return ""
		}
		state[id] = grey
		for _, next := range adjacency[id] {
			if _, ok := adjacency[next]; !ok {
				continue
			}
			if found := visit(next); found != "" {
				return found
			}
		}
		state[id] = black
		return ""
	}

	for _, id := range ids {
		if found := visit(id); found != "" {
			return found
		}
	}
	return ""
}

// canPromote reports whether an edge from -> to exists in the graph.
func canPromote(envs []model.Environment, from, to string) bool {
	for _, target := range buildAdjacency(envs)[from] {
		if target == to {
			return true
		}
	}
	return false
}

// topologicalOrder returns the environments so that every environment precedes the ones it promotes
// into. Ties are broken by rank then name, keeping the output stable. A graph with a cycle cannot be
// ordered, so the input order is returned unchanged; validateGraph rejects those on write.
func topologicalOrder(envs []model.Environment) []model.Environment {
	adjacency := buildAdjacency(envs)
	indegree := make(map[string]int, len(envs))
	byID := make(map[string]model.Environment, len(envs))
	for _, env := range envs {
		byID[env.ID] = env
		if _, seen := indegree[env.ID]; !seen {
			indegree[env.ID] = 0
		}
	}
	for _, edges := range adjacency {
		for _, target := range edges {
			if _, ok := byID[target]; ok {
				indegree[target]++
			}
		}
	}

	ready := make([]model.Environment, 0, len(envs))
	for _, env := range envs {
		if indegree[env.ID] == 0 {
			ready = append(ready, env)
		}
	}
	sortEnvironments(ready)

	out := make([]model.Environment, 0, len(envs))
	for len(ready) > 0 {
		next := ready[0]
		ready = ready[1:]
		out = append(out, next)

		var freed []model.Environment
		for _, target := range adjacency[next.ID] {
			if _, ok := byID[target]; !ok {
				continue
			}
			indegree[target]--
			if indegree[target] == 0 {
				freed = append(freed, byID[target])
			}
		}
		sortEnvironments(freed)
		ready = append(ready, freed...)
		sortEnvironments(ready)
	}

	if len(out) != len(envs) {
		return envs // cycle; leave the caller's order alone
	}
	return out
}

func sortEnvironments(envs []model.Environment) {
	sort.SliceStable(envs, func(i, j int) bool {
		if envs[i].Rank != envs[j].Rank {
			return envs[i].Rank < envs[j].Rank
		}
		return envs[i].Name < envs[j].Name
	})
}
