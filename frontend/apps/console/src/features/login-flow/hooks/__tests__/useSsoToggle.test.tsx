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

import {act, renderHook} from '@testing-library/react';
import type {Edge, Node} from '@xyflow/react';
import type {MouseEvent as ReactMouseEvent} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import useSsoToggle from '../useSsoToggle';
import type {Resources} from '@/features/flows/models/resources';
import {ExecutionTypes, StaticStepTypes, StepTypes} from '@/features/flows/models/steps';

const {mockFlowConfig, mockSetIsVerboseMode, mockInteractionState, mockSetIsOpenResourcePropertiesPanel} = vi.hoisted(
  () => {
    const setIsVerboseMode = vi.fn();
    return {
      mockFlowConfig: {edgeStyle: 'default', isVerboseMode: true, setIsVerboseMode},
      mockInteractionState: {lastInteractedResource: undefined as unknown, lastInteractedStepId: ''},
      mockSetIsOpenResourcePropertiesPanel: vi.fn(),
      mockSetIsVerboseMode: setIsVerboseMode,
    };
  },
);

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: unknown) => (typeof options === 'string' ? options : key),
  }),
}));

vi.mock('@/features/flows/hooks/useFlowConfig', () => ({
  default: () => mockFlowConfig,
}));

vi.mock('@/features/flows/hooks/useInteractionState', () => ({
  default: () => mockInteractionState,
}));

vi.mock('@/features/flows/hooks/useUIPanelState', () => ({
  default: () => ({
    setIsOpenResourcePropertiesPanel: mockSetIsOpenResourcePropertiesPanel,
  }),
}));

const resources = {
  executors: [
    {
      data: {action: {executor: {name: ExecutionTypes.SSOCheck}, onFailure: '', onSuccess: '', type: 'EXECUTOR'}},
      display: {label: 'Check SSO Session'},
      resourceType: 'STEP',
      type: StepTypes.Execution,
    },
    {
      data: {action: {executor: {name: ExecutionTypes.Session}, onSuccess: '', type: 'EXECUTOR'}},
      display: {label: 'Save / Load Session'},
      resourceType: 'STEP',
      type: StepTypes.Execution,
    },
  ],
} as unknown as Resources;

function makeExecution(id: string, executorName: string): Node {
  return {
    data: {action: {executor: {name: executorName}, type: 'EXECUTOR'}},
    id,
    position: {x: 0, y: 0},
    type: StepTypes.Execution,
  };
}

function basicFlow(): {nodes: Node[]; edges: Edge[]} {
  return {
    edges: [
      {id: 'start-to-prompt', source: 'start', sourceHandle: 'start_NEXT', target: 'prompt'},
      {id: 'prompt-to-auth', source: 'prompt', sourceHandle: 'button_NEXT', target: 'credentials_auth'},
      {
        id: 'credentials_auth-to-assert',
        source: 'credentials_auth',
        sourceHandle: 'credentials_auth_NEXT',
        target: 'auth_assert',
      },
      {id: 'assert-to-end', source: 'auth_assert', sourceHandle: 'auth_assert_NEXT', target: 'end'},
    ],
    nodes: [
      {data: {}, id: 'start', position: {x: 0, y: 0}, type: StaticStepTypes.Start},
      {data: {components: []}, id: 'prompt', position: {x: 200, y: 0}, type: StepTypes.View},
      makeExecution('credentials_auth', 'CredentialsAuthExecutor'),
      makeExecution('auth_assert', ExecutionTypes.AuthAssert),
      {data: {}, id: 'end', position: {x: 900, y: 0}, type: StepTypes.End},
    ],
  };
}

function renderSsoToggle(overrides: {nodes?: Node[]; edges?: Edge[]} = {}) {
  const {nodes, edges} = basicFlow();
  const setNodes = vi.fn();
  const setEdges = vi.fn();
  const showInfo = vi.fn();
  const showSuccess = vi.fn();

  const hook = renderHook(() =>
    useSsoToggle({
      edges: overrides.edges ?? edges,
      nodes: overrides.nodes ?? nodes,
      resources,
      setEdges,
      setNodes,
      showInfo,
      showSuccess,
    }),
  );

  return {hook, setEdges, setNodes, showInfo, showSuccess};
}

describe('useSsoToggle', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFlowConfig.isVerboseMode = true;
    mockInteractionState.lastInteractedStepId = '';
    mockInteractionState.lastInteractedResource = undefined;
  });

  it('should derive a disabled SSO state for a flow without SSO checks', () => {
    const {hook} = renderSsoToggle();

    expect(hook.result.current.ssoState).toEqual({enabled: false, ssoCheckIds: []});
    expect(hook.result.current.joinResolution).toEqual({joinNodeId: 'auth_assert', status: 'ok'});
  });

  it('should insert the SSO pair, notify, and request focus on enable', () => {
    const {hook, setNodes, setEdges, showInfo} = renderSsoToggle();

    act(() => {
      hook.result.current.handleEnable();
    });

    expect(setNodes).toHaveBeenCalledTimes(1);
    expect(setEdges).toHaveBeenCalledTimes(1);
    expect(showInfo).toHaveBeenCalledTimes(1);

    const newNodes = setNodes.mock.calls[0][0] as Node[];
    expect(newNodes.some((node) => node.id.startsWith('sso_check_'))).toBe(true);
    expect(newNodes.some((node) => node.id.startsWith('session_'))).toBe(true);
    expect(hook.result.current.focusRequest?.ssoCheckId).toMatch(/^sso_check_/);
  });

  it('should force verbose mode on before inserting when it is off', () => {
    mockFlowConfig.isVerboseMode = false;
    const {hook} = renderSsoToggle();

    act(() => {
      hook.result.current.handleEnable();
    });

    expect(mockSetIsVerboseMode).toHaveBeenCalledWith(true);
  });

  it('should clear the focus request', () => {
    const {hook} = renderSsoToggle();

    act(() => {
      hook.result.current.handleEnable();
    });
    expect(hook.result.current.focusRequest).not.toBeNull();

    act(() => {
      hook.result.current.clearFocusRequest();
    });
    expect(hook.result.current.focusRequest).toBeNull();
  });

  describe('ambiguous join (placement mode)', () => {
    function ambiguousFlow(): {nodes: Node[]; edges: Edge[]} {
      const {nodes, edges} = basicFlow();
      return {
        edges: [
          ...edges,
          {id: 'prompt-to-assert2', source: 'prompt', sourceHandle: 'other_NEXT', target: 'auth_assert_2'},
        ],
        nodes: [...nodes, makeExecution('auth_assert_2', ExecutionTypes.AuthAssert)],
      };
    }

    it('should enter placement mode instead of guessing', () => {
      const {hook, setNodes} = renderSsoToggle(ambiguousFlow());

      act(() => {
        hook.result.current.handleEnable();
      });

      expect(setNodes).not.toHaveBeenCalled();
      expect(hook.result.current.placement.active).toBe(true);
      expect(hook.result.current.placement.candidateEdgeIds.length).toBeGreaterThan(0);
    });

    it('should apply the enable transformation when a candidate edge is clicked', () => {
      const {hook, setNodes} = renderSsoToggle(ambiguousFlow());

      act(() => {
        hook.result.current.handleEnable();
      });
      const [candidateEdgeId] = hook.result.current.placement.candidateEdgeIds;

      act(() => {
        hook.result.current.handleEdgeClick(
          {} as ReactMouseEvent,
          {
            id: candidateEdgeId,
            source: 'credentials_auth',
            target: 'auth_assert',
          } as Edge,
        );
      });

      expect(setNodes).toHaveBeenCalledTimes(1);
      expect(hook.result.current.placement.active).toBe(false);
    });

    it('should ignore clicks on non-candidate edges', () => {
      const {hook, setNodes} = renderSsoToggle(ambiguousFlow());

      act(() => {
        hook.result.current.handleEnable();
      });
      act(() => {
        hook.result.current.handleEdgeClick(
          {} as ReactMouseEvent,
          {
            id: 'start-to-prompt',
            source: 'start',
            target: 'prompt',
          } as Edge,
        );
      });

      expect(setNodes).not.toHaveBeenCalled();
      expect(hook.result.current.placement.active).toBe(true);
    });

    it('should cancel placement mode explicitly and via Escape', () => {
      const {hook} = renderSsoToggle(ambiguousFlow());

      act(() => {
        hook.result.current.handleEnable();
      });
      act(() => {
        hook.result.current.handleCancelPlacement();
      });
      expect(hook.result.current.placement.active).toBe(false);

      act(() => {
        hook.result.current.handleEnable();
      });
      act(() => {
        window.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape'}));
      });
      expect(hook.result.current.placement.active).toBe(false);
    });
  });

  describe('disable', () => {
    function ssoFlow(): {nodes: Node[]; edges: Edge[]} {
      const ssoCheck = makeExecution('sso_check_1', ExecutionTypes.SSOCheck);
      ssoCheck.data = {...ssoCheck.data, properties: {checkpointRef: 'session_1'}};
      const {nodes, edges} = basicFlow();
      return {
        edges: [
          {id: 'start-to-check', source: 'start', sourceHandle: 'start_NEXT', target: 'sso_check_1'},
          {id: 'check-to-session', source: 'sso_check_1', sourceHandle: 'sso_check_1_NEXT', target: 'session_1'},
          {id: 'check-fail-to-prompt', source: 'sso_check_1', sourceHandle: 'failure', target: 'prompt'},
          {id: 'session-to-assert', source: 'session_1', sourceHandle: 'session_1_NEXT', target: 'auth_assert'},
          ...edges.filter((edge) => edge.id !== 'start-to-prompt' && edge.id !== 'credentials_auth-to-assert'),
          {
            id: 'auth-to-session',
            source: 'credentials_auth',
            sourceHandle: 'credentials_auth_NEXT',
            target: 'session_1',
          },
        ],
        nodes: [...nodes, ssoCheck, makeExecution('session_1', ExecutionTypes.Session)],
      };
    }

    it('should open and close the confirmation dialog', () => {
      const {hook} = renderSsoToggle(ssoFlow());

      act(() => {
        hook.result.current.handleDisableRequest();
      });
      expect(hook.result.current.isConfirmDialogOpen).toBe(true);

      act(() => {
        hook.result.current.handleCloseConfirmDialog();
      });
      expect(hook.result.current.isConfirmDialogOpen).toBe(false);
    });

    it('should remove the SSO pair and notify on confirm', () => {
      const {hook, setNodes, setEdges, showSuccess} = renderSsoToggle(ssoFlow());

      act(() => {
        hook.result.current.handleDisableRequest();
      });
      act(() => {
        hook.result.current.handleConfirmDisable();
      });

      const remainingNodes = setNodes.mock.calls[0][0] as Node[];
      expect(remainingNodes.some((node) => node.id === 'sso_check_1')).toBe(false);
      expect(remainingNodes.some((node) => node.id === 'session_1')).toBe(false);
      expect(setEdges).toHaveBeenCalledTimes(1);
      expect(showSuccess).toHaveBeenCalledTimes(1);
      expect(hook.result.current.isConfirmDialogOpen).toBe(false);
    });

    it('should close the properties panel when it shows a removed node', () => {
      mockInteractionState.lastInteractedStepId = 'sso_check_1';
      const {hook} = renderSsoToggle(ssoFlow());

      act(() => {
        hook.result.current.handleConfirmDisable();
      });

      expect(mockSetIsOpenResourcePropertiesPanel).toHaveBeenCalledWith(false);
    });

    it('should leave the properties panel open when it shows an unrelated node', () => {
      mockInteractionState.lastInteractedStepId = 'prompt';
      const {hook} = renderSsoToggle(ssoFlow());

      act(() => {
        hook.result.current.handleConfirmDisable();
      });

      expect(mockSetIsOpenResourcePropertiesPanel).not.toHaveBeenCalled();
    });
  });
});
