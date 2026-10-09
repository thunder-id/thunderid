// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, renderHook, screen} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
import TokenCatalog from '@/components/edit-template/TokenCatalog';

const {mockUpdateMutate, mockUseGetTranslations} = vi.hoisted(() => ({
  mockUpdateMutate: vi.fn(),
  mockUseGetTranslations: vi.fn<() => {data?: {translations?: Record<string, Record<string, string>>}}>(),
}));

vi.mock('@thunderid/i18n', () => ({
  I18nDefaultConstants: {FALLBACK_LANGUAGE: 'en-US'},
  useGetTranslations: () => mockUseGetTranslations(),
  useUpdateTranslation: () => ({mutate: mockUpdateMutate, isPending: false}),
}));

const defaultProps = {
  channel: 'email' as const,
  onInsert: vi.fn(),
  collapsed: false,
  onToggleCollapse: vi.fn(),
};

describe('TokenCatalog', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetTranslations.mockReturnValue({
      data: {translations: {notification: {'otp.subject': 'Your code', 'otp.body': 'Body'}}},
    });
  });

  describe('Rendering', () => {
    it('renders the translation, design and context token groups for an email template', () => {
      render(<TokenCatalog {...defaultProps} channel="email" />);

      expect(screen.getByText(t('notificationTemplates:editor.tokens.translation'))).toBeInTheDocument();
      expect(screen.getByText(t('notificationTemplates:editor.tokens.design'))).toBeInTheDocument();
      expect(screen.getByText(t('notificationTemplates:editor.tokens.context'))).toBeInTheDocument();
      expect(screen.getByText('Primary color')).toBeInTheDocument();
      expect(screen.getByText('App name')).toBeInTheDocument();
    });

    it('does not render design tokens for an SMS template', () => {
      render(<TokenCatalog {...defaultProps} channel="sms" />);

      expect(screen.queryByText(t('notificationTemplates:editor.tokens.design'))).not.toBeInTheDocument();
      expect(screen.queryByText('Primary color')).not.toBeInTheDocument();
      expect(screen.getByText(t('notificationTemplates:editor.tokens.context'))).toBeInTheDocument();
    });

    it('lists translation keys loaded from the i18n API', () => {
      render(<TokenCatalog {...defaultProps} />);

      expect(screen.getByText('otp.subject')).toBeInTheDocument();
      expect(screen.getByText('otp.body')).toBeInTheDocument();
    });

    it('renders only the expand control when collapsed', () => {
      render(<TokenCatalog {...defaultProps} collapsed />);

      expect(screen.getByLabelText(t('notificationTemplates:editor.tokens.expand'))).toBeInTheDocument();
      expect(screen.queryByText('Primary color')).not.toBeInTheDocument();
    });
  });

  describe('Insertion', () => {
    it('inserts a design token placeholder when clicked', async () => {
      const onInsert = vi.fn();
      const user = userEvent.setup();
      render(<TokenCatalog {...defaultProps} onInsert={onInsert} />);

      await user.click(screen.getByText('Primary color'));

      expect(onInsert).toHaveBeenCalledWith('{{design(palette.primary.main)}}');
    });

    it('inserts a context token placeholder when clicked', async () => {
      const onInsert = vi.fn();
      const user = userEvent.setup();
      render(<TokenCatalog {...defaultProps} onInsert={onInsert} />);

      await user.click(screen.getByText('App name'));

      expect(onInsert).toHaveBeenCalledWith('{{ctx(app.name)}}');
    });

    it('inserts a translation token placeholder when clicked', async () => {
      const onInsert = vi.fn();
      const user = userEvent.setup();
      render(<TokenCatalog {...defaultProps} onInsert={onInsert} />);

      await user.click(screen.getByText('otp.subject'));

      expect(onInsert).toHaveBeenCalledWith('{{t(otp.subject)}}');
    });
  });

  describe('Search', () => {
    it('filters tokens to those matching the query', async () => {
      const user = userEvent.setup();
      render(<TokenCatalog {...defaultProps} />);

      await user.type(screen.getByPlaceholderText(t('notificationTemplates:editor.tokens.searchPlaceholder')), 'primary');

      expect(screen.getByText('Primary color')).toBeInTheDocument();
      expect(screen.queryByText('App name')).not.toBeInTheDocument();
    });
  });

  describe('Add translation key', () => {
    const openAddForm = async (user: ReturnType<typeof userEvent.setup>): Promise<void> => {
      await user.click(screen.getByLabelText(t('notificationTemplates:editor.tokens.add.title')));
    };

    it('rejects a key that violates the token grammar and does not submit', async () => {
      const user = userEvent.setup();
      render(<TokenCatalog {...defaultProps} />);

      await openAddForm(user);
      await user.type(screen.getByPlaceholderText('Key (e.g. otp.subject)'), 'bad key!');
      await user.type(screen.getByPlaceholderText('Translation value'), 'A value');
      await user.click(screen.getByText(t('notificationTemplates:editor.tokens.add.submit')));

      expect(screen.getByText('Use only letters, numbers, dots, hyphens and underscores.')).toBeInTheDocument();
      expect(mockUpdateMutate).not.toHaveBeenCalled();
    });

    it('submits a valid key against the base locale and notification namespace', async () => {
      const user = userEvent.setup();
      render(<TokenCatalog {...defaultProps} />);

      await openAddForm(user);
      await user.type(screen.getByPlaceholderText('Key (e.g. otp.subject)'), 'order.shipped');
      await user.type(screen.getByPlaceholderText('Translation value'), 'Your order shipped');
      await user.click(screen.getByText(t('notificationTemplates:editor.tokens.add.submit')));

      expect(mockUpdateMutate).toHaveBeenCalledWith(
        {language: 'en-US', namespace: 'notification', key: 'order.shipped', value: 'Your order shipped'},
        expect.objectContaining({
          onSuccess: expect.any(Function) as unknown as () => void,
          onError: expect.any(Function) as unknown as () => void,
        }),
      );
    });

    it('shows a resolved error alert when adding a key fails, not raw server text', async () => {
      mockUpdateMutate.mockImplementation((_vars: unknown, opts: {onError: (err: Error) => void}) => {
        opts.onError(new Error('raw server text'));
      });
      const user = userEvent.setup();
      render(<TokenCatalog {...defaultProps} />);

      await openAddForm(user);
      await user.type(screen.getByPlaceholderText('Key (e.g. otp.subject)'), 'order.shipped');
      await user.type(screen.getByPlaceholderText('Translation value'), 'Your order shipped');
      await user.click(screen.getByText(t('notificationTemplates:editor.tokens.add.submit')));

      expect(screen.getByText('Failed to add translation key.')).toBeInTheDocument();
      expect(screen.queryByText('raw server text')).not.toBeInTheDocument();
    });
  });
});
