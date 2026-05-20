/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package core

import (
	"encoding/json"
	"errors"

	"github.com/thunder-id/thunderid/internal/flow/common"
)

// GraphInterface defines the graph structure
type GraphInterface interface {
	GetID() string
	GetType() common.FlowType
	AddNode(node NodeInterface) error
	GetNode(nodeID string) (NodeInterface, bool)
	AddEdge(fromNodeID, toNodeID string) error
	RemoveEdge(fromNodeID, toNodeID string) error
	GetNodes() map[string]NodeInterface
	SetNodes(nodes map[string]NodeInterface)
	GetEdges() map[string][]string
	SetEdges(edges map[string][]string)
	GetStartNodeID() string
	GetStartNode() (NodeInterface, error)
	SetStartNode(startNodeID string) error
	ToJSON() (string, error)
	HasSegments() bool
	GetSegments() []Segment
	SetSegments(segments []Segment)
	GetSegmentByID(segmentID string) *Segment
	GetSegmentByStartNode(nodeID string) *Segment
}

// graph implements the GraphInterface for the flow execution
type graph struct {
	id          string
	_type       common.FlowType
	nodes       map[string]NodeInterface
	edges       map[string][]string
	startNodeID string
	segments    []Segment
}

// GetID returns the unique ID of the graph
func (g *graph) GetID() string {
	return g.id
}

// GetType returns the type of the graph
func (g *graph) GetType() common.FlowType {
	return g._type
}

// AddNode adds a node to the graph
func (g *graph) AddNode(node NodeInterface) error {
	if node == nil {
		return errors.New("node cannot be nil")
	}

	g.nodes[node.GetID()] = node
	return nil
}

// GetNode retrieves a node by its ID
func (g *graph) GetNode(nodeID string) (NodeInterface, bool) {
	if node, exists := g.nodes[nodeID]; exists {
		return node, true
	}
	return nil, false
}

// AddEdge adds an edge from one node to another
func (g *graph) AddEdge(fromNodeID, toNodeID string) error {
	if fromNodeID == "" || toNodeID == "" {
		return errors.New("fromNodeID and toNodeID cannot be empty")
	}
	fromNode, exists := g.nodes[fromNodeID]
	if !exists {
		return errors.New("node with fromNodeID does not exist")
	}
	toNode, exists := g.nodes[toNodeID]
	if !exists {
		return errors.New("node with toNodeID does not exist")
	}

	fromNode.AddNextNode(toNodeID)
	toNode.AddPreviousNode(fromNodeID)

	if _, exists := g.edges[fromNodeID]; !exists {
		g.edges[fromNodeID] = []string{}
	}
	g.edges[fromNodeID] = append(g.edges[fromNodeID], toNodeID)
	return nil
}

// RemoveEdge removes an edge from one node to another
func (g *graph) RemoveEdge(fromNodeID, toNodeID string) error {
	if fromNodeID == "" || toNodeID == "" {
		return errors.New("fromNodeID and toNodeID cannot be empty")
	}
	fromNode, exists := g.nodes[fromNodeID]
	if !exists {
		return errors.New("node with fromNodeID does not exist")
	}
	toNode, exists := g.nodes[toNodeID]
	if !exists {
		return errors.New("node with toNodeID does not exist")
	}

	fromNode.RemoveNextNode(toNodeID)
	toNode.RemovePreviousNode(fromNodeID)

	if edges, exists := g.edges[fromNodeID]; exists {
		for i, edge := range edges {
			if edge == toNodeID {
				g.edges[fromNodeID] = append(edges[:i], edges[i+1:]...)
				break
			}
		}
	}

	return nil
}

// GetNodes returns all nodes in the graph
func (g *graph) GetNodes() map[string]NodeInterface {
	return g.nodes
}

// SetNodes sets the nodes for the graph
func (g *graph) SetNodes(nodes map[string]NodeInterface) {
	if nodes == nil {
		g.nodes = make(map[string]NodeInterface)
	} else {
		g.nodes = nodes
	}
}

// GetEdges returns all edges in the graph
func (g *graph) GetEdges() map[string][]string {
	return g.edges
}

// SetEdges sets the edges for the graph
func (g *graph) SetEdges(edges map[string][]string) {
	if edges == nil {
		g.edges = make(map[string][]string)
	} else {
		g.edges = edges
	}
}

// GetStartNodeID returns the start node ID of the graph
func (g *graph) GetStartNodeID() string {
	return g.startNodeID
}

// GetStartNode retrieves the start node of the graph
func (g *graph) GetStartNode() (NodeInterface, error) {
	if g.startNodeID == "" {
		return nil, errors.New("start node not set for the graph")
	}
	node, exists := g.nodes[g.startNodeID]
	if !exists {
		return nil, errors.New("start node does not exist in the graph")
	}
	return node, nil
}

// SetStartNode sets the start node ID for the graph
func (g *graph) SetStartNode(startNodeID string) error {
	node, exists := g.nodes[startNodeID]
	if !exists {
		return errors.New("node with startNodeID does not exist")
	}
	g.startNodeID = startNodeID
	node.SetAsStartNode()

	return nil
}

// HasSegments returns true if the graph has multiple segments (i.e., contains display-only prompt nodes).
func (g *graph) HasSegments() bool {
	return len(g.segments) > 1
}

// GetSegments returns all segments in the graph.
func (g *graph) GetSegments() []Segment {
	return g.segments
}

// SetSegments sets the segments for the graph.
func (g *graph) SetSegments(segments []Segment) {
	g.segments = segments
}

// GetSegmentByID retrieves a segment by its ID.
func (g *graph) GetSegmentByID(segmentID string) *Segment {
	for i := range g.segments {
		if g.segments[i].ID == segmentID {
			return &g.segments[i]
		}
	}
	return nil
}

// GetSegmentByStartNode returns the segment whose start node matches the given node ID.
func (g *graph) GetSegmentByStartNode(nodeID string) *Segment {
	for i := range g.segments {
		if g.segments[i].StartNodeID == nodeID {
			return &g.segments[i]
		}
	}
	return nil
}

// ToJSON converts the graph to a JSON string representation
func (g *graph) ToJSON() (string, error) {
	type JSONInputs struct {
		Ref        string   `json:"ref,omitempty"`
		Identifier string   `json:"identifier"`
		Type       string   `json:"type"`
		Required   bool     `json:"required"`
		Options    []string `json:"options,omitempty"`
	}

	type JSONCondition struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	type JSONNode struct {
		ID                 string         `json:"id"`
		Type               string         `json:"type"`
		IsStartNode        bool           `json:"isStartNode,omitempty"`
		IsFinalNode        bool           `json:"isFinalNode,omitempty"`
		NextNodeIDList     []string       `json:"nextNodeIds"`
		PreviousNodeIDList []string       `json:"previousNodeIds"`
		Inputs             []JSONInputs   `json:"inputs,omitempty"`
		Executor           string         `json:"executor,omitempty"`
		Condition          *JSONCondition `json:"condition,omitempty"`
	}

	type JSONGraph struct {
		ID          string              `json:"id"`
		Nodes       map[string]JSONNode `json:"nodes"`
		Edges       map[string][]string `json:"edges"`
		StartNodeID string              `json:"startNodeId"`
	}

	jsonGraph := JSONGraph{
		ID:          g.id,
		Nodes:       make(map[string]JSONNode),
		Edges:       g.edges,
		StartNodeID: g.startNodeID,
	}

	// Convert nodes to JSONNode
	for id, node := range g.nodes {
		jsonNode := JSONNode{
			ID:                 id,
			Type:               string(node.GetType()),
			IsStartNode:        node.IsStartNode(),
			IsFinalNode:        node.IsFinalNode(),
			NextNodeIDList:     node.GetNextNodeList(),
			PreviousNodeIDList: node.GetPreviousNodeList(),
		}

		// Set executor name and inputs if the node is executor-backed
		if executableNode, ok := node.(ExecutorBackedNodeInterface); ok {
			executorName := executableNode.GetExecutorName()
			if executorName != "" {
				jsonNode.Executor = executorName
			}

			inputs := executableNode.GetInputs()
			if len(inputs) > 0 {
				jsonNode.Inputs = make([]JSONInputs, len(inputs))
				for i, input := range inputs {
					jsonNode.Inputs[i] = JSONInputs{
						Ref:        input.Ref,
						Identifier: input.Identifier,
						Type:       input.Type,
						Required:   input.Required,
						Options:    input.Options,
					}
				}
			}
		}

		// Set condition if present
		if condition := node.GetCondition(); condition != nil {
			jsonNode.Condition = &JSONCondition{
				Key:   condition.Key,
				Value: condition.Value,
			}
		}

		jsonGraph.Nodes[id] = jsonNode
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(jsonGraph)
	if err != nil {
		return "", errors.New("failed to marshal graph to JSON: " + err.Error())
	}

	return string(jsonBytes), nil
}
