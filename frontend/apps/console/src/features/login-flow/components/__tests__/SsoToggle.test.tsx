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

import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {SsoTogglePropsInterface} from '../SsoToggle';
import SsoToggle from '../SsoToggle';
import type {Resource} from '@/features/flows/models/resources';

const mockFitView = vi.fn(() => Promise.resolve(true));
const mockUpdateNodeInternals = vi.fn();
const mockSetLastInteractedResource = vi.fn();
const mockSetLastInteractedStepId = vi.fn();

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: unknown) => (typeof options === 'string' ? options : key),
  }),
}));

vi.mock('@xyflow/react', () => ({
  useReactFlow: () => ({fitView: mockFitView}),
  useUpdateNodeInternals: () => mockUpdateNodeInternals,
}));

vi.mock('@/features/flows/hooks/useInteractionState', () => ({
  default: () => ({
    setLastInteractedResource: mockSetLastInteractedResource,
    setLastInteractedStepId: mockSetLastInteractedStepId,
  }),
}));

const baseProps: SsoTogglePropsInterface = {
  focusRequest: null,
  isReadOnly: false,
  joinResolution: {joinNodeId: 'authorization_check', status: 'ok'},
  onDisableRequest: vi.fn(),
  onEnable: vi.fn(),
  onFocusHandled: vi.fn(),
  placement: {active: false, candidateEdgeIds: []},
  ssoState: {enabled: false, ssoCheckIds: []},
};

function getSwitch(): HTMLInputElement {
  return screen.getByTestId('sso-toggle').querySelector('input')!;
}

describe('SsoToggle', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render the label and description', () => {
    render(<SsoToggle {...baseProps} />);

    expect(screen.getByText('Enable SSO')).toBeInTheDocument();
    expect(screen.getByText('Reuse an active session to skip sign-in')).toBeInTheDocument();
  });

  it('should call onEnable when toggled on', () => {
    render(<SsoToggle {...baseProps} />);

    fireEvent.click(getSwitch());

    expect(baseProps.onEnable).toHaveBeenCalledTimes(1);
    expect(baseProps.onDisableRequest).not.toHaveBeenCalled();
  });

  it('should call onDisableRequest when toggled off', () => {
    render(<SsoToggle {...baseProps} ssoState={{enabled: true, ssoCheckIds: ['sso_check_1']}} />);

    const toggle = getSwitch();
    expect(toggle.checked).toBe(true);

    fireEvent.click(toggle);

    expect(baseProps.onDisableRequest).toHaveBeenCalledTimes(1);
    expect(baseProps.onEnable).not.toHaveBeenCalled();
  });

  it.each([['no-entry' as const], ['entry-not-prompt' as const], ['no-assert' as const]])(
    'should disable the toggle when the join resolution is %s',
    (status) => {
      render(<SsoToggle {...baseProps} joinResolution={{status}} />);

      expect(getSwitch()).toBeDisabled();
    },
  );

  it('should disable the toggle for read-only flows', () => {
    render(<SsoToggle {...baseProps} isReadOnly />);

    expect(getSwitch()).toBeDisabled();
  });

  it('should disable the toggle while placement mode is active', () => {
    render(<SsoToggle {...baseProps} placement={{active: true, candidateEdgeIds: ['e1']}} />);

    expect(getSwitch()).toBeDisabled();
  });

  it('should keep the toggle enabled when the join is ambiguous (placement mode entry)', () => {
    render(
      <SsoToggle
        {...baseProps}
        joinResolution={{candidateEdgeIds: ['e1'], candidateJoinNodeIds: ['a'], status: 'ambiguous'}}
      />,
    );

    expect(getSwitch()).not.toBeDisabled();
  });

  it('should select and focus the inserted SSO check when a focus request arrives', () => {
    const rafSpy = vi
      .spyOn(window, 'requestAnimationFrame')
      .mockImplementation((callback: FrameRequestCallback): number => {
        callback(0);
        return 0;
      });
    const onFocusHandled = vi.fn();
    const resource = {id: 'sso_check_ab12'} as unknown as Resource;

    render(
      <SsoToggle
        {...baseProps}
        focusRequest={{resource, sessionId: 'session_ab12', ssoCheckId: 'sso_check_ab12'}}
        onFocusHandled={onFocusHandled}
      />,
    );

    expect(mockSetLastInteractedStepId).toHaveBeenCalledWith('sso_check_ab12');
    expect(mockSetLastInteractedResource).toHaveBeenCalledWith(resource);
    expect(mockUpdateNodeInternals).toHaveBeenCalledWith(['sso_check_ab12', 'session_ab12']);
    expect(mockFitView).toHaveBeenCalledWith(expect.objectContaining({nodes: [{id: 'sso_check_ab12'}]}));
    expect(onFocusHandled).toHaveBeenCalledTimes(1);

    rafSpy.mockRestore();
  });
});
