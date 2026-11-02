// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {EdgeTypes, NodeProps, NodeTypes} from '@xyflow/react';
import {useEffect, useMemo, useRef} from 'react';
import BaseEdge from '../components/react-flow-overrides/BaseEdge';
import {CommonStaticStepFactory} from '../components/resources/steps/CommonStaticStepFactory';
import ExecutionStack from '../components/resources/steps/execution/ExecutionStack';
import StepFactory from '../components/resources/steps/StepFactory';
import FlowConstants from '../constants/FlowConstants';
import type {Element} from '../models/elements';
import type {Resources} from '../models/resources';
import {StaticStepTypes, type Step} from '../models/steps';
import {EXECUTION_STACK_NODE_TYPE} from '../utils/compactGraphTransforms';

/**
 * Props for the useNodeTypes hook.
 */
export interface UseNodeTypesProps {
  /** Array of step definitions. */
  steps: Step[] | undefined;
  /** All flow builder resources. */
  resources: Resources;
  /** Callback to add an element to a view. */
  onAddElementToView: (element: Element, viewId: string) => void;
  /** Callback to add an element to a form. */
  onAddElementToForm: (element: Element, formId: string) => void;
}

/**
 * Return type for the useNodeTypes hook.
 */
export interface UseNodeTypesReturn {
  /** Node types for React Flow. */
  nodeTypes: NodeTypes;
  /** Edge types for React Flow. */
  edgeTypes: EdgeTypes;
}

/**
 * Hook to generate node and edge types for React Flow.
 * Uses refs to avoid unnecessary re-renders when callbacks change.
 *
 * @param props - Configuration options for the hook.
 * @returns Node and edge types for React Flow.
 */
const useNodeTypes = (props: UseNodeTypesProps): UseNodeTypesReturn => {
  const {steps, resources, onAddElementToView, onAddElementToForm} = props;

  // Use refs to store callbacks and resources to avoid nodeTypes recreation
  const onAddElementToViewRef = useRef(onAddElementToView);
  const onAddElementToFormRef = useRef(onAddElementToForm);
  const resourcesRef = useRef(resources);
  const stepsByTypeRef = useRef<Record<string, Step[]>>({});

  // Update refs when data changes (doesn't trigger re-render)
  useEffect(() => {
    onAddElementToViewRef.current = onAddElementToView;
    onAddElementToFormRef.current = onAddElementToForm;
    resourcesRef.current = resources;
  }, [onAddElementToView, onAddElementToForm, resources]);

  // Organize steps by type
  useEffect(() => {
    if (steps) {
      stepsByTypeRef.current = steps.reduce((acc: Record<string, Step[]>, step: Step) => {
        if (!acc[step.type]) {
          acc[step.type] = [];
        }
        acc[step.type].push(step);
        return acc;
      }, {});
    }
  }, [steps]);

  /**
   * Node types for React Flow.
   * Uses refs to read data at render time, avoiding recreation when callbacks change.
   */
  const nodeTypes = useMemo((): NodeTypes => {
    // Get unique step types from steps (only to determine which types we need)
    const stepTypeSet = new Set(steps?.map((s) => s.type) ?? []);

    const stepNodes: NodeTypes = Array.from(stepTypeSet).reduce((acc: NodeTypes, stepType: string) => {
      // Create a stable component that reads from refs at render time
      acc[stepType] = (nodeProps: NodeProps) => (
        // @ts-expect-error NodeProps doesn't include all required properties but they're provided at runtime
        <StepFactory
          resourceId={nodeProps.id}
          resources={stepsByTypeRef.current[stepType] ?? []}
          allResources={resourcesRef.current}
          onAddElement={(element: Element) => onAddElementToViewRef.current?.(element, nodeProps.id)}
          onAddElementToForm={(element: Element, formId: string) => onAddElementToFormRef.current?.(element, formId)}
          {...nodeProps}
        />
      );
      return acc;
    }, {});

    const staticStepNodes: NodeTypes = Object.values(StaticStepTypes).reduce(
      (acc: NodeTypes, type: StaticStepTypes) => {
        acc[type] = (nodeProps: NodeProps) => <CommonStaticStepFactory {...nodeProps} type={type} />;

        return acc;
      },
      {},
    );

    return {
      ...staticStepNodes,
      ...stepNodes,
      // Synthetic display node for collapsed executor runs in compact mode.
      [EXECUTION_STACK_NODE_TYPE]: (nodeProps: NodeProps) => <ExecutionStack {...nodeProps} />,
    };
    // IMPORTANT: Only depend on the step types array, not resources
    // The actual data is accessed via refs at render time
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [steps?.map((s) => s.type).join(',')]);

  /**
   * Edge types for React Flow.
   * Register BaseEdge which uses SmartStepEdge for intelligent routing around nodes.
   */
  const edgeTypes: EdgeTypes = useMemo(
    () => ({
      default: BaseEdge,
      smoothstep: BaseEdge,
      step: BaseEdge,
      [FlowConstants.DEFAULT_EDGE_TYPE]: BaseEdge,
    }),
    [],
  );

  return {
    nodeTypes,
    edgeTypes,
  };
};

export default useNodeTypes;
