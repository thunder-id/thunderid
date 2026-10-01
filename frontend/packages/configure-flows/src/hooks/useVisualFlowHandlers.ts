// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

/** Matches a `data-action-ref` attribute and captures its delimiter and value. */
const ACTION_REF_SENTINEL = /(\sdata-action-ref\s*=\s*)(?:"([^"]*)"|'([^']*)')/gi;

/**
 * Rewrites the `data-action-ref` sentinels inside a rich text's label so they keep matching the
 * component's `action.ref`. The adapter only dispatches when the clicked anchor's sentinel equals
 * the ref, so the two values must always move together.
 *
 * A label with no sentinel at all is returned unchanged: the adapter dispatches on any anchor
 * click in that case. In a label with several sentinels, one carrying some other ref belongs to a
 * different link and is left alone. A lone sentinel is unambiguously this component's, so it is
 * rewritten even when it does not match `previousRef` — that is how a label left divergent by an
 * earlier build, where the ref moved without the sentinel, heals.
 *
 * With several sentinels and no ref to match them against, only the first is rewritten: it is the
 * anchor the properties panel derives the ref from, and rewriting them all would point unrelated
 * links at this action.
 *
 * @param label - The rich text's label, which may be HTML.
 * @param previousRef - The ref the label's sentinels are expected to carry, if any.
 * @param nextRef - The ref to write.
 * @returns The label with its matching sentinels rewritten.
 */
const syncLabelActionRef = (label: string, previousRef: string | undefined, nextRef: string): string => {
  const isAmbiguous = (label.match(ACTION_REF_SENTINEL)?.length ?? 0) > 1;
  const hasPreviousRef = previousRef !== undefined && previousRef !== '';
  let seen = 0;

  return label.replace(
    ACTION_REF_SENTINEL,
    (match: string, prefix: string, doubleQuoted?: string, singleQuoted?: string) => {
      const currentRef = doubleQuoted ?? singleQuoted;

      seen += 1;

      if (isAmbiguous) {
        if (!hasPreviousRef) {
          return seen === 1 ? `${prefix}"${nextRef}"` : match;
        }

        if (currentRef !== previousRef) {
          return match;
        }
      }

      return `${prefix}"${nextRef}"`;
    },
  );
};

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

      // If the edge originates from a rich-text action handle, name the action after the target
      // node. The ref is a lookup key the runtime matches against the clicked anchor's
      // `data-action-ref`, so the label's sentinel is rewritten in the same pass. Changing one
      // without the other leaves a link that renders but dispatches nothing.
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
                const withAction = el as Element & {action?: {ref?: string} | null; label?: string};
                // A toggled-off link stores `action: null`, which has no ref to move.
                if (withAction.action !== undefined && withAction.action !== null) {
                  const nextLabel: string | undefined =
                    typeof withAction.label === 'string'
                      ? syncLabelActionRef(withAction.label, withAction.action.ref, targetId)
                      : undefined;

                  // Reconnecting to the step the ref already names must still heal a label whose
                  // sentinel has drifted, otherwise the most direct repair gesture is a no-op.
                  if (
                    withAction.action.ref !== targetId ||
                    (nextLabel !== undefined && nextLabel !== withAction.label)
                  ) {
                    localChanged = true;

                    return {
                      ...el,
                      action: {...withAction.action, ref: targetId},
                      ...(nextLabel !== undefined ? {label: nextLabel} : {}),
                    } as Element;
                  }
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
