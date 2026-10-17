// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import type {TemplateWarning} from '@thunderid/configure-design';
import {render, renderHook, screen} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
import TemplateEditor from '@/components/edit-template/TemplateEditor';
import type {EmailTemplate, SmsTemplate} from '@/models/notification-template';

const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useNavigate: () => mockNavigate};
});

const {mockUseUpdate, mockMutate} = vi.hoisted(() => ({
  mockUseUpdate: vi.fn(),
  mockMutate: vi.fn(),
}));
vi.mock('@/api/useUpdateNotificationTemplate', () => ({default: mockUseUpdate}));

// Stub the config + preview panes; this suite covers the editor's header editing, save payload, and
// the warnings hand-off: the preview stub reports a warning and the content stub lists what it gets.
vi.mock('@/components/edit-template/TemplateContentEditor', () => ({
  default: ({warnings = []}: {warnings?: TemplateWarning[]}) => (
    <div data-testid="content-editor">{warnings.map((w) => `${w.field}:${w.code}`).join(',')}</div>
  ),
}));
vi.mock('@/components/edit-template/TemplatePreviewPanel', () => ({
  default: ({onWarningsChange}: {onWarningsChange?: (warnings: TemplateWarning[]) => void}) => (
    <button
      type="button"
      data-testid="preview-panel"
      onClick={() =>
        onWarningsChange?.([{field: 'subject', severity: 'error', code: 'missingTranslation', token: '{{t(x)}}'}])
      }
    >
      report warnings
    </button>
  ),
}));

const emailTemplate: EmailTemplate = {
  id: 'tmpl-1',
  ouId: 'ou-default',
  handle: 'otp-verification',
  displayName: 'OTP Verification',
  description: 'Sent to verify a login',
  design: {colorScheme: 'light'},
  content: {subject: 'Your code', body: '<p>{{ctx(otp.code)}}</p>'},
};

const smsTemplate: SmsTemplate = {
  id: 'sms-1',
  ouId: 'ou-default',
  handle: 'otp-sms',
  displayName: 'OTP SMS',
  content: {body: 'Your code is {{ctx(otp.code)}}'},
};

describe('TemplateEditor', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseUpdate.mockReturnValue({mutate: mockMutate, isPending: false, reset: vi.fn()});
  });

  // The save/reset affordance lives in the unsaved-changes bar, which only appears once a field
  // changes. Rename the template (the simplest reachable edit) to surface it.
  const makeDirty = async (
    user: ReturnType<typeof userEvent.setup>,
    currentName: string,
    newName: string,
  ): Promise<void> => {
    await user.click(screen.getAllByRole('button', {name: t('common:actions.edit')})[0]);
    const nameInput = screen.getByDisplayValue(currentName);
    await user.clear(nameInput);
    await user.type(nameInput, `${newName}{Enter}`);
  };

  describe('Rendering', () => {
    it('renders the template name and description', () => {
      render(<TemplateEditor channel="email" template={emailTemplate} />);

      expect(screen.getByText('OTP Verification')).toBeInTheDocument();
      expect(screen.getByText('Sent to verify a login')).toBeInTheDocument();
    });

    it('shows the empty-description placeholder when there is no description', () => {
      render(<TemplateEditor channel="sms" template={smsTemplate} />);

      expect(screen.getByText(t('notificationTemplates:editor.description.empty'))).toBeInTheDocument();
    });
  });

  describe('Inline name editing', () => {
    it('commits an edited name on Enter', async () => {
      const user = userEvent.setup();
      render(<TemplateEditor channel="email" template={emailTemplate} />);

      // The first edit icon belongs to the name.
      await user.click(screen.getAllByRole('button', {name: t('common:actions.edit')})[0]);
      const input = screen.getByDisplayValue('OTP Verification');
      await user.clear(input);
      await user.type(input, 'Renamed Template{Enter}');

      expect(screen.getByText('Renamed Template')).toBeInTheDocument();
    });

    it('discards an edited name on Escape', async () => {
      const user = userEvent.setup();
      render(<TemplateEditor channel="email" template={emailTemplate} />);

      await user.click(screen.getAllByRole('button', {name: t('common:actions.edit')})[0]);
      const input = screen.getByDisplayValue('OTP Verification');
      await user.clear(input);
      await user.type(input, 'Discarded{Escape}');

      expect(screen.getByText('OTP Verification')).toBeInTheDocument();
      expect(screen.queryByText('Discarded')).not.toBeInTheDocument();
    });

    it('keeps the previous name when the edit is committed empty', async () => {
      const user = userEvent.setup();
      render(<TemplateEditor channel="email" template={emailTemplate} />);

      await user.click(screen.getAllByRole('button', {name: t('common:actions.edit')})[0]);
      const input = screen.getByDisplayValue('OTP Verification');
      await user.clear(input);
      await user.type(input, '{Enter}');

      expect(screen.getByText('OTP Verification')).toBeInTheDocument();
    });
  });

  describe('Inline description editing', () => {
    it('commits an edited description on blur', async () => {
      const user = userEvent.setup();
      render(<TemplateEditor channel="email" template={emailTemplate} />);

      // The second edit icon belongs to the description.
      await user.click(screen.getAllByRole('button', {name: t('common:actions.edit')})[1]);
      const input = screen.getByDisplayValue('Sent to verify a login');
      await user.clear(input);
      await user.type(input, 'Updated description');
      await user.tab();

      expect(screen.getByText('Updated description')).toBeInTheDocument();
    });

    it('discards an edited description on Escape', async () => {
      const user = userEvent.setup();
      render(<TemplateEditor channel="email" template={emailTemplate} />);

      await user.click(screen.getAllByRole('button', {name: t('common:actions.edit')})[1]);
      const input = screen.getByDisplayValue('Sent to verify a login');
      await user.clear(input);
      await user.type(input, 'Throwaway{Escape}');

      expect(screen.getByText('Sent to verify a login')).toBeInTheDocument();
      expect(screen.queryByText('Throwaway')).not.toBeInTheDocument();
    });
  });

  describe('Save', () => {
    it('shows the unsaved-changes bar only after a field changes', async () => {
      const user = userEvent.setup();
      render(<TemplateEditor channel="email" template={emailTemplate} />);

      // No save affordance on a pristine form.
      expect(screen.queryByRole('button', {name: t('notificationTemplates:editor.save')})).not.toBeInTheDocument();

      await makeDirty(user, 'OTP Verification', 'OTP Verification v2');

      expect(screen.getByRole('button', {name: t('notificationTemplates:editor.save')})).toBeInTheDocument();
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });

    it('saves an email template with the design color scheme and full content', async () => {
      const user = userEvent.setup();
      render(<TemplateEditor channel="email" template={emailTemplate} />);

      await makeDirty(user, 'OTP Verification', 'OTP Verification v2');
      await user.click(screen.getByRole('button', {name: t('notificationTemplates:editor.save')}));

      expect(mockMutate).toHaveBeenCalledWith(
        {
          channel: 'email',
          id: 'tmpl-1',
          data: {
            displayName: 'OTP Verification v2',
            description: 'Sent to verify a login',
            design: {colorScheme: 'light'},
            content: {subject: 'Your code', body: '<p>{{ctx(otp.code)}}</p>'},
          },
        },
        expect.objectContaining({onError: expect.any(Function) as unknown as () => void}),
      );
    });

    it('saves an SMS template without design or subject', async () => {
      const user = userEvent.setup();
      render(<TemplateEditor channel="sms" template={smsTemplate} />);

      await makeDirty(user, 'OTP SMS', 'OTP SMS v2');
      await user.click(screen.getByRole('button', {name: t('notificationTemplates:editor.save')}));

      expect(mockMutate).toHaveBeenCalledWith(
        {
          channel: 'sms',
          id: 'sms-1',
          data: {
            displayName: 'OTP SMS v2',
            description: undefined,
            content: {body: 'Your code is {{ctx(otp.code)}}'},
          },
        },
        expect.objectContaining({onError: expect.any(Function) as unknown as () => void}),
      );
    });

    it('shows a resolved error alert when the save fails, not raw server text', async () => {
      mockMutate.mockImplementation((_vars: unknown, opts: {onError: (err: Error) => void}) => {
        opts.onError(new Error('raw server text'));
      });
      const user = userEvent.setup();
      render(<TemplateEditor channel="email" template={emailTemplate} />);

      await makeDirty(user, 'OTP Verification', 'OTP Verification v2');
      await user.click(screen.getByRole('button', {name: t('notificationTemplates:editor.save')}));

      expect(screen.getByText('Failed to save template. Please try again.')).toBeInTheDocument();
      expect(screen.queryByText('raw server text')).not.toBeInTheDocument();
    });

    it('resets edited fields to the loaded values', async () => {
      const user = userEvent.setup();
      render(<TemplateEditor channel="email" template={emailTemplate} />);

      await makeDirty(user, 'OTP Verification', 'OTP Verification v2');
      expect(screen.getByText('OTP Verification v2')).toBeInTheDocument();

      await user.click(screen.getByRole('button', {name: 'Reset'}));

      expect(screen.getByText('OTP Verification')).toBeInTheDocument();
      // The bar is gone once the form matches the loaded template again.
      expect(screen.queryByRole('button', {name: t('notificationTemplates:editor.save')})).not.toBeInTheDocument();
    });

    it('shows the saving label while a save is in flight', async () => {
      mockUseUpdate.mockReturnValue({mutate: mockMutate, isPending: true, reset: vi.fn()});
      const user = userEvent.setup();
      render(<TemplateEditor channel="email" template={emailTemplate} />);

      await makeDirty(user, 'OTP Verification', 'OTP Verification v2');

      const saveButton = screen.getByText(t('common:status.saving')).closest('button');
      expect(saveButton).toBeDisabled();
    });
  });

  describe('Navigation', () => {
    it('navigates back to the channel list when back is clicked', async () => {
      const user = userEvent.setup();
      render(<TemplateEditor channel="email" template={emailTemplate} />);

      await user.click(screen.getByRole('button', {name: t('notificationTemplates:editor.back')}));

      expect(mockNavigate).toHaveBeenCalledWith('/notification-templates/email');
    });
  });

  describe('Warnings', () => {
    it('passes the warnings reported by the preview to the content editor', async () => {
      const user = userEvent.setup();
      render(<TemplateEditor channel="email" template={emailTemplate} />);

      expect(screen.getByTestId('content-editor')).toBeEmptyDOMElement();

      await user.click(screen.getByTestId('preview-panel'));

      expect(screen.getByTestId('content-editor')).toHaveTextContent('subject:missingTranslation');
    });
  });
});
