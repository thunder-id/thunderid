// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import NotificationTemplatePreview from '../NotificationTemplatePreview';

interface UseGetTranslationsResult {
  data: {translations: Record<string, Record<string, string>>} | undefined;
}

const {mockUseGetTranslations} = vi.hoisted(() => ({
  mockUseGetTranslations: vi.fn<() => UseGetTranslationsResult>(),
}));

vi.mock('@thunderid/i18n', () => ({
  useGetTranslations: () => mockUseGetTranslations(),
}));

vi.mock('@thunderid/design', () => ({
  DefaultTheme: {colorSchemes: {light: {palette: {primary: {main: '#3688ff'}}}}},
}));

function setTranslations(notification: Record<string, string> = {}): void {
  mockUseGetTranslations.mockReturnValue({data: {translations: {notification}}});
}

/** The email body is rendered into an iframe via srcDoc; read that back. */
function emailSrcDoc(): string {
  return document.querySelector('iframe[title="Email preview"]')?.getAttribute('srcdoc') ?? '';
}

describe('NotificationTemplatePreview', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setTranslations();
  });

  it('renders the email body as an iframe document', () => {
    render(<NotificationTemplatePreview channel="email" subject="Hi" body="<p>Body</p>" locale="en-US" />);

    expect(emailSrcDoc()).toContain('<p>Body</p>');
  });

  it('resolves {{t(key)}} against the selected locale and leaves {{ctx}} literal', () => {
    setTranslations({'otp.subject': 'Your verification code'});

    render(
      <NotificationTemplatePreview
        channel="email"
        subject="{{t(otp.subject)}}"
        body="<p>{{ctx(otp)}}</p>"
        locale="en-US"
      />,
    );

    expect(screen.getByText('Your verification code')).toBeInTheDocument();
    expect(emailSrcDoc()).toContain('{{ctx(otp)}}');
  });

  it('resolves {{design(key)}} against the theme in the email body', () => {
    render(
      <NotificationTemplatePreview channel="email" body="<p>{{design(palette.primary.main)}}</p>" locale="en-US" />,
    );

    expect(emailSrcDoc()).toContain('#3688ff');
  });

  it('shows no warnings for supported, resolvable tokens', () => {
    setTranslations({'otp.subject': 'Your verification code'});

    render(
      <NotificationTemplatePreview
        channel="email"
        subject="{{t(otp.subject)}}"
        body="<p>{{ctx(otp)}}</p>"
        locale="en-US"
      />,
    );

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('warns about an unsupported token', () => {
    render(<NotificationTemplatePreview channel="email" body="{{meta(application.name)}}" locale="en-US" />);

    expect(screen.getByText(/is not a supported token/i)).toBeInTheDocument();
  });

  it('warns when a design token is used in the subject', () => {
    render(
      <NotificationTemplatePreview
        channel="email"
        subject="{{design(palette.primary.main)}}"
        body="hi"
        locale="en-US"
      />,
    );

    expect(screen.getByText(/only allowed in the email body/i)).toBeInTheDocument();
  });

  it('warns when a translation has no value for the locale', () => {
    render(<NotificationTemplatePreview channel="email" body="{{t(missing.key)}}" locale="en-US" />);

    expect(screen.getByText(/No translation for/i)).toBeInTheDocument();
  });

  it('renders the resolved text for the SMS channel', () => {
    setTranslations({'sms.body': 'Your code'});

    render(<NotificationTemplatePreview channel="sms" body="{{t(sms.body)}} {{ctx(otp)}}" locale="en-US" />);

    expect(screen.getByText('Your code {{ctx(otp)}}')).toBeInTheDocument();
    expect(document.querySelector('iframe[title="Email preview"]')).toBeNull();
  });

  it('shows a spinner and no warnings while the selected theme is loading (theme=null)', () => {
    render(
      <NotificationTemplatePreview
        channel="email"
        body="<p>{{design(palette.primary.main)}}</p>"
        locale="en-US"
        theme={null}
      />,
    );

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(document.querySelector('iframe[title="Email preview"]')).toBeNull();
  });
});
