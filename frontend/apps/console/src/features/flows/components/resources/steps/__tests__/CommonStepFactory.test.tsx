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

import {render, screen} from '@testing-library/react';
import {ReactFlowProvider} from '@xyflow/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi} from 'vitest';
import type {Element} from '../../../../models/elements';
import type {Resources} from '../../../../models/resources';
import {StepTypes, StaticStepTypes, type Step} from '../../../../models/steps';
import CommonStepFactory from '../CommonStepFactory';

// Mock step components
vi.mock('../view/View', () => ({
  default: ({
    resources,
    data,
    enableSourceHandle,
  }: {
    resources: Step[];
    data: unknown;
    enableSourceHandle?: boolean;
  }) => (
    <div
      data-testid="view-step"
      data-resource-count={resources.length}
      data-has-data={!!data}
      data-enable-source-handle={String(enableSourceHandle ?? false)}
    >
      View Step
    </div>
  ),
}));

vi.mock('../rule/Rule', () => ({
  default: ({resources, data}: {resources: Step[]; data: unknown}) => (
    <div data-testid="rule-step" data-resource-count={resources.length} data-has-data={!!data}>
      Rule Step
    </div>
  ),
}));

vi.mock('../execution/Execution', () => ({
  default: ({resources, data}: {resources: Step[]; data: unknown}) => (
    <div data-testid="execution-step" data-resource-count={resources.length} data-has-data={!!data}>
      Execution Step
    </div>
  ),
}));

vi.mock('../end/End', () => ({
  default: ({resources, data}: {resources: Step[]; data: unknown}) => (
    <div data-testid="end-step" data-resource-count={resources.length} data-has-data={!!data}>
      End Step
    </div>
  ),
}));

vi.mock('../call/Call', () => ({
  default: ({resources, data}: {resources: Step[]; data: unknown}) => (
    <div data-testid="call-step" data-resource-count={resources.length} data-has-data={!!data}>
      Call Step
    </div>
  ),
}));

describe('CommonStepFactory', () => {
  const createWrapper = () => {
    function Wrapper({children}: {children: ReactNode}) {
      return <ReactFlowProvider>{children}</ReactFlowProvider>;
    }
    return Wrapper;
  };

  const createMockStep = (overrides: Partial<Step> = {}): Step =>
    ({
      id: 'step-1',
      type: StepTypes.View,
      category: 'STEP',
      config: {},
      ...overrides,
    }) as Step;

  const defaultNodeProps = {
    id: 'node-1',
    type: 'custom',
    data: {components: []},
    positionAbsoluteX: 0,
    positionAbsoluteY: 0,
    zIndex: 0,
    isConnectable: true,
    xPos: 0,
    yPos: 0,
    dragging: false,
    selected: false,
    dragHandle: undefined,
    sourcePosition: undefined,
    targetPosition: undefined,
    draggable: true,
    selectable: true,
    deletable: true,
  };

  describe('View Step', () => {
    it('should render View component for VIEW step type', () => {
      const viewStep = createMockStep({type: StepTypes.View});

      render(<CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[viewStep]} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('view-step')).toBeInTheDocument();
    });

    it('should pass resources array to View component', () => {
      const viewStep1 = createMockStep({id: 'step-1', type: StepTypes.View});
      const viewStep2 = createMockStep({id: 'step-2', type: StepTypes.View});

      render(<CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[viewStep1, viewStep2]} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('view-step')).toHaveAttribute('data-resource-count', '2');
    });

    it('should pass data to View component', () => {
      const viewStep = createMockStep({type: StepTypes.View});

      render(
        <CommonStepFactory
          {...defaultNodeProps}
          resourceId="resource-1"
          resources={[viewStep]}
          data={{components: [{id: 'comp-1'}]}}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('view-step')).toHaveAttribute('data-has-data', 'true');
    });

    it('should pass availableElements from allResources', () => {
      const viewStep = createMockStep({type: StepTypes.View});
      const allResources: Resources = {
        steps: [],
        elements: [{id: 'element-1'} as Element],
        widgets: [],
        templates: [],
        executors: [],
      };

      render(
        <CommonStepFactory
          {...defaultNodeProps}
          resourceId="resource-1"
          resources={[viewStep]}
          allResources={allResources}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('view-step')).toBeInTheDocument();
    });

    it('should pass onAddElement callback', () => {
      const viewStep = createMockStep({type: StepTypes.View});
      const onAddElement = vi.fn();

      render(
        <CommonStepFactory
          {...defaultNodeProps}
          resourceId="resource-1"
          resources={[viewStep]}
          onAddElement={onAddElement}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('view-step')).toBeInTheDocument();
    });

    it('should pass onAddElementToForm callback', () => {
      const viewStep = createMockStep({type: StepTypes.View});
      const onAddElementToForm = vi.fn();

      render(
        <CommonStepFactory
          {...defaultNodeProps}
          resourceId="resource-1"
          resources={[viewStep]}
          onAddElementToForm={onAddElementToForm}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('view-step')).toBeInTheDocument();
    });
  });

  describe('Rule Step', () => {
    it('should render Rule component for RULE step type', () => {
      const ruleStep = createMockStep({type: StepTypes.Rule});

      render(<CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[ruleStep]} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('rule-step')).toBeInTheDocument();
    });

    it('should pass resources to Rule component', () => {
      const ruleStep = createMockStep({type: StepTypes.Rule});

      render(<CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[ruleStep]} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('rule-step')).toHaveAttribute('data-resource-count', '1');
    });

    it('should pass data to Rule component', () => {
      const ruleStep = createMockStep({type: StepTypes.Rule});

      render(
        <CommonStepFactory
          {...defaultNodeProps}
          resourceId="resource-1"
          resources={[ruleStep]}
          data={{condition: 'true'}}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('rule-step')).toHaveAttribute('data-has-data', 'true');
    });
  });

  describe('Execution Step', () => {
    it('should render Execution component for EXECUTION step type', () => {
      const executionStep = createMockStep({type: StepTypes.Execution});

      render(<CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[executionStep]} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('execution-step')).toBeInTheDocument();
    });

    it('should pass resources to Execution component', () => {
      const executionStep = createMockStep({type: StepTypes.Execution});

      render(<CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[executionStep]} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('execution-step')).toHaveAttribute('data-resource-count', '1');
    });

    it('should pass data to Execution component', () => {
      const executionStep = createMockStep({type: StepTypes.Execution});

      render(
        <CommonStepFactory
          {...defaultNodeProps}
          resourceId="resource-1"
          resources={[executionStep]}
          data={{executor: 'test'}}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('execution-step')).toHaveAttribute('data-has-data', 'true');
    });
  });

  describe('End Step', () => {
    it('should render End component for END step type', () => {
      const endStep = createMockStep({type: StepTypes.End});

      render(<CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[endStep]} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('end-step')).toBeInTheDocument();
    });

    it('should pass resources to End component', () => {
      const endStep = createMockStep({type: StepTypes.End});

      render(<CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[endStep]} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('end-step')).toHaveAttribute('data-resource-count', '1');
    });

    it('should pass data to End component', () => {
      const endStep = createMockStep({type: StepTypes.End});

      render(
        <CommonStepFactory
          {...defaultNodeProps}
          resourceId="resource-1"
          resources={[endStep]}
          data={{status: 'complete'}}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('end-step')).toHaveAttribute('data-has-data', 'true');
    });
  });

  describe('Call Step', () => {
    it('should render Call component for CALL step type', () => {
      const callStep = createMockStep({type: StepTypes.Call});

      render(<CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[callStep]} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('call-step')).toBeInTheDocument();
      expect(screen.getByTestId('call-step')).toHaveAttribute('data-resource-count', '1');
    });

    it('should pass data to Call component', () => {
      const callStep = createMockStep({type: StepTypes.Call});

      render(
        <CommonStepFactory
          {...defaultNodeProps}
          resourceId="resource-1"
          resources={[callStep]}
          data={{flow: {ref: 'flow-x'}}}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('call-step')).toHaveAttribute('data-has-data', 'true');
    });
  });

  describe('Unknown Step Type', () => {
    it('should return null for unknown step type', () => {
      const unknownStep = createMockStep({type: 'UNKNOWN' as StepTypes});

      const {container} = render(
        <CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[unknownStep]} />,
        {wrapper: createWrapper()},
      );

      expect(container.firstChild).toBeNull();
    });
  });

  describe('Start Step', () => {
    it('should return null for START step type (not handled in factory)', () => {
      const startStep = createMockStep({type: StaticStepTypes.Start as unknown as StepTypes});

      const {container} = render(
        <CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[startStep]} />,
        {wrapper: createWrapper()},
      );

      expect(container.firstChild).toBeNull();
    });
  });

  describe('Default Props', () => {
    it('should work with undefined allResources', () => {
      const viewStep = createMockStep({type: StepTypes.View});

      render(<CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[viewStep]} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('view-step')).toBeInTheDocument();
    });

    it('should work with undefined onAddElement', () => {
      const viewStep = createMockStep({type: StepTypes.View});

      render(<CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[viewStep]} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('view-step')).toBeInTheDocument();
    });

    it('should work with undefined onAddElementToForm', () => {
      const viewStep = createMockStep({type: StepTypes.View});

      render(<CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[viewStep]} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('view-step')).toBeInTheDocument();
    });
  });

  describe('Rest Props Spreading', () => {
    it('should spread additional props to step components', () => {
      const viewStep = createMockStep({type: StepTypes.View});

      render(
        <CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[viewStep]} data-custom="value" />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('view-step')).toBeInTheDocument();
    });
  });

  describe('Display-only enableSourceHandle behavior', () => {
    it('should set enableSourceHandle=true when data has no components key', () => {
      const viewStep = createMockStep({type: StepTypes.View});

      render(<CommonStepFactory {...defaultNodeProps} resourceId="resource-1" resources={[viewStep]} data={{}} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('view-step')).toHaveAttribute('data-enable-source-handle', 'true');
    });

    it('should set enableSourceHandle=true when components has only DISPLAY elements', () => {
      const viewStep = createMockStep({type: StepTypes.View});

      render(
        <CommonStepFactory
          {...defaultNodeProps}
          resourceId="resource-1"
          resources={[viewStep]}
          data={{components: [{id: 'text-1', category: 'DISPLAY', type: 'TEXT'}]}}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('view-step')).toHaveAttribute('data-enable-source-handle', 'true');
    });

    it('should set enableSourceHandle=false when components has a top-level ACTION element', () => {
      const viewStep = createMockStep({type: StepTypes.View});

      render(
        <CommonStepFactory
          {...defaultNodeProps}
          resourceId="resource-1"
          resources={[viewStep]}
          data={{components: [{id: 'btn-1', category: 'ACTION', type: 'ACTION'}]}}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('view-step')).toHaveAttribute('data-enable-source-handle', 'false');
    });

    it('should set enableSourceHandle=false when components has a RESEND element', () => {
      const viewStep = createMockStep({type: StepTypes.View});

      render(
        <CommonStepFactory
          {...defaultNodeProps}
          resourceId="resource-1"
          resources={[viewStep]}
          data={{components: [{id: 'resend-1', category: 'DISPLAY', type: 'RESEND'}]}}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('view-step')).toHaveAttribute('data-enable-source-handle', 'false');
    });

    it('should set enableSourceHandle=false when ACTION is nested inside a BLOCK component', () => {
      const viewStep = createMockStep({type: StepTypes.View});

      render(
        <CommonStepFactory
          {...defaultNodeProps}
          resourceId="resource-1"
          resources={[viewStep]}
          data={{
            components: [
              {
                id: 'block-1',
                category: 'BLOCK',
                type: 'BLOCK',
                components: [{id: 'btn-1', category: 'ACTION', type: 'ACTION'}],
              },
            ],
          }}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('view-step')).toHaveAttribute('data-enable-source-handle', 'false');
    });
  });
});
