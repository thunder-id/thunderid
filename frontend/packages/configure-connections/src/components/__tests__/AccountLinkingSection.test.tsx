// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AccountLinkingSection from '../AccountLinkingSection';

const EXTERNAL_ATTRIBUTE = 'External Attribute';

// The SettingsCard is mocked to a labelled <section> so the card can be located and scoped by title.
vi.mock('@thunderid/components', () => ({
  SettingsCard: ({title, children}: {title: string; children: React.ReactNode}) => (
    <section aria-label={title}>{children}</section>
  ),
}));

describe('AccountLinkingSection', () => {
  const onChange = vi.fn();
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders a starter attribute input without needing to add one first', () => {
    render(<AccountLinkingSection onChange={onChange} />);
    expect(screen.getAllByLabelText(EXTERNAL_ATTRIBUTE)).toHaveLength(1);
  });

  it('emits undefined on an empty mount', () => {
    render(<AccountLinkingSection onChange={onChange} />);
    expect(onChange).toHaveBeenLastCalledWith(undefined);
  });

  it('adds account-linking attributes', () => {
    render(<AccountLinkingSection onChange={onChange} />);
    fireEvent.click(screen.getByTestId('attribute-mapping-link-add'));
    const linkInputs = screen.getAllByLabelText(EXTERNAL_ATTRIBUTE);
    fireEvent.change(linkInputs[linkInputs.length - 1], {target: {value: 'email'}});
    expect(onChange).toHaveBeenLastCalledWith({attributes: ['email']});
  });

  it('hydrates existing attributes on edit prefill', () => {
    render(<AccountLinkingSection initialConfig={{attributes: ['email', 'username']}} onChange={onChange} />);
    expect(screen.getAllByLabelText(EXTERNAL_ATTRIBUTE)).toHaveLength(2);
    expect(onChange).toHaveBeenLastCalledWith({attributes: ['email', 'username']});
  });

  it('hides delete for the default empty row until it has content', () => {
    render(<AccountLinkingSection onChange={onChange} />);
    expect(screen.queryByRole('button', {name: /remove account linking attribute/i})).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(EXTERNAL_ATTRIBUTE), {target: {value: 'email'}});
    expect(screen.getByRole('button', {name: /remove account linking attribute/i})).toBeInTheDocument();
  });

  it('shows delete for every row once an extra blank row is added', () => {
    render(<AccountLinkingSection onChange={onChange} />);
    // "Add attribute" is disabled while the last row is empty, so fill the first row before adding.
    fireEvent.change(screen.getByLabelText(EXTERNAL_ATTRIBUTE), {target: {value: 'email'}});
    fireEvent.click(screen.getByTestId('attribute-mapping-link-add'));
    expect(screen.getAllByRole('button', {name: /remove account linking attribute/i})).toHaveLength(2);
  });

  it('disables "Add Attribute" until the last row has content', () => {
    render(<AccountLinkingSection onChange={onChange} />);
    expect(screen.getByTestId('attribute-mapping-link-add')).toBeDisabled();

    fireEvent.change(screen.getByLabelText(EXTERNAL_ATTRIBUTE), {target: {value: 'email'}});
    expect(screen.getByTestId('attribute-mapping-link-add')).not.toBeDisabled();
  });
});
