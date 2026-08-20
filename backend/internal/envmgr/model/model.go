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

// Package model holds the core domain types for the environment-management service.
package model

import (
	"time"
)

// Origin describes how a version came to exist.
type Origin string

const (
	// OriginCaptured means the version was captured from an environment's control-plane source.
	OriginCaptured Origin = "captured"
	// OriginPromoted means the version was produced by promoting from a lower environment.
	OriginPromoted Origin = "promoted"
	// OriginReverted means the version was produced by reverting to an earlier version.
	OriginReverted Origin = "reverted"
	// OriginUploaded means the version's payload was supplied directly by the caller.
	OriginUploaded Origin = "uploaded"
)

// Target identifies a data plane that a version is applied to.
//
// The data plane is named, not addressed. It dials the control plane and holds that connection open,
// so the control plane reaches it over that channel rather than over its management API: there is no
// URL to call and no credential to hold. DataPlaneID is the id the data plane presents when it
// connects, and BaseURL is kept only to show an operator where that deployment serves.
type Target struct {
	DataPlaneID string `json:"dataPlaneId"`
	BaseURL     string `json:"baseUrl,omitempty"`
}

// DataPlaneStatus reports whether a data plane is connected to this control plane. It is part of the
// environment as read, not as stored: an operator about to promote needs to know the destination can
// be reached, and nothing can be applied to a data plane that is not.
type DataPlaneStatus struct {
	Connected bool      `json:"connected"`
	LastSeen  time.Time `json:"lastSeen,omitempty"`
}

// Environment is a node in the promotion graph, bound to one data-plane target.
//
// An environment is a resource of an organization rather than a deployment of its own: the
// organization has a single workspace that every environment captures from, and an environment holds
// the versions, variables and secrets that reach its own data plane.
type Environment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Rank orders environments for display, lowest first. It no longer defines the promotion path.
	Rank int `json:"rank"`
	// PromotesTo lists the environments this one can promote into: the outgoing edges of the
	// promotion graph, which is a DAG. An environment can fan out to several (a shared dev promoting
	// to both eu-prod and us-prod) and can be fanned into by several (a prod gated behind both qa and
	// staging). When empty, the edge falls back to the next environment by rank, which keeps a simple
	// linear chain working without declaring edges.
	PromotesTo []string `json:"promotesTo,omitempty"`
	Target     Target   `json:"target"`
	// ManagedByControlPlane marks the one environment the control plane administers directly, rather
	// than only promotes into. Editing configuration in the organization's workspace is editing this
	// environment; every other environment receives that configuration when it is promoted and
	// applied.
	//
	// It decides where a credential created in the workspace is issued. A credential is created once,
	// but each environment holds its own, and sending it everywhere would set the credential running
	// in production from a change made while developing. It goes here alone; the others receive theirs
	// when one is set against them deliberately.
	//
	// Exactly one environment of an organization holds this. Typically it is the development
	// environment, but nothing requires that: with none marked, the lowest rank stands in, being the
	// bottom of the promotion chain and where work starts.
	ManagedByControlPlane bool `json:"managedByControlPlane,omitempty"`
	// Excluded lists resource keys a user chose not to promote into this environment. The choice is
	// remembered rather than asked again on every run: a resource held back once stays held back until
	// it is deliberately selected again, at which point it is dropped from this list and promotes from
	// then on. A key here is skipped by both promotion and apply.
	Excluded []string `json:"excluded,omitempty"`
	// AppliedSeq is the version sequence last applied to Target (0 when nothing has been applied).
	AppliedSeq int `json:"appliedSeq"`
	// SecretNames is what this environment's data plane last reported holding, and SecretNamesAt when
	// it said so. It is recorded because only the control plane pod holding that data plane's
	// connection can ask: any other pod answers from this instead of failing. Names only, never
	// values.
	SecretNames   []string  `json:"secretNames,omitempty"`
	SecretNamesAt time.Time `json:"secretNamesAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Version is an immutable snapshot of an environment's declarative configuration. The parameterized
// resource YAML and the externalized variable values are stored alongside the metadata.
type Version struct {
	Seq         int               `json:"seq"`
	EnvID       string            `json:"envId"`
	Origin      Origin            `json:"origin"`
	ParentSeq   int               `json:"parentSeq,omitempty"`
	SourceEnvID string            `json:"sourceEnvId,omitempty"`
	SourceSeq   int               `json:"sourceSeq,omitempty"`
	Note        string            `json:"note,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	Resources   string            `json:"resources,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	// SecretKeys are the placeholders backed by a secret. Their values are deliberately absent: an
	// apply sends a ${KEY} placeholder for each, leaving the data plane to supply the real value, so
	// secrets never pass through this service.
	SecretKeys []string `json:"secretKeys,omitempty"`
}
