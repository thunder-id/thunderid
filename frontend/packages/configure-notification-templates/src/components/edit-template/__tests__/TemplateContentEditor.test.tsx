// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, renderHook, screen} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
import TemplateContentEditor, {
  type TemplateContentEditorProps,
} from '@/components/edit-template/TemplateContentEditor';

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
});
