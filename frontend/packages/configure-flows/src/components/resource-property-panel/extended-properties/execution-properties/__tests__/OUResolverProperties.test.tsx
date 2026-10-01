// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import OUResolverProperties from '../OUResolverProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.executions.ouResolver.description': 'Configure how the organization unit is resolved.',
        'flows:core.executions.ouResolver.resolveFrom.label': 'Resolve from',
        'flows:core.executions.ouResolver.resolveFrom.caller': 'Caller',
        'flows:core.executions.ouResolver.resolveFrom.prompt': 'Prompt',
        'flows:core.executions.ouResolver.resolveFrom.promptAll': 'Prompt all',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('OUResolverProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it('should render the description and resolve-from label', () => {
    const resource = {data: {properties: {resolveFrom: 'prompt'}}} as unknown as Resource;

    render(<OUResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByText('Configure how the organization unit is resolved.')).toBeInTheDocument();
    expect(screen.getByText('Resolve from')).toBeInTheDocument();
  });

  it('should default resolveFrom to caller when the node has no properties', () => {
    const resource = {data: {}} as unknown as Resource;

    render(<OUResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByRole('combobox')).toHaveTextContent('Caller');
  });

  it('should show the configured resolveFrom value when present', () => {
    const resource = {data: {properties: {resolveFrom: 'promptAll'}}} as unknown as Resource;

    render(<OUResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByRole('combobox')).toHaveTextContent('Prompt all');
  });

  it('should call onChange when a new resolveFrom option is selected', async () => {
    const user = userEvent.setup();
    const resource = {data: {properties: {resolveFrom: 'caller'}}} as unknown as Resource;

    render(<OUResolverProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'Prompt'}));

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.resolveFrom', 'prompt', resource);
  });
});
