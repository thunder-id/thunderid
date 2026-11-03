// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// propertyVariables is the property of a VARIABLES node, in the graph, that carries the variable declarations.
const propertyVariables = "variables"

// runtimeKeyPrefix prefixes the runtime data key a variable's value is kept under, as a JSON encoded value.
const runtimeKeyPrefix = "var."

// variablesNode is a VARIABLES node. It resolves the variables of the flow into the flow's context, where the
// {{var(name)}} placeholders of the steps after it read them, and goes on to its onSuccess node. It asks the user for
// nothing and shows nothing.
//
// The node carries every variable the flow declares, on all its VARIABLES nodes, not only the ones it declares
// itself. Each time one runs it resolves the whole set again, so a value that depends on something that changed
// since the previous one, such as the organization unit the flow is running for, is current from there on.
type variablesNode struct {
	*node
	onSuccess string
	logger    *log.Logger
}

var _ RepresentationNodeInterface = (*variablesNode)(nil)

// newVariablesNode creates a new variables node. The properties carry the variable declarations.
func newVariablesNode(id string, properties map[string]interface{}, isStartNode, isFinalNode bool) NodeInterface {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	return &variablesNode{
		node: &node{
			id:               id,
			_type:            common.NodeTypeVariables,
			properties:       properties,
			isStartNode:      isStartNode,
			isFinalNode:      isFinalNode,
			nextNodeList:     []string{},
			previousNodeList: []string{},
		},
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "VariablesNode"),
			log.String(log.LoggerKeyNodeID, id)),
	}
}

// Execute resolves every variable of the flow into the runtime data and goes on to the onSuccess node.
func (n *variablesNode) Execute(ctx *providers.NodeContext) (*common.NodeResponse, *tidcommon.ServiceError) {
	logger := n.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	defs, err := n.definitions()
	if err != nil {
		logger.Error(ctx.Context, "The variables are not readable", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	values, err := n.runtimeValues(defs)
	if err != nil {
		logger.Error(ctx.Context, "The default of a variable is not readable", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return &common.NodeResponse{
		Status:         common.NodeStatusComplete,
		RuntimeData:    values,
		AdditionalData: make(map[string]string),
		NextNodeID:     n.onSuccess,
	}, nil
}

// VariableRuntimeValues resolves every variable a VARIABLES node declares into the runtime data entries a flow reads
// them from: the value the organization unit holds in raws where it holds one, and the declared default otherwise. So
// a flow shared with an organization unit runs on what that unit holds for its variables, and resolving again for
// another unit starts from the defaults and not from what the first one held. A node that is not a VARIABLES node
// declares none.
func VariableRuntimeValues(node NodeInterface, raws map[string]string) (map[string]string, error) {
	n, ok := node.(*variablesNode)
	if !ok {
		return nil, nil
	}
	defs, err := n.definitions()
	if err != nil {
		return nil, err
	}
	values, err := n.runtimeValues(defs)
	if err != nil {
		return nil, err
	}
	for _, def := range defs {
		raw, set := raws[def.Identifier]
		if !set {
			continue
		}
		enc, err := n.encodeRuntime(dataTypeOf(def), raw)
		if err != nil {
			return nil, fmt.Errorf("variable %q: its value %w", def.Identifier, err)
		}
		values[runtimeKey(def.Identifier)] = enc
	}
	return values, nil
}

// UnsetConnectionVariables lists the connection variables a VARIABLES node has resolved to no connection, in the order
// the node declares them. A node that is not a VARIABLES node has none.
func UnsetConnectionVariables(node NodeInterface, runtimeData map[string]string) []string {
	n, ok := node.(*variablesNode)
	if !ok {
		return nil
	}
	defs, err := n.definitions()
	if err != nil {
		return nil
	}
	var unset []string
	for _, def := range defs {
		if def.Type != providers.InputTypeConnectionSelect {
			continue
		}
		if enc, set := runtimeData[runtimeKey(def.Identifier)]; !set || enc == `""` {
			unset = append(unset, def.Identifier)
		}
	}
	return unset
}

// GetOnSuccess returns the ID of the node the flow goes on to.
func (n *variablesNode) GetOnSuccess() string {
	return n.onSuccess
}

// SetOnSuccess sets the ID of the node the flow goes on to.
func (n *variablesNode) SetOnSuccess(nodeID string) {
	n.onSuccess = nodeID
}

// definitions reads the variable declarations the node carries in its properties. It returns none when the property
// is absent.
func (n *variablesNode) definitions() ([]providers.VariableDefinition, error) {
	raw, ok := n.GetProperties()[propertyVariables]
	if !ok || raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("the variables are not readable: %w", err)
	}
	var defs []providers.VariableDefinition
	if err := json.Unmarshal(data, &defs); err != nil {
		return nil, fmt.Errorf("the variables are malformed: %w", err)
	}
	return defs, nil
}

// runtimeValues encodes every variable's default, keyed by its runtime data key.
func (n *variablesNode) runtimeValues(defs []providers.VariableDefinition) (map[string]string, error) {
	out := make(map[string]string, len(defs))
	for _, def := range defs {
		raw, err := DefaultRaw(def)
		if err != nil {
			return nil, fmt.Errorf("variable %q: its default %w", def.Identifier, err)
		}
		enc, err := n.encodeRuntime(dataTypeOf(def), raw)
		if err != nil {
			return nil, fmt.Errorf("variable %q: its default %w", def.Identifier, err)
		}
		out[runtimeKey(def.Identifier)] = enc
	}
	return out, nil
}

// encodeRuntime encodes a raw value as the JSON a runtime data entry holds, so a placeholder resolves to a value of
// the variable's own type: a boolean to a boolean, a number to a number, anything else to a string.
func (n *variablesNode) encodeRuntime(dataType, raw string) (string, error) {
	switch dataType {
	case variableDataTypeBoolean:
		return strconv.FormatBool(raw == "true"), nil
	case variableDataTypeNumber:
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return "", errors.New("is not a number")
		}
		return strconv.FormatFloat(n, 'f', -1, 64), nil
	default:
		data, err := json.Marshal(raw)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}
