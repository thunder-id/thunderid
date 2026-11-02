// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {createAgentLabelResolver, getMissingAgentTypeHandles} from '../resolveAgentLabel';

describe('createAgentLabelResolver', () => {
  const nameType = {handle: 'default', systemAttributes: {display: 'name'}};

  it('uses the display attribute of a bundled type', () => {
    const resolve = createAgentLabelResolver([nameType], []);

    expect(resolve({id: 'a1', type: 'default', attributes: {name: 'Calendar Agent'}})).toBe('Calendar Agent');
  });

  it('prefers the bundled type over a server type with the same handle', () => {
    const resolve = createAgentLabelResolver([nameType], [{handle: 'default', systemAttributes: {display: 'title'}}]);

    expect(resolve({id: 'a1', type: 'default', attributes: {name: 'Bundled', title: 'Server'}})).toBe('Bundled');
  });

  it('does not consult the server when the bundled type has no display path', () => {
    const resolve = createAgentLabelResolver(
      [{handle: 'default'}],
      [{handle: 'default', systemAttributes: {display: 'name'}}],
    );

    expect(resolve({id: 'a1', type: 'default', attributes: {name: 'Server'}})).toBe('a1');
  });

  it('falls back to the ID when the bundled type has no usable value', () => {
    const resolve = createAgentLabelResolver([nameType], [{handle: 'default', systemAttributes: {display: 'title'}}]);

    expect(resolve({id: 'a1', type: 'default', attributes: {title: 'Server'}})).toBe('a1');
  });

  it('uses a server type when the type is not bundled', () => {
    const resolve = createAgentLabelResolver([], [nameType]);

    expect(resolve({id: 'a1', type: 'default', attributes: {name: 'From Server'}})).toBe('From Server');
  });

  it('falls back to the ID when the type is in neither place', () => {
    const resolve = createAgentLabelResolver([], []);

    expect(resolve({id: 'a1', type: 'default', attributes: {name: 'Ignored'}})).toBe('a1');
  });

  it('supports custom and nested display paths', () => {
    const resolve = createAgentLabelResolver([{handle: 'bot', systemAttributes: {display: 'profile.title'}}], []);

    expect(resolve({id: 'a1', type: 'bot', attributes: {profile: {title: 'Nested'}}})).toBe('Nested');
  });

  it('accepts numeric values and rejects other value kinds', () => {
    const resolve = createAgentLabelResolver([{handle: 'bot', systemAttributes: {display: 'v'}}], []);

    expect(resolve({id: 'a1', type: 'bot', attributes: {v: 42}})).toBe('42');
    expect(resolve({id: 'a1', type: 'bot', attributes: {v: true}})).toBe('a1');
    expect(resolve({id: 'a1', type: 'bot', attributes: {v: ['x']}})).toBe('a1');
    expect(resolve({id: 'a1', type: 'bot', attributes: {v: {x: 1}}})).toBe('a1');
    expect(resolve({id: 'a1', type: 'bot', attributes: {v: ''}})).toBe('a1');
    expect(resolve({id: 'a1', type: 'bot', attributes: {}})).toBe('a1');
  });

  it('does not index into arrays along a path', () => {
    const resolve = createAgentLabelResolver([{handle: 'bot', systemAttributes: {display: 'tags.0'}}], []);

    expect(resolve({id: 'a1', type: 'bot', attributes: {tags: ['first']}})).toBe('a1');
  });

  it('returns undefined when there is neither a value nor an ID', () => {
    const resolve = createAgentLabelResolver([nameType], []);

    expect(resolve({type: 'default', attributes: {}})).toBeUndefined();
  });

  it('handles an agent with no type', () => {
    const resolve = createAgentLabelResolver([nameType], []);

    expect(resolve({id: 'a1', attributes: {name: 'Typeless'}})).toBe('a1');
  });
});

describe('getMissingAgentTypeHandles', () => {
  it('lists distinct handles that are absent from the bundled types', () => {
    const agents = [{type: 'a'}, {type: 'a'}, {type: 'b'}, {type: 'local'}, {}];

    expect(getMissingAgentTypeHandles(agents, [{handle: 'local'}])).toEqual(['a', 'b']);
  });

  it('returns nothing when every type is bundled or there are no agents', () => {
    expect(getMissingAgentTypeHandles([{type: 'local'}], [{handle: 'local'}])).toEqual([]);
    expect(getMissingAgentTypeHandles([], [])).toEqual([]);
  });
});
