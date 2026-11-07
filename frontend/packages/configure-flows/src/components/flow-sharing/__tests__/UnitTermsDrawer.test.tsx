// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {FlowVariableDefinition} from '@thunderid/react';
import {describe, it, expect, vi} from 'vitest';
import UnitTermsDrawer from '../UnitTermsDrawer';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue ?? key,
  }),
}));

vi.mock('../VariableSharingRules', () => ({
  default: ({onChange, rules}: {onChange: (rules: unknown) => void; rules: unknown}) => (
    <div data-testid="rules" data-rules={JSON.stringify(rules)}>
      <button type="button" onClick={() => onChange({'variables.a': {editable: true}})}>
        change-rules
      </button>
    </div>
  ),
}));

const variables: FlowVariableDefinition[] = [{defaultValue: '', identifier: 'a', type: 'TEXT_INPUT'}];

describe('UnitTermsDrawer', () => {
  const renderDrawer = (onApply = vi.fn(), onClose = vi.fn()) =>
    render(
      <UnitTermsDrawer
        open
        title="Terms for Acme Retail"
        subtitle="These terms belong to Acme Retail only."
        applyLabel="Apply"
        variables={variables}
        rules={{'variables.a': {editable: false, value: ['x']}}}
        onApply={onApply}
        onClose={onClose}
      />,
    );

  it('opens on the terms it is given', () => {
    renderDrawer();

    expect(screen.getByText('Terms for Acme Retail')).toBeInTheDocument();
    expect(screen.getByTestId('rules')).toHaveAttribute(
      'data-rules',
      JSON.stringify({'variables.a': {editable: false, value: ['x']}}),
    );
  });

  it('writes the edited terms only when applied', async () => {
    const user = userEvent.setup();
    const onApply = vi.fn();

    renderDrawer(onApply);
    await user.click(screen.getByText('change-rules'));

    expect(onApply).not.toHaveBeenCalled();

    await user.click(screen.getByTestId('unit-terms-apply'));

    expect(onApply).toHaveBeenCalledWith({'variables.a': {editable: true}});
  });

  it('closes without applying', async () => {
    const user = userEvent.setup();
    const onApply = vi.fn();
    const onClose = vi.fn();

    renderDrawer(onApply, onClose);
    await user.click(screen.getByText('Cancel'));

    expect(onClose).toHaveBeenCalled();
    expect(onApply).not.toHaveBeenCalled();
  });
});
