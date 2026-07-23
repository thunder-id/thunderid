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

import {describe, expect, it} from 'vitest';
import {
  resolveApplicationMeta,
  resolveTemplatesDeep,
  withDerivedEventTypes,
  withDynamicFieldStandIns,
  withRichTextActionRefs,
  type PreviewComponent,
} from '../gatePreviewTransforms';
import type {Application} from '@/features/applications/models/application';

const application = {
  id: 'app-1',
  name: 'My App',
  logoUrl: 'https://myapp.example/logo.png',
} as unknown as Application;

describe('resolveApplicationMeta', () => {
  it('should replace application meta placeholders in nested structures', () => {
    const tree = {
      label: '{{ meta(application.name) }} wants access',
      components: [{src: '{{ meta(application.logoUrl) }}'}],
    };

    expect(resolveApplicationMeta(tree, application)).toEqual({
      label: 'My App wants access',
      components: [{src: 'https://myapp.example/logo.png'}],
    });
  });

  it('should replace unknown or non-string properties with an empty string', () => {
    expect(resolveApplicationMeta('{{ meta(application.missing) }}', application)).toBe('');
  });

  it('should leave non-string values untouched', () => {
    expect(resolveApplicationMeta({required: true, count: 3}, application)).toEqual({required: true, count: 3});
  });
});

describe('resolveTemplatesDeep', () => {
  const resolveText = (raw: string): string => raw.replaceAll('{{ t(some:key) }}', 'Resolved');

  it('should resolve template strings recursively and leave plain strings as-is', () => {
    const tree = {
      label: '{{ t(some:key) }}',
      components: [{label: 'plain text'}],
    };

    expect(resolveTemplatesDeep(tree, resolveText)).toEqual({
      label: 'Resolved',
      components: [{label: 'plain text'}],
    });
  });

  it('should hand non-i18n templates to the resolver as well', () => {
    const seen: string[] = [];
    const trackingResolver = (raw: string): string => {
      seen.push(raw);
      return raw;
    };

    resolveTemplatesDeep({label: '{{ meta(application.name) }}'}, trackingResolver);

    expect(seen).toEqual(['{{ meta(application.name) }}']);
  });
});

describe('withDerivedEventTypes', () => {
  const makeAction = (overrides: Record<string, unknown> = {}): PreviewComponent =>
    ({id: 'action_001', type: 'ACTION', category: 'ACTION', label: 'Continue', ...overrides}) as PreviewComponent;

  it('should derive TRIGGER for actions without a button type', () => {
    const [block] = withDerivedEventTypes([
      {id: 'block_001', type: 'BLOCK', components: [makeAction(), makeAction({id: 'action_002'})]} as PreviewComponent,
    ]);

    const [first, second] = block.components as (PreviewComponent & {eventType?: string})[];
    expect(first.eventType).toBe('TRIGGER');
    expect(second.eventType).toBe('TRIGGER');
  });

  it('should derive SUBMIT for submit-type buttons', () => {
    const [action] = withDerivedEventTypes([makeAction({buttonType: 'submit'})]) as (PreviewComponent & {
      eventType?: string;
    })[];

    expect(action.eventType).toBe('SUBMIT');
  });

  it('should promote the sole action of a block with inputs to SUBMIT', () => {
    const [block] = withDerivedEventTypes([
      {
        id: 'block_001',
        type: 'BLOCK',
        components: [{id: 'input_001', type: 'TEXT_INPUT'} as PreviewComponent, makeAction()],
      } as PreviewComponent,
    ]);

    const action = (block.components as (PreviewComponent & {eventType?: string})[])[1];
    expect(action.eventType).toBe('SUBMIT');
  });

  it('should keep an explicit eventType untouched', () => {
    const [action] = withDerivedEventTypes([makeAction({eventType: 'NAVIGATE'})]) as (PreviewComponent & {
      eventType?: string;
    })[];

    expect(action.eventType).toBe('NAVIGATE');
  });
});

describe('withDynamicFieldStandIns', () => {
  const dynamicPlaceholder = {
    id: 'dynamic_001',
    type: 'DYNAMIC_INPUT_PLACEHOLDER',
    placeholder: 'Dynamic Input',
    hint: 'Resolves input fields passed from runtime.',
  } as unknown as PreviewComponent;

  it('should replace nested placeholders with a rich-text skeleton and blank the builder chrome', () => {
    const [block] = withDynamicFieldStandIns(
      [{id: 'block_001', type: 'BLOCK', components: [dynamicPlaceholder]} as unknown as PreviewComponent],
      'Input fields resolved at runtime',
    );

    const [standIn] = block.components as PreviewComponent[];
    expect(standIn.type).toBe('RICH_TEXT');
    expect(standIn.label).toContain('Input fields resolved at runtime');
    expect(standIn.placeholder).toBe('');
    expect((standIn as PreviewComponent & {hint?: string}).hint).toBe('');
  });

  it('should escape HTML in the caption', () => {
    const [standIn] = withDynamicFieldStandIns([dynamicPlaceholder], '<img src=x onerror=alert(1)>');

    expect(standIn.label).not.toContain('<img');
    expect(standIn.label).toContain('&lt;img');
  });

  it('should leave unrelated components untouched', () => {
    const text = {id: 'text_001', type: 'TEXT', label: 'Sign In'} as unknown as PreviewComponent;

    expect(withDynamicFieldStandIns([text], 'caption')[0]).toEqual(text);
  });
});

describe('withRichTextActionRefs', () => {
  it('should attach the edge id as action.ref on a wired rich-text component', () => {
    const richText = {id: 'rich_001', type: 'RICH_TEXT', label: '<p><a href="#">Reset</a></p>'} as PreviewComponent;
    const [result] = withRichTextActionRefs([richText], new Map([['rich_001', 'action_recovery']]));

    expect((result as PreviewComponent & {action?: {ref?: string}}).action?.ref).toBe('action_recovery');
  });

  it('should re-attach action.ref on rich text nested inside a block', () => {
    const block = {
      id: 'block_001',
      type: 'BLOCK',
      components: [{id: 'rich_001', type: 'RICH_TEXT', label: '<p><a href="#">Reset</a></p>'}],
    } as unknown as PreviewComponent;
    const [result] = withRichTextActionRefs([block], new Map([['rich_001', 'action_recovery']]));
    const [nested] = result.components as (PreviewComponent & {action?: {ref?: string}})[];

    expect(nested.action?.ref).toBe('action_recovery');
  });

  it('should not touch rich text without a matching option', () => {
    const richText = {id: 'rich_002', type: 'RICH_TEXT', label: 'plain'} as PreviewComponent;
    const [result] = withRichTextActionRefs([richText], new Map([['rich_001', 'action_recovery']]));

    expect((result as PreviewComponent & {action?: unknown}).action).toBeUndefined();
  });

  it('should leave non-rich-text components untouched', () => {
    const action = {id: 'action_001', type: 'ACTION', label: 'Sign In'} as unknown as PreviewComponent;
    const [result] = withRichTextActionRefs([action], new Map([['action_001', 'action_001']]));

    expect((result as PreviewComponent & {action?: unknown}).action).toBeUndefined();
  });
});
