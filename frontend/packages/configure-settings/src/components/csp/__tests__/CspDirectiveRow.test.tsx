// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import CspDirectiveRow from '../CspDirectiveRow';

const sourcesField = (directive = 'img-src'): HTMLElement =>
  screen.getByRole('combobox', {name: `Sources for ${directive}`});

describe('CspDirectiveRow', () => {
  it('shows each source as a chip', () => {
    renderWithProviders(<CspDirectiveRow directive="img-src" sources={["'self'", 'data:']} />);

    expect(screen.getByRole('button', {name: "'self'"})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'data:'})).toBeInTheDocument();
  });

  it('adds a source on Enter', async () => {
    const user = userEvent.setup();
    const onSourcesChange = vi.fn();
    renderWithProviders(<CspDirectiveRow directive="img-src" sources={["'self'"]} onSourcesChange={onSourcesChange} />);

    await user.type(sourcesField(), 'data:{Enter}');

    expect(onSourcesChange).toHaveBeenCalledWith(["'self'", 'data:']);
  });

  it('adds a source on a space, and splits a pasted list into sources', async () => {
    const user = userEvent.setup();
    const onSourcesChange = vi.fn();
    renderWithProviders(<CspDirectiveRow directive="img-src" sources={[]} onSourcesChange={onSourcesChange} />);

    await user.type(sourcesField(), 'data: ');
    expect(onSourcesChange).toHaveBeenLastCalledWith(['data:']);

    await user.click(sourcesField());
    await user.paste("'self'  https://cdn.example.com");
    expect(onSourcesChange).toHaveBeenLastCalledWith(["'self'"]);
    expect(sourcesField()).toHaveValue('https://cdn.example.com');
  });

  it('adds the typed source when the field loses focus, ignoring duplicates', async () => {
    const user = userEvent.setup();
    const onSourcesChange = vi.fn();
    renderWithProviders(<CspDirectiveRow directive="img-src" sources={['data:']} onSourcesChange={onSourcesChange} />);

    await user.type(sourcesField(), 'data:');
    await user.tab();
    expect(onSourcesChange).not.toHaveBeenCalled();

    await user.type(sourcesField(), 'blob:');
    await user.tab();
    expect(onSourcesChange).toHaveBeenCalledWith(['data:', 'blob:']);
  });

  it('removes a source when its chip is deleted', async () => {
    const user = userEvent.setup();
    const onSourcesChange = vi.fn();
    renderWithProviders(
      <CspDirectiveRow directive="img-src" sources={["'self'", 'data:']} onSourcesChange={onSourcesChange} />,
    );

    await user.click(screen.getByRole('button', {name: 'data:'}));
    await user.keyboard('{Backspace}');

    expect(onSourcesChange).toHaveBeenCalledWith(["'self'"]);
  });

  it('lets a new directive be changed, and offers its remove action', async () => {
    const user = userEvent.setup();
    const onDirectiveChange = vi.fn();
    const onAction = vi.fn();
    renderWithProviders(
      <CspDirectiveRow
        directive="media-src"
        sources={[]}
        directiveOptions={['media-src', 'font-src']}
        action="remove"
        onDirectiveChange={onDirectiveChange}
        onAction={onAction}
      />,
    );

    await user.click(screen.getByRole('combobox', {name: 'Directive'}));
    await user.click(screen.getByRole('option', {name: 'font-src'}));
    expect(onDirectiveChange).toHaveBeenCalledWith('font-src');

    await user.click(screen.getByRole('button', {name: 'Remove directive'}));
    expect(onAction).toHaveBeenCalled();
  });

  it('fixes the directive of an existing row and offers a reset when it is overridden', async () => {
    const user = userEvent.setup();
    const onAction = vi.fn();
    renderWithProviders(
      <CspDirectiveRow directive="img-src" sources={['https:']} action="reset" onAction={onAction} />,
    );

    expect(screen.getByRole('combobox', {name: 'Directive'})).toHaveAttribute('aria-disabled', 'true');
    await user.click(screen.getByRole('button', {name: 'Reset to default'}));
    expect(onAction).toHaveBeenCalled();
  });

  it('offers no action for a directive at its default value', () => {
    renderWithProviders(<CspDirectiveRow directive="img-src" sources={["'self'"]} />);
    expect(screen.queryByRole('button', {name: 'Reset to default'})).toBeNull();
    expect(screen.queryByRole('button', {name: 'Remove directive'})).toBeNull();
  });

  it('shows an error message', () => {
    renderWithProviders(<CspDirectiveRow directive="img-src" sources={[]} error="Bad source" />);
    expect(screen.getByText('Bad source')).toBeInTheDocument();
  });
});
