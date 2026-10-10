// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';

const {mockUseGetAgentTypes} = vi.hoisted(() => ({mockUseGetAgentTypes: vi.fn()}));

vi.mock('@thunderid/configure-agent-types', () => ({useGetAgentTypes: mockUseGetAgentTypes}));

import useAgentLabels from '../useAgentLabels';

const serverTypes = {data: {types: [{handle: 'default', systemAttributes: {display: 'name'}}]}};

describe('useAgentLabels', () => {
  beforeEach(() => {
    mockUseGetAgentTypes.mockReset();
    mockUseGetAgentTypes.mockReturnValue({data: undefined});
  });

  it('resolves from a bundled type without querying the server', () => {
    const configData = {
      agent_type: [{handle: 'default', systemAttributes: {display: 'name'}}],
      agent: [{id: 'a1', type: 'default', attributes: {name: 'Bundled'}}],
    };

    const {result} = renderHook(() => useAgentLabels(configData));

    expect(result.current(configData.agent[0])).toBe('Bundled');
    expect(mockUseGetAgentTypes).toHaveBeenCalledWith(undefined, {enabled: false});
  });

  it('resolves an agents-only bundle through the server type', () => {
    mockUseGetAgentTypes.mockReturnValue(serverTypes);
    const configData = {agent: [{id: 'a1', type: 'default', attributes: {name: 'From Server'}}]};

    const {result} = renderHook(() => useAgentLabels(configData));

    expect(result.current(configData.agent[0])).toBe('From Server');
    expect(mockUseGetAgentTypes).toHaveBeenCalledWith(undefined, {enabled: true});
  });

  it('makes a single lookup for many agents that share a missing type', () => {
    const configData = {
      agent: [
        {id: 'a1', type: 'default'},
        {id: 'a2', type: 'default'},
      ],
    };

    renderHook(() => useAgentLabels(configData));

    expect(mockUseGetAgentTypes).toHaveBeenCalledTimes(1);
  });

  it('falls back to the ID while the lookup is pending or after it fails', () => {
    mockUseGetAgentTypes.mockReturnValue({data: undefined, isError: true});
    const configData = {agent: [{id: 'a1', type: 'default', attributes: {name: 'Hidden'}}]};

    const {result} = renderHook(() => useAgentLabels(configData));

    expect(result.current(configData.agent[0])).toBe('a1');
  });

  it('ignores user types that share the handle', () => {
    const configData = {
      user_type: [{handle: 'default', systemAttributes: {display: 'name'}}],
      agent: [{id: 'a1', type: 'default', attributes: {name: 'Hidden'}}],
    };

    const {result} = renderHook(() => useAgentLabels(configData));

    expect(result.current(configData.agent[0])).toBe('a1');
    expect(mockUseGetAgentTypes).toHaveBeenCalledWith(undefined, {enabled: true});
  });

  it('recomputes when the configuration changes', () => {
    const first = {
      agent_type: [{handle: 'default', systemAttributes: {display: 'name'}}],
      agent: [{id: 'a1', type: 'default', attributes: {name: 'One', title: 'Two'}}],
    };
    const second = {...first, agent_type: [{handle: 'default', systemAttributes: {display: 'title'}}]};

    const {result, rerender} = renderHook(({data}) => useAgentLabels(data), {initialProps: {data: first}});
    expect(result.current(first.agent[0])).toBe('One');

    rerender({data: second});

    expect(result.current(second.agent[0])).toBe('Two');
  });

  it('handles an empty or missing configuration', () => {
    const {result} = renderHook(() => useAgentLabels(null));

    expect(result.current({id: 'a1'})).toBe('a1');
    expect(mockUseGetAgentTypes).toHaveBeenCalledWith(undefined, {enabled: false});
  });
});
