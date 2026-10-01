// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {AllowedOriginTypes, createRow} from '@thunderid/configure-settings';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import CorsOriginsEditor from '../CorsOriginsEditor';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

const ORIGIN_PLACEHOLDER = 'applications:onboarding.configure.details.corsOrigins.placeholder';
const REGEX_PLACEHOLDER = 'applications:onboarding.configure.details.corsOrigins.regexPlaceholder';

const origin = (value: string) => createRow(AllowedOriginTypes.ORIGIN, value);
const regex = (value: string) => createRow(AllowedOriginTypes.REGEX, value);

describe('CorsOriginsEditor', () => {
  let user: ReturnType<typeof userEvent.setup>;

  beforeEach(() => {
    vi.clearAllMocks();
    user = userEvent.setup();
  });

  it('shows one empty origin row when the list is empty', () => {
    render(<CorsOriginsEditor rows={[]} onRowsChange={vi.fn()} />);

    expect(screen.getByPlaceholderText(ORIGIN_PLACEHOLDER)).toHaveValue('');
    expect(screen.getByRole('combobox', {name: 'settings:cors.type.label'})).toHaveTextContent(
      'settings:cors.type.origin',
    );
  });

  it('renders a loaded regex row with its type preselected and its delimiters', () => {
    render(<CorsOriginsEditor rows={[regex('^https://x\\.io$')]} onRowsChange={vi.fn()} />);

    expect(screen.getByPlaceholderText(REGEX_PLACEHOLDER)).toHaveValue('^https://x\\.io$');
    expect(screen.getAllByText('/')).toHaveLength(2);
  });

  it('appends an origin row on Add Origin', async () => {
    const onRowsChange = vi.fn();
    render(<CorsOriginsEditor rows={[origin('https://a.example.com')]} onRowsChange={onRowsChange} />);

    await user.click(
      screen.getByRole('button', {name: 'applications:onboarding.configure.details.corsOrigins.addOrigin'}),
    );

    expect(onRowsChange).toHaveBeenCalledWith([
      expect.objectContaining({type: AllowedOriginTypes.ORIGIN, value: 'https://a.example.com'}),
      expect.objectContaining({type: AllowedOriginTypes.ORIGIN, value: ''}),
    ]);
  });

  it('keeps the text a row already holds when its type changes', async () => {
    const onRowsChange = vi.fn();
    render(<CorsOriginsEditor rows={[origin('https://app.example.com')]} onRowsChange={onRowsChange} />);

    await user.click(screen.getByRole('combobox', {name: 'settings:cors.type.label'}));
    await user.click(screen.getByRole('option', {name: 'settings:cors.type.regex'}));

    expect(onRowsChange).toHaveBeenCalledWith([
      expect.objectContaining({type: AllowedOriginTypes.REGEX, value: 'https://app.example.com'}),
    ]);
  });

  it('removes a row', async () => {
    const onRowsChange = vi.fn();
    render(<CorsOriginsEditor rows={[origin('https://a.example.com')]} onRowsChange={onRowsChange} />);

    await user.click(
      screen.getByRole('button', {name: 'applications:onboarding.configure.details.corsOrigins.removeOrigin'}),
    );

    expect(onRowsChange).toHaveBeenCalledWith([]);
  });

  it('reports a typed value for the edited row only, leaving its siblings as they are', async () => {
    const onRowsChange = vi.fn();
    const untouched = origin('https://a.example.com');
    render(<CorsOriginsEditor rows={[untouched, origin('')]} onRowsChange={onRowsChange} />);

    await user.type(screen.getAllByPlaceholderText(ORIGIN_PLACEHOLDER)[1], 'h');

    expect(onRowsChange).toHaveBeenCalledWith([untouched, expect.objectContaining({value: 'h'})]);
  });

  it('stays quiet until the row is blurred, then reports an invalid origin', async () => {
    render(<CorsOriginsEditor rows={[origin('https://example.com/path')]} onRowsChange={vi.fn()} />);

    expect(screen.queryByText('settings:cors.validation.invalidOrigin')).toBeNull();

    await user.click(screen.getByPlaceholderText(ORIGIN_PLACEHOLDER));
    await user.tab();

    expect(await screen.findByText('settings:cors.validation.invalidOrigin')).toBeInTheDocument();
  });

  it('reports a regex that does not compile', async () => {
    render(<CorsOriginsEditor rows={[regex('(bad')]} onRowsChange={vi.fn()} />);

    await user.click(screen.getByPlaceholderText(REGEX_PLACEHOLDER));
    await user.tab();

    expect(await screen.findByText('settings:cors.validation.invalidRegex')).toBeInTheDocument();
  });

  it('warns about an unanchored pattern without reporting it as an error', async () => {
    render(<CorsOriginsEditor rows={[regex('acme\\.io')]} onRowsChange={vi.fn()} />);

    await user.click(screen.getByPlaceholderText(REGEX_PLACEHOLDER));
    await user.tab();

    expect(await screen.findByText('settings:cors.validation.unanchoredRegex')).toBeInTheDocument();
    expect(screen.getByPlaceholderText(REGEX_PLACEHOLDER)).toHaveAttribute('aria-invalid', 'false');
  });
});
