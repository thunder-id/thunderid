// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import NotificationTemplatePreview from '../NotificationTemplatePreview';

interface UseGetTranslationsResult {
  data: {translations: Record<string, Record<string, string>>} | undefined;
  isSuccess: boolean;
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

function setTranslations(notification: Record<string, string> = {}, isSuccess = true): void {
  mockUseGetTranslations.mockReturnValue({data: {translations: {notification}}, isSuccess});
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
        body="<p>{{ctx(otpCode)}}</p>"
        locale="en-US"
      />,
    );

    expect(screen.getByText('Your verification code')).toBeInTheDocument();
    expect(emailSrcDoc()).toContain('{{ctx(otpCode)}}');
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
        body="<p>{{ctx(otpCode)}}</p>"
        locale="en-US"
      />,
    );

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('warns about an unsupported token', () => {
    const onWarningsChange = vi.fn();
    render(
      <NotificationTemplatePreview
        channel="email"
        body="{{meta(application.name)}}"
        locale="en-US"
        onWarningsChange={onWarningsChange}
      />,
    );

    expect(onWarningsChange).toHaveBeenLastCalledWith([
      {field: 'body', severity: 'error', code: 'unsupported', token: '{{meta(application.name)}}'},
    ]);
    // The editor shows warnings on the fields now; the preview no longer renders a banner.
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('warns when a design token is used in the subject', () => {
    const onWarningsChange = vi.fn();
    render(
      <NotificationTemplatePreview
        channel="email"
        subject="{{design(palette.primary.main)}}"
        body="hi"
        locale="en-US"
        onWarningsChange={onWarningsChange}
      />,
    );

    expect(onWarningsChange).toHaveBeenLastCalledWith([
      {field: 'subject', severity: 'error', code: 'designNotAllowed', token: '{{design(palette.primary.main)}}'},
    ]);
  });

  it('warns when a translation has no value for the locale', () => {
    const onWarningsChange = vi.fn();
    render(
      <NotificationTemplatePreview
        channel="email"
        body="{{t(missing.key)}}"
        locale="en-US"
        onWarningsChange={onWarningsChange}
      />,
    );

    expect(onWarningsChange).toHaveBeenLastCalledWith([
      {field: 'body', severity: 'error', code: 'missingTranslation', token: '{{t(missing.key)}}'},
    ]);
  });

  it('warns when an unknown context token is used', () => {
    const onWarningsChange = vi.fn();
    render(
      <NotificationTemplatePreview
        channel="email"
        body="{{ctx(unknownKey)}}"
        locale="en-US"
        contextKeys={new Set(['otpCode'])}
        onWarningsChange={onWarningsChange}
      />,
    );

    expect(onWarningsChange).toHaveBeenLastCalledWith([
      {field: 'body', severity: 'error', code: 'unknownContext', token: '{{ctx(unknownKey)}}'},
    ]);
  });

  it('leaves ctx tokens unchecked when no context catalog is provided', () => {
    render(<NotificationTemplatePreview channel="email" body="{{ctx(unknownKey)}}" locale="en-US" />);

    expect(screen.queryByText(/is not a recognized context value/i)).not.toBeInTheDocument();
  });

  it('suppresses missing-translation warnings until translations have loaded', () => {
    // Fetch still in flight: translations empty and not yet successful.
    setTranslations({}, false);

    render(<NotificationTemplatePreview channel="email" body="{{t(missing.key)}}" locale="en-US" />);

    expect(screen.queryByText(/No translation for/i)).not.toBeInTheDocument();
  });

  it('renders the resolved text for the SMS channel', () => {
    setTranslations({'sms.body': 'Your code'});

    render(<NotificationTemplatePreview channel="sms" body="{{t(sms.body)}} {{ctx(otpCode)}}" locale="en-US" />);

    expect(screen.getByText('Your code {{ctx(otpCode)}}')).toBeInTheDocument();
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
