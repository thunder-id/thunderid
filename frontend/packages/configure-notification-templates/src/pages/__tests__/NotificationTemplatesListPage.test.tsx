// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, renderHook, screen, waitFor} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
import NotificationTemplatesListPage from '@/pages/NotificationTemplatesListPage';

const {mockParams, mockNavigate} = vi.hoisted(() => ({
  mockParams: {current: {channel: 'email'} as Record<string, string | undefined>},
  mockNavigate: vi.fn(),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useNavigate: () => mockNavigate, useParams: () => mockParams.current};
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), warn: vi.fn(), debug: vi.fn()}),
}));

// Stub the inner list so this suite covers the page shell (title, subtitle, create action).
vi.mock('@/components/NotificationTemplatesList', () => ({
  default: ({channel}: {channel: string}) => <div data-testid="templates-list" data-channel={channel} />,
}));

describe('NotificationTemplatesListPage', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockParams.current = {channel: 'email'};
  });

  it('renders the email title, subtitle and list for the email channel', () => {
    render(<NotificationTemplatesListPage />);

    expect(screen.getByText(t('notificationTemplates:listing.title.email'))).toBeInTheDocument();
    expect(screen.getByText(t('notificationTemplates:listing.subtitle.email'))).toBeInTheDocument();
    expect(screen.getByTestId('templates-list')).toHaveAttribute('data-channel', 'email');
  });

  it('sets the document title for the email channel', async () => {
    render(<NotificationTemplatesListPage />);

    await waitFor(() => expect(document.title).toBe('Email templates | ThunderID'));
  });

  it('renders the SMS title and list for the sms channel', () => {
    mockParams.current = {channel: 'sms'};
    render(<NotificationTemplatesListPage />);

    expect(screen.getByText(t('notificationTemplates:listing.title.sms'))).toBeInTheDocument();
    expect(screen.getByTestId('templates-list')).toHaveAttribute('data-channel', 'sms');
  });

  it('redirects to the canonical list for an unknown route param instead of rendering a channel', () => {
    mockParams.current = {channel: 'not-a-channel'};
    render(<NotificationTemplatesListPage />);

    expect(screen.queryByTestId('templates-list')).not.toBeInTheDocument();
  });

  it('navigates to the create page for the channel when New template is clicked', async () => {
    const user = userEvent.setup();
    render(<NotificationTemplatesListPage />);

    await user.click(screen.getByTestId('notification-template-add-button'));

    expect(mockNavigate).toHaveBeenCalledWith('/notification-templates/email/create');
  });
});
