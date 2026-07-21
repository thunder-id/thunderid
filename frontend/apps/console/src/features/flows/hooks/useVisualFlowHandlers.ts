/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {
  type Connection,
  type Edge,
  type Node,
  type OnConnect,
  type OnEdgesDelete,
  type OnNodesDelete,
  MarkerType,
  addEdge,
  getConnectedEdges,
  getIncomers,
  getOutgoers,
  useReactFlow,
} from '@xyflow/react';
import {useRef} from 'react';
import useFlowConfig from './useFlowConfig';
import useFlowPlugins from './useFlowPlugins';
import VisualFlowConstants from '../constants/VisualFlowConstants';
import type {Element} from '../models/elements';
import {ElementTypes} from '../models/elements';
import type {StepData} from '../models/steps';

/**
 * Props interface for useVisualFlowHandlers hook
 */
export interface UseVisualFlowHandlersProps {
  onEdgeResolve?: (connection: Connection, nodes: Node[]) => Edge;
  setEdges: React.Dispatch<React.SetStateAction<Edge[]>>;
}

/**
 * Return type for useVisualFlowHandlers hook
 */
export interface UseVisualFlowHandlersReturn {
  handleConnect: OnConnect;
  handleNodesDelete: OnNodesDelete<Node>;
  handleEdgesDelete: OnEdgesDelete<Edge>;
}

/**
 * Hook that provides stable callbacks for VisualFlow handlers.
 *
 * - Returns stable function references that NEVER change
 * - ALL dependencies are stored in refs and read at call time
 * - The actual work only happens when the function is called (on user interaction)
 * - Minimal work during render - just ref assignments
 */
const useVisualFlowHandlers = (props: UseVisualFlowHandlersProps): UseVisualFlowHandlersReturn => {
  // Get references from ReactFlow hooks
  const reactFlowInstance = useReactFlow();
  const {edgeStyle} = useFlowConfig();

  const {emitEdgeDelete} = useFlowPlugins();

  // Store ALL dependencies in refs - updated every render
  const depsRef = useRef({
    props,
    reactFlowInstance,
    edgeStyle,
    emitEdgeDelete,
  });

  // Update refs every render (minimal overhead - just assignment)
  depsRef.current = {
    props,
    reactFlowInstance,
    edgeStyle,
    emitEdgeDelete,
  };

  // Store stable references to handler functions
  const handlersRef = useRef<UseVisualFlowHandlersReturn | null>(null);

  // Create handlers only once - reads ALL deps from ref at call time
  handlersRef.current ??= {
    handleConnect: (connection: Connection): void => {
      const {props: currentProps, reactFlowInstance: rf, edgeStyle: currentEdgeStyle} = depsRef.current;
      const {onEdgeResolve, setEdges} = currentProps;
      const {getNodes, updateNodeData} = rf;

      const currentNodes = getNodes();

      // If the edge originates from a rich-text action handle, copy the target node's id
      // into the rich-text component's `action.ref`. Author-facing UX: they draw the edge
      // and the "Connected step" field auto-populates.
      const nextSuffix = VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX;
      if (
        connection.sourceHandle &&
        connection.sourceHandle.endsWith(nextSuffix) &&
        connection.source &&
        connection.target
      ) {
        const componentId = connection.sourceHandle.slice(0, -nextSuffix.length);
        const sourceNode = currentNodes.find((n) => n.id === connection.source);
        const components = (sourceNode?.data as StepData | undefined)?.components;
        if (components) {
          const targetId: string = connection.target;
          let changed = false;

          const updateRichTextRef = (elements: Element[]): Element[] => {
            let localChanged = false;
            const next = elements.map((el) => {
              if (el.id === componentId && el.type === ElementTypes.RichText) {
                const withAction = el as Element & {action?: {ref?: string}};
                if (withAction.action !== undefined && withAction.action.ref !== targetId) {
                  localChanged = true;
                  return {...el, action: {...withAction.action, ref: targetId}} as Element;
                }
              }

              if (el.components) {
                const nestedNext = updateRichTextRef(el.components);
                if (nestedNext !== el.components) {
                  localChanged = true;
                  return {...el, components: nestedNext};
                }
              }

              return el;
            });

            if (localChanged) {
              changed = true;
              return next;
            }

            return elements;
          };

          const nextComponents = updateRichTextRef(components);
          if (changed) {
            updateNodeData(connection.source, (node) => ({
              ...(node.data as StepData),
              components: nextComponents,
            }));
          }
        }
      }

      if (onEdgeResolve) {
        const newEdge: Edge = onEdgeResolve(connection, currentNodes);
        setEdges((eds: Edge[]) => addEdge(newEdge, eds));
      } else {
        setEdges((eds: Edge[]) =>
          addEdge(
            {
              ...connection,
              type: currentEdgeStyle,
              markerEnd: {type: MarkerType.ArrowClosed},
            },
            eds,
          ),
        );
      }
    },

    handleNodesDelete: (deleted: Node[]): void => {
      const {props: currentProps, reactFlowInstance: rf, edgeStyle: currentEdgeStyle} = depsRef.current;
      const {setEdges} = currentProps;
      const {getNodes} = rf;

      const currentNodes = getNodes();

      setEdges((latestEdges: Edge[]) =>
        deleted.reduce((acc: Edge[], node: Node) => {
          const incomers: Node[] = getIncomers(node, currentNodes, acc);
          const outgoers: Node[] = getOutgoers(node, currentNodes, acc);
          const connectedEdges: Edge[] = getConnectedEdges([node], acc);

          const remainingEdges: Edge[] = acc.filter((edge: Edge) => !connectedEdges.includes(edge));

          const createdEdges: Edge[] = incomers.flatMap(({id: source}: Node) =>
            outgoers.map(({id: target}: Node) => ({
              id: `${source}-->${target}`,
              source,
              target,
              type: currentEdgeStyle,
              markerEnd: {type: MarkerType.ArrowClosed},
            })),
          );

          return [...remainingEdges, ...createdEdges];
        }, latestEdges),
      );
    },

    handleEdgesDelete: (deletedEdges: Edge[]): void => {
      depsRef.current.emitEdgeDelete(deletedEdges);
    },
  };

  return handlersRef.current;
};

export default useVisualFlowHandlers;
