// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AudienceSection from '../AudienceSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string, opts?: {entity?: string}) =>
      (fallback ?? key).replace('{{entity}}', opts?.entity ?? ''),
  }),
}));

describe('AudienceSection', () => {
  const onAudienceChange = vi.fn();

  beforeEach(() => {
    onAudienceChange.mockClear();
  });

  it('renders the card and the current audience value', () => {
    render(<AudienceSection audience="https://api.example.com" onAudienceChange={onAudienceChange} />);

    expect(screen.getByText('Default Audience')).toBeInTheDocument();
    expect(screen.getByDisplayValue('https://api.example.com')).toBeInTheDocument();
  });

  it('reports the typed audience (trimmed)', async () => {
    const user = userEvent.setup();
    render(<AudienceSection audience="" onAudienceChange={onAudienceChange} />);

    const input = screen.getByPlaceholderText('e.g. https://api.example.com');
    await user.type(input, 'x');

    expect(onAudienceChange).toHaveBeenLastCalledWith('x');
  });

  it('disables the input when disabled is set', () => {
    render(<AudienceSection audience="" onAudienceChange={onAudienceChange} disabled />);

    expect(screen.getByPlaceholderText('e.g. https://api.example.com')).toBeDisabled();
  });
});
