// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, renderHook, screen} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
import TemplatePreviewPanel from '@/components/edit-template/TemplatePreviewPanel';

const {mockUseGetTheme, mockPreview} = vi.hoisted(() => ({
  mockUseGetTheme: vi.fn<(id: string) => {data?: {theme?: Record<string, unknown>}}>(),
  mockPreview: vi.fn(),
}));

// Stub the shared preview so the panel's own wiring (selectors → preview props) is asserted in
// isolation, without exercising token resolution again.
vi.mock('@thunderid/configure-design', () => ({
  NotificationTemplatePreview: (props: Record<string, unknown>) => {
    mockPreview(props);
    return <div data-testid="preview" data-color-scheme={String(props.colorScheme)} data-locale={String(props.locale)} />;
  },
}));

vi.mock('@thunderid/design', () => ({
  useGetThemes: () => ({
    data: {themes: [{id: 'theme-1', displayName: 'Acrylic Orange'}]},
    isLoading: false,
  }),
  useGetTheme: (id: string) => mockUseGetTheme(id),
}));

vi.mock('@thunderid/i18n', () => ({
  I18nDefaultConstants: {FALLBACK_LANGUAGE: 'en-US'},
  useGetLanguages: () => ({data: {languages: ['fr-FR']}, isLoading: false}),
  getDisplayNameForCode: (code: string) => `Language(${code})`,
  toFlagEmoji: (code: string) => `Flag(${code})`,
}));

const defaultProps = {
  channel: 'email' as const,
  subject: 'Hello',
  body: '<p>Body</p>',
  colorScheme: 'light' as const,
};

describe('TemplatePreviewPanel', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetTheme.mockReturnValue({data: undefined});
  });

  describe('Email channel', () => {
    it('renders both the language and design-theme selectors', () => {
      render(<TemplatePreviewPanel {...defaultProps} />);

      expect(screen.getByText(t('notificationTemplates:editor.preview.language'))).toBeInTheDocument();
      expect(screen.getByText(t('notificationTemplates:editor.preview.designTheme'))).toBeInTheDocument();
    });

    it('passes the body, color scheme and default locale to the preview', () => {
      render(<TemplatePreviewPanel {...defaultProps} colorScheme="dark" />);

      expect(mockPreview).toHaveBeenCalledWith(
        expect.objectContaining({channel: 'email', body: '<p>Body</p>', colorScheme: 'dark', locale: 'en-US'}),
      );
    });

    it('resolves and forwards the selected theme to the preview', async () => {
      mockUseGetTheme.mockReturnValue({data: {theme: {colorSchemes: {light: {palette: {}}}}}});
      const user = userEvent.setup();
      render(<TemplatePreviewPanel {...defaultProps} />);

      await user.click(screen.getByText(t('notificationTemplates:editor.preview.designThemeDefault')));
      await user.click(screen.getByRole('option', {name: 'Acrylic Orange'}));

      expect(mockUseGetTheme).toHaveBeenCalledWith('theme-1');
      expect(mockPreview).toHaveBeenLastCalledWith(
        expect.objectContaining({theme: {colorSchemes: {light: {palette: {}}}}}),
      );
    });
  });

  describe('SMS channel', () => {
    it('renders the language selector but not the design-theme selector', () => {
      render(<TemplatePreviewPanel {...defaultProps} channel="sms" subject={undefined} />);

      expect(screen.getByText(t('notificationTemplates:editor.preview.language'))).toBeInTheDocument();
      expect(screen.queryByText(t('notificationTemplates:editor.preview.designTheme'))).not.toBeInTheDocument();
    });
  });
});
