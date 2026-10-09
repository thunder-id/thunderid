// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, renderHook, screen} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
import NotificationTemplateCreatePage from '@/pages/NotificationTemplateCreatePage';

const {mockParams, mockNavigate, mockUseCreate, mockMutate, mockUseGetTemplates} = vi.hoisted(() => ({
  mockParams: {current: {channel: 'email'} as Record<string, string | undefined>},
  mockNavigate: vi.fn(),
  mockUseCreate: vi.fn(),
  mockMutate: vi.fn(),
  mockUseGetTemplates: vi.fn(),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useNavigate: () => mockNavigate, useParams: () => mockParams.current};
});

vi.mock('@/api/useCreateNotificationTemplate', () => ({default: mockUseCreate}));
vi.mock('@/api/useGetNotificationTemplates', () => ({default: mockUseGetTemplates}));

// Minimal layout: render the step content and the footer (which holds the wizard buttons).
vi.mock('@thunderid/components', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/components')>()),
  FullScreenCreationWizardLayout: ({children, footer}: {children: React.ReactNode; footer: React.ReactNode}) => (
    <div>
      {children}
      {footer}
    </div>
  ),
}));

// Step-1 stub: a button that marks identity ready and sets the name + handle the parent will submit.
vi.mock('@/components/create-template/ConfigureTemplateIdentity', () => ({
  default: ({
    onReadyChange,
    onDisplayNameChange,
    onHandleChange,
  }: {
    onReadyChange: (v: boolean) => void;
    onDisplayNameChange: (v: string) => void;
    onHandleChange: (v: string) => void;
  }) => (
    <button
      type="button"
      data-testid="fill-identity"
      onClick={() => {
        onDisplayNameChange('OTP Verification');
        onHandleChange('otp-verification');
        onReadyChange(true);
      }}
    >
      fill identity
    </button>
  ),
}));

// Step-2 stubs: a button that fills the content; the preview is inert.
vi.mock('@/components/edit-template/TemplateContentEditor', () => ({
  default: ({
    onSubjectChange,
    onBodyChange,
  }: {
    onSubjectChange: (v: string) => void;
    onBodyChange: (v: string) => void;
  }) => (
    <button
      type="button"
      data-testid="fill-content"
      onClick={() => {
        onSubjectChange('Your code');
        onBodyChange('<p>Body</p>');
      }}
    >
      fill content
    </button>
  ),
}));
vi.mock('@/components/edit-template/TemplatePreviewPanel', () => ({default: () => <div data-testid="preview" />}));

describe('NotificationTemplateCreatePage', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockParams.current = {channel: 'email'};
    mockUseCreate.mockReturnValue({mutate: mockMutate, isPending: false});
    mockUseGetTemplates.mockReturnValue({data: {templates: []}});
  });

  const continueBtn = (): HTMLElement => screen.getByRole('button', {name: t('common:actions.continue')});
  const createBtn = (): HTMLElement => screen.getByRole('button', {name: t('notificationTemplates:create.submit')});

  it('keeps the Continue action disabled until the identity step is ready', async () => {
    const user = userEvent.setup();
    render(<NotificationTemplateCreatePage />);

    expect(continueBtn()).toBeDisabled();

    await user.click(screen.getByTestId('fill-identity'));

    expect(continueBtn()).toBeEnabled();
  });

  it('advances to the content step and submits the full email payload, then opens the new template', async () => {
    mockMutate.mockImplementation((_vars: unknown, opts: {onSuccess: (tpl: {id: string}) => void}) => {
      opts.onSuccess({id: 'new-id'});
    });
    const user = userEvent.setup();
    render(<NotificationTemplateCreatePage />);

    await user.click(screen.getByTestId('fill-identity'));
    await user.click(continueBtn());

    // Content step: Create stays disabled until the body (and email subject) are filled.
    expect(createBtn()).toBeDisabled();
    await user.click(screen.getByTestId('fill-content'));
    expect(createBtn()).toBeEnabled();

    await user.click(createBtn());

    expect(mockMutate).toHaveBeenCalledWith(
      {
        channel: 'email',
        data: {
          handle: 'otp-verification',
          displayName: 'OTP Verification',
          design: {colorScheme: 'light'},
          content: {subject: 'Your code', body: '<p>Body</p>'},
        },
      },
      expect.objectContaining({
        onSuccess: expect.any(Function) as unknown as () => void,
        onError: expect.any(Function) as unknown as () => void,
      }),
    );
    expect(mockNavigate).toHaveBeenCalledWith('/notification-templates/email/new-id');
  });

  it('submits an SMS payload without design or subject', async () => {
    mockParams.current = {channel: 'sms'};
    const user = userEvent.setup();
    render(<NotificationTemplateCreatePage />);

    await user.click(screen.getByTestId('fill-identity'));
    await user.click(continueBtn());
    await user.click(screen.getByTestId('fill-content'));
    await user.click(createBtn());

    expect(mockMutate).toHaveBeenCalledWith(
      {
        channel: 'sms',
        data: {handle: 'otp-verification', displayName: 'OTP Verification', content: {body: '<p>Body</p>'}},
      },
      expect.anything(),
    );
  });

  it('shows a resolved error alert when creation fails, not raw server text', async () => {
    mockMutate.mockImplementation((_vars: unknown, opts: {onError: (err: Error) => void}) => {
      opts.onError(new Error('raw server text'));
    });
    const user = userEvent.setup();
    render(<NotificationTemplateCreatePage />);

    await user.click(screen.getByTestId('fill-identity'));
    await user.click(continueBtn());
    await user.click(screen.getByTestId('fill-content'));
    await user.click(createBtn());

    expect(screen.getByText('Failed to create template. Please try again.')).toBeInTheDocument();
    expect(screen.queryByText('raw server text')).not.toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('navigates back to the channel list when cancel is clicked on the identity step', async () => {
    const user = userEvent.setup();
    render(<NotificationTemplateCreatePage />);

    await user.click(screen.getByRole('button', {name: t('common:actions.cancel')}));

    expect(mockNavigate).toHaveBeenCalledWith('/notification-templates/email');
  });
});
