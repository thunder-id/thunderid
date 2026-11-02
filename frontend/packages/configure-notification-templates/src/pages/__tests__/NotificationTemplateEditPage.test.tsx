// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, renderHook, screen} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
import NotificationTemplateEditPage from '@/pages/NotificationTemplateEditPage';

const {mockParams, mockUseGetTemplate} = vi.hoisted(() => ({
  mockParams: {current: {channel: 'email', id: 'tmpl-1'} as Record<string, string | undefined>},
  mockUseGetTemplate: vi.fn(),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useParams: () => mockParams.current};
});

vi.mock('@/api/useGetNotificationTemplate', () => ({default: mockUseGetTemplate}));

// Stub the editor so the page's own loading/error/guard branches are isolated.
vi.mock('@/components/edit-template/TemplateEditor', () => ({
  default: ({channel, template}: {channel: string; template: {id: string}}) => (
    <div data-testid="template-editor" data-channel={channel} data-template-id={template.id} />
  ),
}));

describe('NotificationTemplateEditPage', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockParams.current = {channel: 'email', id: 'tmpl-1'};
    mockUseGetTemplate.mockReturnValue({data: undefined, isLoading: false, error: null, refetch: vi.fn()});
  });

  it('renders the editor once the template has loaded', () => {
    mockUseGetTemplate.mockReturnValue({
      data: {id: 'tmpl-1', handle: 'otp', displayName: 'OTP', content: {subject: 's', body: 'b'}},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
    render(<NotificationTemplateEditPage />);

    const editor = screen.getByTestId('template-editor');
    expect(editor).toHaveAttribute('data-channel', 'email');
    expect(editor).toHaveAttribute('data-template-id', 'tmpl-1');
  });

  it('shows a spinner while the template is loading', () => {
    mockUseGetTemplate.mockReturnValue({data: undefined, isLoading: true, error: null, refetch: vi.fn()});
    render(<NotificationTemplateEditPage />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
    expect(screen.queryByTestId('template-editor')).not.toBeInTheDocument();
  });

  it('shows the load-error notice when the query fails', () => {
    mockUseGetTemplate.mockReturnValue({data: undefined, isLoading: false, error: new Error('boom'), refetch: vi.fn()});
    render(<NotificationTemplateEditPage />);

    expect(screen.getByText(t('notificationTemplates:editor.loadError'))).toBeInTheDocument();
    expect(screen.queryByTestId('template-editor')).not.toBeInTheDocument();
  });

  it('shows the load-error notice when the route has no id', () => {
    mockParams.current = {channel: 'email', id: undefined};
    render(<NotificationTemplateEditPage />);

    expect(screen.getByText(t('notificationTemplates:editor.loadError'))).toBeInTheDocument();
    expect(screen.queryByTestId('template-editor')).not.toBeInTheDocument();
  });

  it('treats an sms route param as the sms channel', () => {
    mockParams.current = {channel: 'sms', id: 'sms-1'};
    mockUseGetTemplate.mockReturnValue({
      data: {id: 'sms-1', handle: 'otp', displayName: 'OTP', content: {body: 'b'}},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
    render(<NotificationTemplateEditPage />);

    expect(screen.getByTestId('template-editor')).toHaveAttribute('data-channel', 'sms');
  });
});
