// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/userlookup"
)

// actorNamer puts names to the users actors record.
type actorNamer interface {
	fill(ctx context.Context, actors ...*Actor)
}

// actorNames puts names to actors from the trusted issuer's user API, through the user lookup.
type actorNames struct {
	lookup *userlookup.Lookup
}

// newActorNames names actors through a lookup read from the server configuration.
func newActorNames() *actorNames {
	return &actorNames{lookup: userlookup.New()}
}

// fill puts a name to each actor that has none, when a lookup is configured. An actor already named
// is left as it is.
func (n *actorNames) fill(ctx context.Context, actors ...*Actor) {
	if !n.lookup.Enabled() {
		return
	}
	ids := make([]string, 0, len(actors))
	for _, actor := range actors {
		if actor != nil && actor.ID != "" && actor.Name == "" {
			ids = append(ids, actor.ID)
		}
	}
	names := n.lookup.Names(ctx, ids...)
	for _, actor := range actors {
		if actor != nil && actor.Name == "" {
			actor.Name = names[actor.ID]
		}
	}
}
