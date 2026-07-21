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

/* eslint-disable @typescript-eslint/non-nullable-type-assertion-style */

import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import RichTextActionFields from '../RichTextActionFields';
import type {Resource} from '@/features/flows/models/resources';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, fallback?: string) => fallback ?? _key,
  }),
}));

const mockSetEdges = vi.fn();
const mockGetEdges = vi.fn(() => []);
const mockEdges: {sourceHandle?: string; target?: string}[] = [];
vi.mock('@xyflow/react', () => ({
  useReactFlow: () => ({
    getEdges: mockGetEdges,
    setEdges: mockSetEdges,
  }),
  useEdges: () => mockEdges,
}));

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'rt-1',
    type: 'RICH_TEXT',
    category: 'DISPLAY',
    resourceType: 'ELEMENT',
    ...overrides,
  }) as Resource;

describe('RichTextActionFields', () => {
  const onChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the toggle in the off position when action is undefined', () => {
    render(<RichTextActionFields resource={makeResource()} onChange={onChange} />);
    const toggle = screen.getByTestId('rich-text-action-enabled').querySelector('input') as HTMLInputElement;
    expect(toggle.checked).toBe(false);
    expect(screen.queryByTestId('rich-text-action-ref')).not.toBeInTheDocument();
  });

  it('turning the toggle on writes an empty action ref via onChange', () => {
    render(<RichTextActionFields resource={makeResource()} onChange={onChange} />);
    const toggle = screen.getByTestId('rich-text-action-enabled').querySelector('input') as HTMLInputElement;
    fireEvent.click(toggle);
    expect(onChange).toHaveBeenCalledWith('action', {ref: ''}, expect.anything());
  });

  it('preserves an existing action ref when the toggle is turned back on', () => {
    // Widget-supplied refs (e.g. Self Sign Up Link ships `action_signup`) must survive
    // a disable→enable cycle so the anchor's data-action-ref still matches at runtime.
    const resource = makeResource({action: {ref: 'action_signup'}} as unknown as Partial<Resource>);
    render(<RichTextActionFields resource={resource} onChange={onChange} />);
    const toggle = screen.getByTestId('rich-text-action-enabled').querySelector('input') as HTMLInputElement;
    fireEvent.click(toggle);
    fireEvent.click(toggle);
    expect(onChange).toHaveBeenLastCalledWith('action', {ref: 'action_signup'}, resource);
  });

  it('renders the connected step field as read-only when action is enabled', () => {
    const resource = makeResource({action: {ref: 'action_signup'}} as unknown as Partial<Resource>);
    render(<RichTextActionFields resource={resource} onChange={onChange} />);
    const input = screen.getByTestId('rich-text-action-ref').querySelector('input') as HTMLInputElement;
    expect(input.readOnly).toBe(true);
  });

  it('turning the toggle off clears the action', () => {
    const resource = makeResource({action: {ref: 'action_signup'}} as unknown as Partial<Resource>);
    render(<RichTextActionFields resource={resource} onChange={onChange} />);
    const toggle = screen.getByTestId('rich-text-action-enabled').querySelector('input') as HTMLInputElement;
    fireEvent.click(toggle);
    expect(onChange).toHaveBeenCalledWith('action', null, resource);
  });
});
