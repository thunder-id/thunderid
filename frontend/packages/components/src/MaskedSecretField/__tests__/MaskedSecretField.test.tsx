// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import MaskedSecretField from '../MaskedSecretField';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

describe('MaskedSecretField', () => {
  it('shows the editable hint when replacing or creating a secret', () => {
    render(
      <MaskedSecretField
        id="client-secret"
        label="Client secret"
        value=""
        onChange={vi.fn()}
        hasStoredSecret={false}
        replacing={false}
        onReplacingChange={vi.fn()}
        hint="connections:form.fields.clientSecret.hint"
      />,
    );

    expect(screen.getByText('connections:form.fields.clientSecret.hint')).toBeInTheDocument();
  });

  it('keeps the stored-secret helper when the secret is not being replaced', () => {
    render(
      <MaskedSecretField
        id="client-secret"
        label="Client secret"
        value=""
        onChange={vi.fn()}
        hasStoredSecret
        replacing={false}
        onReplacingChange={vi.fn()}
        hint="connections:form.fields.clientSecret.hint"
      />,
    );

    expect(screen.getByText('Leave unchanged to keep the stored secret.')).toBeInTheDocument();
    expect(screen.queryByText('connections:form.fields.clientSecret.hint')).not.toBeInTheDocument();
  });

  it('cancels a replacement, restoring the value it had before Update', () => {
    const onChange = vi.fn();
    const onReplacingChange = vi.fn();
    const {rerender} = render(
      <MaskedSecretField
        id="client-secret"
        label="Client secret"
        value=""
        onChange={onChange}
        hasStoredSecret
        replacing={false}
        onReplacingChange={onReplacingChange}
      />,
    );

    fireEvent.click(screen.getByTestId('client-secret-replace'));
    expect(onReplacingChange).toHaveBeenLastCalledWith(true);
    rerender(
      <MaskedSecretField
        id="client-secret"
        label="Client secret"
        value="typed"
        onChange={onChange}
        hasStoredSecret
        replacing
        onReplacingChange={onReplacingChange}
      />,
    );
    fireEvent.click(screen.getByTestId('client-secret-cancel'));

    expect(onChange).toHaveBeenLastCalledWith('');
    expect(onReplacingChange).toHaveBeenLastCalledWith(false);
  });

  it('offers no Cancel when there is no stored secret to return to', () => {
    render(
      <MaskedSecretField
        id="client-secret"
        label="Client secret"
        value=""
        onChange={vi.fn()}
        hasStoredSecret={false}
        replacing={false}
        onReplacingChange={vi.fn()}
      />,
    );

    expect(screen.queryByTestId('client-secret-cancel')).not.toBeInTheDocument();
  });
});
