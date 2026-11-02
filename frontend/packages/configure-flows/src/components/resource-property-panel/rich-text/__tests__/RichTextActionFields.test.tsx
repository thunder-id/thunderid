// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/non-nullable-type-assertion-style */

import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Resource} from '../../../../models/resources';
import RichTextActionFields from '../RichTextActionFields';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
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

  it('turning the toggle on falls back to the component id when the label has no sentinel', () => {
    // A label with no data-action-ref dispatches on any anchor click, so the component id is a
    // safe ref. An empty one would serialise as `{"ref": ""}` and never dispatch.
    render(<RichTextActionFields resource={makeResource()} onChange={onChange} />);
    const toggle = screen.getByTestId('rich-text-action-enabled').querySelector('input') as HTMLInputElement;
    fireEvent.click(toggle);
    expect(onChange).toHaveBeenCalledWith('action', {ref: 'rt-1'}, expect.anything());
  });

  it("turning the toggle on adopts the label anchor's sentinel", () => {
    const resource = makeResource({
      label: '<p><a href="#" data-action-ref="action_recovery">Forgot password?</a></p>',
    } as unknown as Partial<Resource>);
    render(<RichTextActionFields resource={resource} onChange={onChange} />);
    const toggle = screen.getByTestId('rich-text-action-enabled').querySelector('input') as HTMLInputElement;
    fireEvent.click(toggle);
    expect(onChange).toHaveBeenCalledWith('action', {ref: 'action_recovery'}, resource);
  });

  it('turning the toggle back on drops a ref no anchor carries', () => {
    // Flows saved before the ref and the sentinel were kept in sync hold a target node id in
    // `action.ref`. Re-seeding it would keep the link dead; the label is authoritative.
    const resource = makeResource({
      action: {ref: 'recovery_call_g0v6'},
      label: '<p><a href="#" data-action-ref="action_recovery">Forgot password?</a></p>',
    } as unknown as Partial<Resource>);
    render(<RichTextActionFields resource={resource} onChange={onChange} />);
    const toggle = screen.getByTestId('rich-text-action-enabled').querySelector('input') as HTMLInputElement;
    fireEvent.click(toggle);
    fireEvent.click(toggle);
    expect(onChange).toHaveBeenLastCalledWith('action', {ref: 'action_recovery'}, resource);
  });

  it('keeps a ref that matches a later anchor in a multi-link label', () => {
    const resource = makeResource({
      action: {ref: 'action_signup'},
      label:
        '<p><a href="#" data-action-ref="action_recovery">Forgot password?</a>' +
        '<a href="#" data-action-ref="action_signup">Sign up</a></p>',
    } as unknown as Partial<Resource>);
    render(<RichTextActionFields resource={resource} onChange={onChange} />);
    const toggle = screen.getByTestId('rich-text-action-enabled').querySelector('input') as HTMLInputElement;
    fireEvent.click(toggle);
    fireEvent.click(toggle);
    expect(onChange).toHaveBeenLastCalledWith('action', {ref: 'action_signup'}, resource);
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
