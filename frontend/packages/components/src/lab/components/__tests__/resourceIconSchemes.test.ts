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

/* eslint-disable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-argument */
import {
  extractAvatarParamsFromUri as parseAvatarSpec,
  buildAvatarSpec,
  ANONYMOUS_ANIMAL_ICONS,
  ANONYMOUS_ENTITY_ICONS,
} from '@thunderid/react';
import {isAbsoluteUrl as isUrl} from '@thunderid/utils';
import {describe, it, expect} from 'vitest';
import {resolveResourceIcon} from '../resourceIconSchemes';

describe('isUrl', () => {
  it('should recognize http and https URLs', () => {
    expect(isUrl('http://example.com/logo.png')).toBe(true);
    expect(isUrl('https://example.com/logo.png')).toBe(true);
  });

  it('should reject non-URL strings', () => {
    expect(isUrl('emoji:🐼')).toBe(false);
    expect(isUrl('🐼')).toBe(false);
  });
});

describe('parseAvatarSpec / buildAvatarSpec', () => {
  it('should round-trip shape, variant, content, and colors', () => {
    const spec = buildAvatarSpec({colors: 2, content: 'AC', shape: 'circle', variant: 'two_letter'});
    expect(parseAvatarSpec(spec)).toEqual({colors: 2, content: 'AC', shape: 'circle', variant: 'two_letter'});
  });

  it('should round-trip an anonymous_animal spec with a bg override', () => {
    const spec = buildAvatarSpec({
      bg: '#FF5733',
      colors: 0,
      content: 'jackalope',
      shape: 'rounded',
      variant: 'anonymous_animal',
    });
    expect(parseAvatarSpec(spec)).toEqual({
      bg: '#FF5733',
      colors: 0,
      content: 'jackalope',
      shape: 'rounded',
      variant: 'anonymous_animal',
    });
  });

  it('should round-trip an anonymous_entity spec with a bg override', () => {
    const spec = buildAvatarSpec({
      bg: '#FF5733',
      colors: 0,
      content: 'hexagon',
      shape: 'rounded',
      variant: 'anonymous_entity',
    });
    expect(parseAvatarSpec(spec)).toEqual({
      bg: '#FF5733',
      colors: 0,
      content: 'hexagon',
      shape: 'rounded',
      variant: 'anonymous_entity',
    });
  });

  it('should fall back to defaults for missing or invalid params', () => {
    expect(parseAvatarSpec('avatar:')).toEqual({colors: 0, content: '', shape: 'rounded', variant: 'two_letter'});
  });
});

describe('resolveResourceIcon', () => {
  it('should resolve an emoji: spec to its glyph', () => {
    expect(resolveResourceIcon('emoji:🐼')).toEqual({char: '🐼', type: 'emoji'});
  });

  it('should treat a bare non-scheme value as a raw emoji for backwards compatibility', () => {
    expect(resolveResourceIcon('🐼')).toEqual({char: '🐼', type: 'emoji'});
  });

  it('should resolve a plain URL as an image', () => {
    expect(resolveResourceIcon('https://example.com/logo.png')).toEqual({
      src: 'https://example.com/logo.png',
      type: 'image',
    });
  });

  it('should resolve an avatar: spec to a generated data URI, falling back to seed text', () => {
    const resolved = resolveResourceIcon('avatar:shape=circle,variant=two_letter,colors=0', 'Acme');
    expect(resolved.type).toBe('image');
    expect(decodeURIComponent((resolved as {src: string}).src)).toContain('>AC<');
  });

  it('should resolve an anonymous_animal avatar: spec to its bundled icon', () => {
    const [name, icon] = Object.entries(ANONYMOUS_ANIMAL_ICONS)[0];
    const resolved = resolveResourceIcon(`avatar:shape=rounded,variant=anonymous_animal,content=${name}`);
    expect(resolved.type).toBe('image');
    expect(decodeURIComponent((resolved as {src: string}).src)).toContain(icon.color);
  });

  it('should resolve an anonymous_entity avatar: spec to its bundled icon', () => {
    const [name, icon] = Object.entries(ANONYMOUS_ENTITY_ICONS)[0];
    const resolved = resolveResourceIcon(`avatar:shape=rounded,variant=anonymous_entity,content=${name}`);
    expect(resolved.type).toBe('image');
    expect(decodeURIComponent((resolved as {src: string}).src)).toContain(icon.color);
  });
});
