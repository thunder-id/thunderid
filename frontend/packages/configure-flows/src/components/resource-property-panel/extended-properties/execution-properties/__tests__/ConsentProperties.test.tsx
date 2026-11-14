// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import ConsentProperties from '../ConsentProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, fallback?: string) => {
      const translations: Record<string, string> = {
        'flows:core.executions.consent.failOnDeny.label': 'Fail flow when user denies consent',
        'flows:core.executions.consent.promptValidityPeriod.label': 'Let the user choose the consent validity period',
        'flows:core.executions.consent.promptValidityPeriod.hint': 'Limited by the application or agent period.',
      };
      return translations[key] ?? fallback ?? key;
    },
  }),
}));

describe('ConsentProperties', () => {
  const mockOnChange = vi.fn();
  const validityOption = 'Let the user choose the consent validity period';

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it('should render the validity period option unchecked, with its hint, when the node does not set it', () => {
    const resource = {data: {properties: {}}} as unknown as Resource;

    render(<ConsentProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByRole('checkbox', {name: validityOption})).not.toBeChecked();
    expect(screen.getByText('Limited by the application or agent period.')).toBeInTheDocument();
  });

  it('should render the validity period option checked when the node prompts for it', () => {
    const resource = {data: {properties: {promptValidityPeriod: true}}} as unknown as Resource;

    render(<ConsentProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByRole('checkbox', {name: validityOption})).toBeChecked();
  });

  it('should store the validity period option as a boolean on the node', () => {
    const resource = {data: {properties: {}}} as unknown as Resource;

    render(<ConsentProperties resource={resource} onChange={mockOnChange} />);

    fireEvent.click(screen.getByRole('checkbox', {name: validityOption}));

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.promptValidityPeriod', true, resource);
  });

  it('should store the fail on deny option on its own property', () => {
    const resource = {data: {properties: {promptValidityPeriod: true}}} as unknown as Resource;

    render(<ConsentProperties resource={resource} onChange={mockOnChange} />);

    fireEvent.click(screen.getByRole('checkbox', {name: 'Fail flow when user denies consent'}));

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.failOnDeny', true, resource);
  });

  it('should not offer a validity period field for the flow', () => {
    const resource = {data: {properties: {}}} as unknown as Resource;

    render(<ConsentProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.queryByRole('spinbutton', {name: /validity period/i})).not.toBeInTheDocument();
  });
});
