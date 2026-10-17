// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, renderHook, screen} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
import TemplateContentEditor, {type TemplateContentEditorProps} from '@/components/edit-template/TemplateContentEditor';

// Stub the token catalog so this suite focuses on the content editor's own fields and the
// insert-at-cursor wiring. The stub exposes a button that inserts a fixed placeholder.
vi.mock('@/components/edit-template/TokenCatalog', () => ({
  default: ({onInsert}: {onInsert: (value: string) => void}) => (
    <button type="button" data-testid="insert-token" onClick={() => onInsert('{{ctx(otp.code)}}')}>
      insert
    </button>
  ),
}));

const baseProps: TemplateContentEditorProps = {
  channel: 'email',
  subject: 'Hello',
  body: 'Body text',
  colorScheme: 'light',
  onSubjectChange: vi.fn(),
  onBodyChange: vi.fn(),
  onColorSchemeChange: vi.fn(),
};

describe('TemplateContentEditor', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Email channel', () => {
    it('renders the color scheme toggle, subject and body fields', () => {
      render(<TemplateContentEditor {...baseProps} />);

      expect(screen.getByText(t('notificationTemplates:editor.design.colorScheme'))).toBeInTheDocument();
      expect(screen.getByText(t('notificationTemplates:editor.subject'))).toBeInTheDocument();
      expect(screen.getByText(t('notificationTemplates:editor.body'))).toBeInTheDocument();
    });

    it('calls onColorSchemeChange when the dark scheme is selected', async () => {
      const onColorSchemeChange = vi.fn();
      const user = userEvent.setup();
      render(<TemplateContentEditor {...baseProps} onColorSchemeChange={onColorSchemeChange} />);

      await user.click(screen.getByRole('button', {name: t('notificationTemplates:editor.design.dark')}));

      expect(onColorSchemeChange).toHaveBeenCalledWith('dark');
    });

    it('calls onSubjectChange when the subject changes', async () => {
      const onSubjectChange = vi.fn();
      const user = userEvent.setup();
      render(<TemplateContentEditor {...baseProps} subject="" onSubjectChange={onSubjectChange} />);

      // The subject input is the first textbox; the body is the multiline textarea after it.
      await user.type(screen.getAllByRole('textbox')[0], 'A');

      expect(onSubjectChange).toHaveBeenCalledWith('A');
    });
  });

  describe('SMS channel', () => {
    it('hides the color scheme toggle and subject for SMS', () => {
      render(<TemplateContentEditor {...baseProps} channel="sms" />);

      expect(screen.queryByText(t('notificationTemplates:editor.design.colorScheme'))).not.toBeInTheDocument();
      expect(screen.queryByText(t('notificationTemplates:editor.subject'))).not.toBeInTheDocument();
      expect(screen.getByText(t('notificationTemplates:editor.body'))).toBeInTheDocument();
    });
  });

  describe('Token insertion', () => {
    it('inserts a token into the body when the catalog requests it', async () => {
      const onBodyChange = vi.fn();
      const onSubjectChange = vi.fn();
      const user = userEvent.setup();
      render(
        <TemplateContentEditor
          {...baseProps}
          body="Body text"
          onBodyChange={onBodyChange}
          onSubjectChange={onSubjectChange}
        />,
      );

      await user.click(screen.getByTestId('insert-token'));

      expect(onBodyChange).toHaveBeenCalledWith(expect.stringContaining('{{ctx(otp.code)}}'));
      expect(onSubjectChange).not.toHaveBeenCalled();
    });
  });

  describe('Field warnings', () => {
    const subjectInput = (): HTMLElement => screen.getAllByRole('textbox')[0];
    const subjectFieldText = (): string => subjectInput().closest('.MuiFormControl-root')?.textContent ?? '';

    it('shows a subject warning on the subject field and marks it invalid', () => {
      render(
        <TemplateContentEditor
          {...baseProps}
          warnings={[
            {field: 'subject', severity: 'error', code: 'missingTranslation', token: '{{t(otp.email.subject1)}}'},
          ]}
        />,
      );

      expect(subjectFieldText()).toMatch(/No translation for/i);
      expect(subjectInput()).toHaveAttribute('aria-invalid', 'true');
    });

    it('shows a body warning below the body and leaves the subject untouched', () => {
      render(
        <TemplateContentEditor
          {...baseProps}
          warnings={[{field: 'body', severity: 'error', code: 'unsupported', token: '{{meta(x)}}'}]}
        />,
      );

      expect(screen.getByText(/is not a supported token/i)).toBeInTheDocument();
      expect(subjectFieldText()).not.toMatch(/is not a supported token/i);
      expect(subjectInput()).toHaveAttribute('aria-invalid', 'false');
    });

    it('shows a token that appears in both fields on each of them', () => {
      render(
        <TemplateContentEditor
          {...baseProps}
          warnings={[
            {field: 'subject', severity: 'error', code: 'unsupported', token: '{{meta(x)}}'},
            {field: 'body', severity: 'error', code: 'unsupported', token: '{{meta(x)}}'},
          ]}
        />,
      );

      expect(screen.getAllByText(/is not a supported token/i)).toHaveLength(2);
    });

    it('keeps the field out of the error state when it only has warning-severity messages', () => {
      render(
        <TemplateContentEditor
          {...baseProps}
          warnings={[
            {field: 'subject', severity: 'warning', code: 'designUnresolved', token: '{{design(palette.unknown)}}'},
          ]}
        />,
      );

      expect(subjectFieldText()).toMatch(/does not match any value/i);
      expect(subjectInput()).toHaveAttribute('aria-invalid', 'false');
    });
  });
});
