// Copyright 2023-2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useNodeId} from '@xyflow/react';
import {memo, useMemo, type ReactElement} from 'react';
import ExecutionCompact from './ExecutionCompact';
import ExecutionMinimal from './ExecutionMinimal';
import VisualFlowConstants from '../../../../constants/VisualFlowConstants';
import useFlowConfig from '../../../../hooks/useFlowConfig';
import useInteractionState from '../../../../hooks/useInteractionState';
import type {Element} from '../../../../models/elements';
import {ResourceTypes} from '../../../../models/resources';
import {type StepAction, type Step, StepCategories} from '../../../../models/steps';
import ValidationErrorBoundary from '../../../validation-panel/ValidationErrorBoundary';
import type {CommonStepFactoryPropsInterface} from '../CommonStepFactory';
import View from '../view/View';

/**
 * Props interface of {@link Execution}
 */
export type ExecutionPropsInterface = CommonStepFactoryPropsInterface;

/**
 * Execution Node component.
 *
 * - Uses useMemo for resource object creation
 * - Conditional rendering: View for executors with components, ExecutionMinimal for simple ones
 * - Memoized component checks to avoid unnecessary re-renders
 *
 * @param props - Props injected to the component.
 * @returns Execution node component.
 */
function Execution({data, resources}: ExecutionPropsInterface): ReactElement | null {
  const stepId: string | null = useNodeId();
  const {setLastInteractedResource, setLastInteractedStepId} = useInteractionState();
  const {isVerboseMode} = useFlowConfig();

  const executorName = (data?.['action'] as StepAction | undefined)?.executor?.name ?? 'Executor';
  // Get display metadata from data (set by resolveStepMetadata)
  const displayFromData = data?.['display'] as
    | {
        label?: string;
        image?: string;
        preserveImageColor?: boolean;
        description?: string;
        showOnResourcePanel?: boolean;
        outcomes?: {success?: string; failure?: string; incomplete?: string};
      }
    | undefined;

  const dataComponents = data?.['components'];

  const hasComponents = useMemo(() => {
    const components = (dataComponents as Element[]) ?? [];
    return components.length > 0;
  }, [dataComponents]);

  const resource = useMemo(
    () =>
      ({
        id: stepId ?? '',
        type: 'EXECUTION',
        category: StepCategories.Workflow,
        resourceType: ResourceTypes.Step,
        data,
        display: {
          label: displayFromData?.label ?? executorName,
          image: displayFromData?.image ?? '',
          preserveImageColor: displayFromData?.preserveImageColor,
          description: displayFromData?.description,
          showOnResourcePanel: displayFromData?.showOnResourcePanel ?? false,
          outcomes: displayFromData?.outcomes,
        },
      }) as Step,
    [stepId, data, executorName, displayFromData],
  );

  // Selecting the executor surfaces its properties panel (the provider opens the
  // panel for EXECUTION resources). Reachable from both the header itself and its
  // Cog button, mirroring ExecutionMinimal.
  const handleSelect = useMemo(
    () => () => {
      if (stepId) {
        setLastInteractedStepId(stepId);
      }
      setLastInteractedResource(resource);
    },
    [stepId, resource, setLastInteractedStepId, setLastInteractedResource],
  );

  return (
    // The compact chip is a circle, so the boundary is rounded to match it
    // instead of boxing it in the default rounded rectangle.
    <ValidationErrorBoundary borderRadius={isVerboseMode ? undefined : '50%'} resource={resource}>
      {!isVerboseMode ? (
        <ExecutionCompact resource={resource} />
      ) : hasComponents ? (
        <View
          heading={executorName}
          data={data}
          resources={resources}
          enableSourceHandle
          deletable={false}
          configurable
          droppableAllowedTypes={VisualFlowConstants.FLOW_BUILDER_STATIC_CONTENT_ALLOWED_RESOURCE_TYPES}
          onActionPanelClick={handleSelect}
          onConfigure={handleSelect}
        />
      ) : (
        <ExecutionMinimal resource={resource} />
      )}
    </ValidationErrorBoundary>
  );
}

// Memoize Execution to prevent re-renders when parent re-renders with same props
const MemoizedExecution = memo(Execution, (prevProps, nextProps) => {
  // Re-render if data changed
  if (prevProps.data !== nextProps.data) {
    return false;
  }
  // Re-render if resources changed
  if (prevProps.resources !== nextProps.resources) {
    return false;
  }
  return true;
});

export default MemoizedExecution;
