/**
 * Copyright (c) 2023-2025, WSO2 LLC. (https://www.wso2.com).
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

import {useNodeId} from '@xyflow/react';
import {memo, useMemo, type ReactElement} from 'react';
import ExecutionMinimal from './ExecutionMinimal';
import ValidationErrorBoundary from '../../../validation-panel/ValidationErrorBoundary';
import type {CommonStepFactoryPropsInterface} from '../CommonStepFactory';
import View from '../view/View';
import VisualFlowConstants from '@/features/flows/constants/VisualFlowConstants';
import useInteractionState from '@/features/flows/hooks/useInteractionState';
import type {Element} from '@/features/flows/models/elements';
import {ResourceTypes} from '@/features/flows/models/resources';
import {type StepAction, type Step, StepCategories} from '@/features/flows/models/steps';

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

  const executorName = (data?.action as StepAction | undefined)?.executor?.name ?? 'Executor';
  // Get display metadata from data (set by resolveStepMetadata)
  const displayFromData = data?.display as
    | {
        label?: string;
        image?: string;
        preserveImageColor?: boolean;
        description?: string;
        showOnResourcePanel?: boolean;
        outcomes?: {success?: string; failure?: string; incomplete?: string};
      }
    | undefined;

  const hasComponents = useMemo(() => {
    const components = (data?.components as Element[]) ?? [];
    return components.length > 0;
  }, [data?.components]);

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
    <ValidationErrorBoundary resource={resource}>
      {hasComponents ? (
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
