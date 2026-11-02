// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, renderHook, screen} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
import NotificationTemplateCreatePage from '@/pages/NotificationTemplateCreatePage';

const {mockParams, mockNavigate, mockUseCreate, mockMutate, mockUseGetTemplates, mockUseHasMultipleOUs, mockUseGetOU} =
  vi.hoisted(() => ({
    mockParams: {current: {channel: 'email'} as Record<string, string | undefined>},
    mockNavigate: vi.fn(),
    mockUseCreate: vi.fn(),
    mockMutate: vi.fn(),
    mockUseGetTemplates: vi.fn(),
    mockUseHasMultipleOUs: vi.fn(),
    mockUseGetOU: vi.fn(),
  }));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useNavigate: () => mockNavigate, useParams: () => mockParams.current};
});

vi.mock('@/api/useCreateNotificationTemplate', () => ({default: mockUseCreate}));
vi.mock('@/api/useGetNotificationTemplates', () => ({default: mockUseGetTemplates}));

// OU wizard dependencies: the picker screen is a stub exposing a button that selects an OU.
vi.mock('@thunderid/configure-organization-units', () => ({
  useHasMultipleOUs: mockUseHasMultipleOUs,
  useGetOrganizationUnit: mockUseGetOU,
  OrganizationUnitPickerScreen: ({
    value,
    onChange,
    onContinue,
  }: {
    value: string;
    onChange: (v: string) => void;
    onContinue: () => void;
  }) => (
    <div>
      <span data-testid="ou-step" />
      <button type="button" data-testid="pick-ou" onClick={() => onChange('ou-picked')}>
        {value === '' ? 'pick' : value}
      </button>
      <button type="button" data-testid="ou-continue" onClick={onContinue}>
        ou-continue
      </button>
    </div>
  ),
}));

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

// Step-1 stub: buttons that pick the blank option or a sample (with simple, assertable content).
vi.mock('@/components/create-template/SelectNotificationTemplate', () => ({
  default: ({
    onSelect,
  }: {
    onSelect: (selection: {kind: 'blank'} | {kind: 'sample'; sample: Record<string, unknown>}) => void;
  }) => (
    <div>
      <button type="button" data-testid="select-blank" onClick={() => onSelect({kind: 'blank'})}>
        blank
      </button>
      <button
        type="button"
        data-testid="select-sample"
        onClick={() =>
          onSelect({
            kind: 'sample',
            sample: {
              id: 'email-sample',
              channel: 'email',
              display: {label: 'Sample', description: 'desc'},
              suggestedName: 'Sample Name',
              subject: 'Sample subject',
              body: '<p>Sample body</p>',
              colorScheme: 'light',
            },
          })
        }
      >
        sample
      </button>
    </div>
  ),
}));

// Step-2 stub: a button that marks identity ready and sets the name + handle the parent will submit.
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

// Step-3 stubs: a button that fills the content; the preview is inert.
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
    // Default: a single-OU deployment, so the OU step is skipped and the only OU is adopted.
    mockUseHasMultipleOUs.mockReturnValue({hasMultipleOUs: false, isLoading: false, ouList: [{id: 'ou-default'}]});
    mockUseGetOU.mockReturnValue({data: {id: 'ou-default', name: 'Default'}, isLoading: false});
  });

  const continueBtn = (): HTMLElement => screen.getByRole('button', {name: t('common:actions.continue')});
  const createBtn = (): HTMLElement => screen.getByRole('button', {name: t('notificationTemplates:create.submit')});

  /** Pick "start from scratch" and advance off the template step. */
  const startFromScratch = async (user: ReturnType<typeof userEvent.setup>): Promise<void> => {
    await user.click(screen.getByTestId('select-blank'));
    await user.click(continueBtn());
  };

  it('keeps Continue disabled on the template step until a starting point is chosen', async () => {
    const user = userEvent.setup();
    render(<NotificationTemplateCreatePage />);

    expect(continueBtn()).toBeDisabled();

    await user.click(screen.getByTestId('select-blank'));

    expect(continueBtn()).toBeEnabled();
  });

  it('keeps the Continue action disabled until the identity step is ready', async () => {
    const user = userEvent.setup();
    render(<NotificationTemplateCreatePage />);

    await startFromScratch(user);

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

    await startFromScratch(user);
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
          ouId: 'ou-default',
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

  it('pre-fills the content from a chosen sample without needing the content editor', async () => {
    const user = userEvent.setup();
    render(<NotificationTemplateCreatePage />);

    // Pick a sample (pre-fills name + content), then supply identity readiness via the stub.
    await user.click(screen.getByTestId('select-sample'));
    await user.click(continueBtn());
    await user.click(screen.getByTestId('fill-identity'));
    await user.click(continueBtn());

    // The sample already satisfied the content step, so Create is enabled with no fill-content click.
    expect(createBtn()).toBeEnabled();
    await user.click(createBtn());

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        channel: 'email',
        data: expect.objectContaining({
          design: {colorScheme: 'light'},
          content: {subject: 'Sample subject', body: '<p>Sample body</p>'},
        }) as unknown,
      }),
      expect.anything(),
    );
  });

  it('submits an SMS payload without design or subject', async () => {
    mockParams.current = {channel: 'sms'};
    const user = userEvent.setup();
    render(<NotificationTemplateCreatePage />);

    await startFromScratch(user);
    await user.click(screen.getByTestId('fill-identity'));
    await user.click(continueBtn());
    await user.click(screen.getByTestId('fill-content'));
    await user.click(createBtn());

    expect(mockMutate).toHaveBeenCalledWith(
      {
        channel: 'sms',
        data: {
          handle: 'otp-verification',
          ouId: 'ou-default',
          displayName: 'OTP Verification',
          content: {body: '<p>Body</p>'},
        },
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

    await startFromScratch(user);
    await user.click(screen.getByTestId('fill-identity'));
    await user.click(continueBtn());
    await user.click(screen.getByTestId('fill-content'));
    await user.click(createBtn());

    expect(screen.getByText('Failed to create template. Please try again.')).toBeInTheDocument();
    expect(screen.queryByText('raw server text')).not.toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('shows the OU step first on a multi-OU deployment and submits the picked ouId', async () => {
    mockUseHasMultipleOUs.mockReturnValue({
      hasMultipleOUs: true,
      isLoading: false,
      ouList: [{id: 'ou-a'}, {id: 'ou-b'}],
    });
    mockMutate.mockImplementation((_vars: unknown, opts: {onSuccess: (tpl: {id: string}) => void}) => {
      opts.onSuccess({id: 'new-id'});
    });
    const user = userEvent.setup();
    render(<NotificationTemplateCreatePage />);

    // The OU picker is the first screen; pick an OU and continue into the wizard.
    expect(screen.getByTestId('ou-step')).toBeInTheDocument();
    await user.click(screen.getByTestId('pick-ou'));
    await user.click(screen.getByTestId('ou-continue'));

    await startFromScratch(user);
    await user.click(screen.getByTestId('fill-identity'));
    await user.click(continueBtn());
    await user.click(screen.getByTestId('fill-content'));
    await user.click(createBtn());

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        channel: 'email',
        data: expect.objectContaining({ouId: 'ou-picked'}) as unknown,
      }),
      expect.anything(),
    );
  });

  it('navigates back to the channel list when cancel is clicked on the template step', async () => {
    const user = userEvent.setup();
    render(<NotificationTemplateCreatePage />);

    await user.click(screen.getByRole('button', {name: t('common:actions.cancel')}));

    expect(mockNavigate).toHaveBeenCalledWith('/notification-templates/email');
  });
});
