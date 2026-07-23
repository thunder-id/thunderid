/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import Call from '../Call';
import type {Step} from '@/features/flows/models/steps';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, fallback?: string) => fallback ?? _key,
  }),
}));

const mockNavigate = vi.fn();
vi.mock('react-router', () => ({
  useNavigate: () => mockNavigate,
}));

const mockUseGetFlows = vi.fn<() => unknown>();
vi.mock('@/features/flows/api/useGetFlows', () => ({
  default: (): unknown => mockUseGetFlows(),
}));

const mockDeleteElements = vi.fn();
const mockUseNodeId = vi.fn<() => string | null>(() => 'call-node-id');

vi.mock('@xyflow/react', () => ({
  // eslint-disable-next-line react/require-default-props
  Handle: ({type, position, id}: {type: string; position: string; id?: string}) => (
    <div data-testid={`handle-${type}-${id ?? position}`} data-position={position} data-handle-id={id} />
  ),
  Position: {Left: 'left', Right: 'right', Top: 'top', Bottom: 'bottom'},
  useNodeId: () => mockUseNodeId(),
  useReactFlow: () => ({deleteElements: mockDeleteElements}),
}));

const mockSetLastInteractedResource = vi.fn();
const mockSetLastInteractedStepId = vi.fn();
vi.mock('@/features/flows/hooks/useInteractionState', () => ({
  default: () => ({
    setLastInteractedResource: mockSetLastInteractedResource,
    setLastInteractedStepId: mockSetLastInteractedStepId,
  }),
}));

const mockSetIsOpenResourcePropertiesPanel = vi.fn();
vi.mock('@/features/flows/hooks/useUIPanelState', () => ({
  default: () => ({setIsOpenResourcePropertiesPanel: mockSetIsOpenResourcePropertiesPanel}),
}));

const paletteResource: Step = {
  id: 'call-palette',
  type: 'CALL',
  category: 'WORKFLOW',
  resourceType: 'STEP',
  display: {label: 'Flow'},
} as unknown as Step;

const renderCall = (data?: Record<string, unknown>) => {
  const props = {
    resources: [paletteResource],
    data: data as never,
    resourceId: 'call-palette',
    id: 'call-node-id',
    type: 'CALL',
    position: {x: 0, y: 0},
    selected: false,
    isConnectable: true,
  } as unknown as Parameters<typeof Call>[0];
  return render(<Call {...props} />);
};

describe('Call', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseNodeId.mockReturnValue('call-node-id');
    mockUseGetFlows.mockReturnValue({data: {flows: []}, isLoading: false, error: null});
  });

  describe('Rendering', () => {
    it('falls back to the default "Call flow" label when no palette resource is provided', () => {
      mockUseNodeId.mockReturnValue(null);
      const props = {
        resources: [],
        data: {} as never,
        resourceId: 'call-palette',
        id: 'call-node-id',
        type: 'CALL',
        position: {x: 0, y: 0},
        selected: false,
        isConnectable: true,
      } as unknown as Parameters<typeof Call>[0];
      render(<Call {...props} />);
      expect(screen.getAllByText(/^flow$/i).length).toBeGreaterThan(0);
    });

    it('records the last interacted resource when the card body is clicked', () => {
      renderCall({flow: {ref: 'flow-abc'}});
      fireEvent.click(screen.getByTestId('call-node-content'));
      expect(mockSetLastInteractedStepId).toHaveBeenCalledWith('call-node-id');
      expect(mockSetLastInteractedResource).toHaveBeenCalled();
      // handleCardClick does not open the properties panel
      expect(mockSetIsOpenResourcePropertiesPanel).not.toHaveBeenCalled();
    });

    it('shows the flow name in the body when the referenced flow is resolvable', () => {
      mockUseGetFlows.mockReturnValue({
        data: {flows: [{id: 'flow-123', name: 'My Flow', flowType: 'AUTHENTICATION'}]},
        isLoading: false,
        error: null,
      });
      renderCall({flow: {ref: 'flow-123'}});
      expect(screen.getByTestId('call-node-flow-ref')).toHaveTextContent('My Flow');
    });

    it('shows the "Select a flow to invoke" placeholder when no flow ref is set', () => {
      renderCall({});
      expect(screen.getByTestId('call-node-flow-ref')).toHaveTextContent(/select a flow/i);
    });

    it('shows the step id in the header', () => {
      renderCall({});
      expect(screen.getByText('call-node-id')).toBeInTheDocument();
    });

    it('shows the palette label in the header without a node context', () => {
      mockUseNodeId.mockReturnValue(null);
      renderCall({});
      expect(screen.getAllByText('Flow').length).toBeGreaterThan(0);
    });
  });

  describe('Handles', () => {
    it('renders the target handle on the left', () => {
      renderCall({flow: {ref: 'f1'}});
      const target = screen.getByTestId('handle-target-left');
      expect(target).toHaveAttribute('data-position', 'left');
    });

    it('renders a right-positioned success source handle', () => {
      renderCall({flow: {ref: 'f1'}});
      const handles = screen.getAllByTestId(/^handle-source-/);
      const right = handles.find((h) => h.getAttribute('data-position') === 'right');
      expect(right).toBeTruthy();
    });

    it('renders the failure source handle on the bottom with id "failure"', () => {
      renderCall({flow: {ref: 'f1'}});
      const failure = screen.getByTestId('handle-source-failure');
      expect(failure).toHaveAttribute('data-position', 'bottom');
      expect(failure).toHaveAttribute('data-handle-id', 'failure');
    });
  });

  describe('Configure button', () => {
    it('opens the properties panel and sets the interacted resource on click', () => {
      const {container} = renderCall({flow: {ref: 'flow-abc'}});
      const configBtn = container.querySelectorAll('button')[0];
      expect(configBtn).toBeTruthy();
      fireEvent.click(configBtn);
      expect(mockSetLastInteractedStepId).toHaveBeenCalledWith('call-node-id');
      expect(mockSetIsOpenResourcePropertiesPanel).toHaveBeenCalledWith(true);
      expect(mockSetLastInteractedResource).toHaveBeenCalled();
    });
  });

  describe('Delete button', () => {
    it('calls deleteElements with the node id', () => {
      const {container} = renderCall({flow: {ref: 'flow-abc'}});
      const deleteBtn = container.querySelectorAll('button')[1];
      expect(deleteBtn).toBeTruthy();
      fireEvent.click(deleteBtn);
      expect(mockDeleteElements).toHaveBeenCalledWith({nodes: [{id: 'call-node-id'}]});
    });

    it('does not call deleteElements when nodeId is empty', () => {
      mockUseNodeId.mockReturnValue('');
      const {container} = renderCall({});
      const deleteBtn = container.querySelectorAll('button')[1];
      fireEvent.click(deleteBtn);
      expect(mockDeleteElements).not.toHaveBeenCalled();
    });
  });

  describe('Open referenced flow', () => {
    it('is disabled when no flow is referenced', () => {
      renderCall({});
      expect(screen.getByTestId('call-open-referenced-flow')).toBeDisabled();
    });

    it('is disabled when the referenced flow id does not resolve to a known flow', () => {
      mockUseGetFlows.mockReturnValue({data: {flows: []}, isLoading: false, error: null});
      renderCall({flow: {ref: 'unknown-flow'}});
      expect(screen.getByTestId('call-open-referenced-flow')).toBeDisabled();
    });

    it('navigates to the referenced AUTHENTICATION flow after confirmation', () => {
      mockUseGetFlows.mockReturnValue({
        data: {flows: [{id: 'flow-a', name: 'Flow A', flowType: 'AUTHENTICATION'}]},
        isLoading: false,
        error: null,
      });
      renderCall({flow: {ref: 'flow-a'}});
      fireEvent.click(screen.getByTestId('call-open-referenced-flow'));
      fireEvent.click(screen.getByRole('button', {name: /continue/i}));
      expect(mockNavigate).toHaveBeenCalledWith('/flows/signin/flow-a');
    });

    it('navigates to the referenced REGISTRATION flow after confirmation', () => {
      mockUseGetFlows.mockReturnValue({
        data: {flows: [{id: 'flow-r', name: 'Reg', flowType: 'REGISTRATION'}]},
        isLoading: false,
        error: null,
      });
      renderCall({flow: {ref: 'flow-r'}});
      fireEvent.click(screen.getByTestId('call-open-referenced-flow'));
      fireEvent.click(screen.getByRole('button', {name: /continue/i}));
      expect(mockNavigate).toHaveBeenCalledWith('/flows/registration/flow-r');
    });

    it('navigates to the referenced RECOVERY flow after confirmation', () => {
      mockUseGetFlows.mockReturnValue({
        data: {flows: [{id: 'flow-rec', name: 'Rec', flowType: 'RECOVERY'}]},
        isLoading: false,
        error: null,
      });
      renderCall({flow: {ref: 'flow-rec'}});
      fireEvent.click(screen.getByTestId('call-open-referenced-flow'));
      fireEvent.click(screen.getByRole('button', {name: /continue/i}));
      expect(mockNavigate).toHaveBeenCalledWith('/flows/recovery/flow-rec');
    });

    it('does not offer opening a referenced SIGNOUT flow', () => {
      // Sign-out flows cannot be call targets, so there is no route to open one.
      mockUseGetFlows.mockReturnValue({
        data: {flows: [{id: 'flow-so', name: 'Sign Out', flowType: 'SIGNOUT'}]},
        isLoading: false,
        error: null,
      });
      renderCall({flow: {ref: 'flow-so'}});
      expect(screen.getByTestId('call-open-referenced-flow')).toBeDisabled();
    });

    it('does not navigate when the user cancels the confirmation dialog', () => {
      mockUseGetFlows.mockReturnValue({
        data: {flows: [{id: 'flow-a', name: 'Flow A', flowType: 'AUTHENTICATION'}]},
        isLoading: false,
        error: null,
      });
      renderCall({flow: {ref: 'flow-a'}});
      fireEvent.click(screen.getByTestId('call-open-referenced-flow'));
      fireEvent.click(screen.getByRole('button', {name: /cancel/i}));
      expect(mockNavigate).not.toHaveBeenCalled();
    });
  });
});
