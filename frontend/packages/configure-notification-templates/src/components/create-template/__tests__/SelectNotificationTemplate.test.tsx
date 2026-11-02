// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import SelectNotificationTemplate, {
  type TemplateSelection,
} from '@/components/create-template/SelectNotificationTemplate';
import {getSampleTemplates} from '@/constants/sample-templates';

describe('SelectNotificationTemplate', () => {
  it('renders the start-from-scratch card and the channel samples', () => {
    render(<SelectNotificationTemplate channel="email" selected={null} onSelect={vi.fn()} />);

    expect(screen.getByTestId('template-card-blank')).toBeInTheDocument();
    for (const sample of getSampleTemplates('email')) {
      expect(screen.getByTestId(`template-card-${sample.id}`)).toBeInTheDocument();
    }
  });

  it('shows only the channel-specific samples', () => {
    render(<SelectNotificationTemplate channel="sms" selected={null} onSelect={vi.fn()} />);

    // An email-only sample is not offered for SMS.
    expect(screen.queryByTestId('template-card-email-magic-link')).not.toBeInTheDocument();
    expect(screen.getByTestId('template-card-sms-otp')).toBeInTheDocument();
  });

  it('reports a blank selection', async () => {
    const onSelect = vi.fn<(selection: TemplateSelection) => void>();
    const user = userEvent.setup();
    render(<SelectNotificationTemplate channel="email" selected={null} onSelect={onSelect} />);

    await user.click(screen.getByTestId('template-card-blank'));

    expect(onSelect).toHaveBeenCalledWith({kind: 'blank'});
  });

  it('reports the chosen sample', async () => {
    const onSelect = vi.fn<(selection: TemplateSelection) => void>();
    const user = userEvent.setup();
    render(<SelectNotificationTemplate channel="email" selected={null} onSelect={onSelect} />);

    await user.click(screen.getByTestId('template-card-email-otp'));

    expect(onSelect).toHaveBeenCalledWith({
      kind: 'sample',
      sample: expect.objectContaining({id: 'email-otp'}) as unknown,
    });
  });

  it('filters samples by the search query', async () => {
    const user = userEvent.setup();
    render(<SelectNotificationTemplate channel="email" selected={null} onSelect={vi.fn()} />);

    await user.type(screen.getByPlaceholderText('Search templates...'), 'magic');

    expect(screen.getByTestId('template-card-email-magic-link')).toBeInTheDocument();
    expect(screen.queryByTestId('template-card-email-otp')).not.toBeInTheDocument();
  });
});
