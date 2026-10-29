// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
)

// systemFlowFiles are the system flows: flows a service runs in process, such as revoking what a
// change invalidates. They are part of the build, never stored, and never listed.
//
//go:embed system/*.json
var systemFlowFiles embed.FS

// loadSystemFlows reads and validates the system flows in files, keyed by handle. Any fault in them is
// a fault in the build, so it is returned and fails startup.
func loadSystemFlows(ctx context.Context, files fs.FS, validator FlowValidatorInterface) (
	map[string]json.RawMessage, error) {
	names, err := fs.Glob(files, "system/*.json")
	if err != nil {
		return nil, err
	}
	flows := make(map[string]json.RawMessage, len(names))
	ids := make(map[string]bool, len(names))
	for _, name := range names {
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, fmt.Errorf("system flow %s: %w", name, err)
		}
		var flowDef FlowDefinition
		if err := json.Unmarshal(data, &flowDef); err != nil {
			return nil, fmt.Errorf("system flow %s: %w", name, err)
		}
		if flowDef.ID == "" {
			return nil, fmt.Errorf("system flow %s has no id", name)
		}
		if _, dup := flows[flowDef.Handle]; dup {
			return nil, fmt.Errorf("system flow %s reuses the handle %q", name, flowDef.Handle)
		}
		if ids[flowDef.ID] {
			return nil, fmt.Errorf("system flow %s reuses the id %q", name, flowDef.ID)
		}
		if svcErr := validator.ValidateSystemFlowDefinition(ctx, &flowDef); svcErr != nil {
			return nil, fmt.Errorf("system flow %s: %s: %s", name, svcErr.Code, svcErr.ErrorDescription.String())
		}
		flows[flowDef.Handle] = data
		ids[flowDef.ID] = true
	}
	return flows, nil
}
