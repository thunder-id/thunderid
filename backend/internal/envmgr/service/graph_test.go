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
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/envmgr/model"
)

func env(id, name string, rank int, promotesTo ...string) model.Environment {
	return model.Environment{ID: id, Name: name, Rank: rank, PromotesTo: promotesTo}
}

func TestValidateGraphAcceptsFanOutAndFanIn(t *testing.T) {
	// dev fans out to two regional qa environments, which both fan in to a single prod.
	envs := []model.Environment{
		env("dev", "dev", 1, "qa-eu", "qa-us"),
		env("qa-eu", "qa-eu", 2, "prod"),
		env("qa-us", "qa-us", 2, "prod"),
		env("prod", "prod", 3),
	}
	if err := validateGraph(envs); err != nil {
		t.Fatalf("expected a valid DAG, got %v", err)
	}
}

func TestValidateGraphRejectsCycle(t *testing.T) {
	envs := []model.Environment{
		env("a", "a", 1, "b"),
		env("b", "b", 2, "c"),
		env("c", "c", 3, "a"),
	}
	err := validateGraph(envs)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected a cycle rejection, got %v", err)
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestValidateGraphRejectsSelfEdgeAndUnknownTarget(t *testing.T) {
	if err := validateGraph([]model.Environment{env("a", "a", 1, "a")}); err == nil {
		t.Fatal("expected a self-edge rejection")
	}
	if err := validateGraph([]model.Environment{env("a", "a", 1, "ghost")}); err == nil {
		t.Fatal("expected an unknown-target rejection")
	}
}

func TestEdgesFallBackToRankWhenUndeclared(t *testing.T) {
	envs := []model.Environment{env("a", "a", 1), env("b", "b", 2), env("c", "c", 3)}
	adjacency := buildAdjacency(envs)

	if len(adjacency["a"]) != 1 || adjacency["a"][0] != "b" {
		t.Fatalf("a should fall back to b, got %v", adjacency["a"])
	}
	if len(adjacency["c"]) != 0 {
		t.Fatalf("the last environment should have no successor, got %v", adjacency["c"])
	}
}

func TestTopologicalOrderPutsSourcesBeforeTargets(t *testing.T) {
	// Ranks deliberately disagree with the edges, so only the graph can produce a correct order.
	envs := []model.Environment{
		env("prod", "prod", 1, ""),
		env("dev", "dev", 9, "prod"),
	}
	envs[0].PromotesTo = nil

	ordered := topologicalOrder(envs)
	if len(ordered) != 2 || ordered[0].ID != "dev" || ordered[1].ID != "prod" {
		t.Fatalf("expected dev before prod, got %v", []string{ordered[0].ID, ordered[1].ID})
	}
}

func TestCanPromoteFollowsEdges(t *testing.T) {
	envs := []model.Environment{
		env("dev", "dev", 1, "qa"),
		env("qa", "qa", 2, "prod"),
		env("prod", "prod", 3),
	}
	if !canPromote(envs, "dev", "qa") {
		t.Fatal("dev -> qa should be allowed")
	}
	if canPromote(envs, "dev", "prod") {
		t.Fatal("dev -> prod skips qa and must not be allowed")
	}
}

func TestPromoteRejectsEnvironmentsWithNoEdge(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	a, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "a", Rank: intp(1), Target: model.Target{DataPlaneID: "a"},
	})
	b, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "b", Rank: intp(2), Target: model.Target{DataPlaneID: "b"},
	})
	c, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "c", Rank: intp(3), Target: model.Target{DataPlaneID: "c"},
	})
	// Declare a -> b only, leaving c unreachable from a.
	if _, err := svc.UpdateEnvironmentEdges(context.Background(), a.ID, []string{b.ID}); err != nil {
		t.Fatalf("set edges: %v", err)
	}
	if _, err := svc.UpdateEnvironmentEdges(context.Background(), b.ID, []string{}); err != nil {
		t.Fatalf("set edges: %v", err)
	}
	_, _ = svc.UploadVersion(context.Background(), a.ID, bundleOf("app-a"), nil, "v1")

	if _, err := svc.Promote(context.Background(),
		PromoteInput{FromEnvID: a.ID, ToEnvID: c.ID}); !errors.Is(err, ErrNoPromotionPath) {
		t.Fatalf("expected ErrNoPromotionPath, got %v", err)
	}
	if _, err := svc.Promote(context.Background(), PromoteInput{FromEnvID: a.ID, ToEnvID: b.ID}); err != nil {
		t.Fatalf("a -> b should be allowed, got %v", err)
	}
	// The reverse edge is a demotion and stays allowed.
	if _, err := svc.Promote(context.Background(), PromoteInput{FromEnvID: b.ID, ToEnvID: a.ID}); err != nil {
		t.Fatalf("demotion b -> a should be allowed, got %v", err)
	}
}

func TestUpdateEnvironmentEdgesRejectsCycle(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	a, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "a", Rank: intp(1), Target: model.Target{DataPlaneID: "a"},
	})
	b, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "b", Rank: intp(2), Target: model.Target{DataPlaneID: "b"},
	})
	if _, err := svc.UpdateEnvironmentEdges(context.Background(), a.ID, []string{b.ID}); err != nil {
		t.Fatalf("a -> b: %v", err)
	}
	if _, err := svc.UpdateEnvironmentEdges(context.Background(), b.ID, []string{a.ID}); err == nil {
		t.Fatal("b -> a closes a cycle and must be rejected")
	}
}

func TestSummariesExposeGraphEdges(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	dev, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "dev", Rank: intp(1), Target: model.Target{DataPlaneID: "dev"},
	})
	prod, _ := svc.CreateEnvironment(context.Background(), CreateEnvironmentInput{
		Name: "prod", Rank: intp(2), Target: model.Target{DataPlaneID: "prod"},
	})
	if _, err := svc.UpdateEnvironmentEdges(context.Background(), dev.ID, []string{prod.ID}); err != nil {
		t.Fatalf("set edges: %v", err)
	}

	summaries, err := svc.ListEnvironmentSummaries(context.Background())
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if summaries[0].ID != dev.ID {
		t.Fatalf("dev should come first, got %s", summaries[0].Name)
	}
	if len(summaries[0].PromotesToResolved) != 1 || summaries[0].PromotesToResolved[0] != prod.ID {
		t.Fatalf("dev should promote to prod, got %v", summaries[0].PromotesToResolved)
	}
	if len(summaries[1].PromotedFrom) != 1 || summaries[1].PromotedFrom[0] != dev.ID {
		t.Fatalf("prod should be promoted from dev, got %v", summaries[1].PromotedFrom)
	}
}
