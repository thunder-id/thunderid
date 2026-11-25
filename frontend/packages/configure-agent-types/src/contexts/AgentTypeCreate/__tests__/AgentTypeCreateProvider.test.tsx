// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, renderHook} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect} from 'vitest';
import {AgentTypeCreateFlowStep} from '../../../models/agent-type-create-flow';
import AgentTypeCreateProvider from '../AgentTypeCreateProvider';
import useAgentTypeCreate from '../useAgentTypeCreate';

const wrapper = ({children}: {children: ReactNode}) => <AgentTypeCreateProvider>{children}</AgentTypeCreateProvider>;

describe('AgentTypeCreateProvider', () => {
  it('should start on the organization unit step with empty fields', () => {
    const {result} = renderHook(() => useAgentTypeCreate(), {wrapper});

    expect(result.current.currentStep).toBe(AgentTypeCreateFlowStep.ORGANIZATION_UNIT);
    expect(result.current.name).toBe('');
    expect(result.current.handle).toBe('');
    expect(result.current.handleEdited).toBe(false);
    expect(result.current.ouId).toBe('');
    expect(result.current.properties).toEqual([]);
    expect(result.current.displayAttribute).toBe('');
    expect(result.current.error).toBeNull();
  });

  it('should reset every field to its initial value', () => {
    const {result} = renderHook(() => useAgentTypeCreate(), {wrapper});

    act(() => {
      result.current.setCurrentStep(AgentTypeCreateFlowStep.PROPERTIES);
      result.current.setName('Assistant');
      result.current.setHandle('assistant');
      result.current.setHandleEdited(true);
      result.current.setOuId('ou-1');
      result.current.setDisplayAttribute('name');
      result.current.setError('Failed');
    });

    expect(result.current.name).toBe('Assistant');

    act(() => {
      result.current.reset();
    });

    expect(result.current.currentStep).toBe(AgentTypeCreateFlowStep.ORGANIZATION_UNIT);
    expect(result.current.name).toBe('');
    expect(result.current.handle).toBe('');
    expect(result.current.handleEdited).toBe(false);
    expect(result.current.ouId).toBe('');
    expect(result.current.displayAttribute).toBe('');
    expect(result.current.error).toBeNull();
  });

  it('should throw when used outside the provider', () => {
    expect(() => renderHook(() => useAgentTypeCreate())).toThrow(
      'useAgentTypeCreate must be used within an AgentTypeCreateProvider',
    );
  });
});
