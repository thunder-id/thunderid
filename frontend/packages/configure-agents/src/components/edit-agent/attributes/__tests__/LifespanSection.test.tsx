// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent} from '../../../../models/agent';
import LifespanSection from '../LifespanSection';

const DAY = 24 * 3600 * 1000;

describe('LifespanSection', () => {
  const mockOnFieldChange = vi.fn();

  const mockAgent: Agent = {
    id: 'agent-1',
    ouId: 'ou-1',
    type: 'default',
    display: 'Test Agent',
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('says an agent without an end does not expire', () => {
    render(<LifespanSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('Lifespan')).toBeInTheDocument();
    expect(screen.getByText('No expiry date. Set one to end its access automatically.')).toBeInTheDocument();
  });

  it('says how long until a stored end', () => {
    const validUntil = new Date(Date.now() + 90 * DAY).toISOString();

    render(<LifespanSection agent={{...mockAgent, validUntil}} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('Expires in 3 months')).toBeInTheDocument();
  });

  // The stored end of an expired agent is shown as it is, not flagged, so it does not block saving other changes.
  it('shows an expired end without reporting it as invalid', () => {
    const onValidationChange = vi.fn();
    const validUntil = new Date(Date.now() - 13 * DAY).toISOString();

    render(
      <LifespanSection
        agent={{...mockAgent, validUntil}}
        editedAgent={{}}
        onFieldChange={mockOnFieldChange}
        onValidationChange={onValidationChange}
      />,
    );

    expect(screen.getByText('Expired 13 days ago')).toBeInTheDocument();
    expect(onValidationChange).not.toHaveBeenCalledWith(true);
  });

  it('prefers the edited end over the stored one', () => {
    render(
      <LifespanSection
        agent={{...mockAgent, validUntil: new Date(Date.now() + 90 * DAY).toISOString()}}
        editedAgent={{validUntil: new Date(Date.now() + 5 * DAY).toISOString()}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByText('Expires in 5 days')).toBeInTheDocument();
  });

  it('stages null, which removes the end, when the field is cleared', async () => {
    const user = userEvent.setup();

    render(
      <LifespanSection
        agent={{...mockAgent, validUntil: new Date(Date.now() + 90 * DAY).toISOString()}}
        editedAgent={{}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByRole('button', {name: /clear/i}));

    expect(mockOnFieldChange).toHaveBeenCalledWith('validUntil', null);
  });

  it('is read only for a declarative agent', () => {
    render(
      <LifespanSection agent={{...mockAgent, isReadOnly: true}} editedAgent={{}} onFieldChange={mockOnFieldChange} />,
    );

    expect(screen.getByRole('button', {name: /choose date/i})).toBeDisabled();
  });
});
