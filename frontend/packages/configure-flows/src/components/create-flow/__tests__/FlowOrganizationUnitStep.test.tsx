// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import FlowOrganizationUnitStep from '../FlowOrganizationUnitStep';

vi.mock('@thunderid/configure-organization-units', () => ({
  OrganizationUnitPickerScreen: ({
    title,
    subtitle,
    value,
    backLabel,
    continueLabel,
    onChange,
    onBack,
    onContinue,
  }: {
    title: string;
    subtitle: string;
    value: string;
    backLabel: string;
    continueLabel: string;
    onChange: (ouId: string) => void;
    onBack: () => void;
    onContinue: () => void;
  }) => (
    <div data-testid="ou-picker" data-value={value}>
      <h1>{title}</h1>
      <p>{subtitle}</p>
      <button type="button" onClick={() => onChange('ou-b')}>
        pick b
      </button>
      <button type="button" onClick={onBack}>
        {backLabel}
      </button>
      <button type="button" onClick={onContinue}>
        {continueLabel}
      </button>
    </div>
  ),
}));

describe('FlowOrganizationUnitStep', () => {
  it('shows a loading indicator instead of the picker while the organization units load', () => {
    render(<FlowOrganizationUnitStep isLoading value="" onChange={vi.fn()} onBack={vi.fn()} onContinue={vi.fn()} />);

    expect(screen.getByRole('progressbar', {name: 'Loading content'})).toBeInTheDocument();
    expect(screen.queryByTestId('ou-picker')).not.toBeInTheDocument();
  });

  it('renders the picker with the flow wording and the selected value', () => {
    render(
      <FlowOrganizationUnitStep
        isLoading={false}
        value="ou-a"
        onChange={vi.fn()}
        onBack={vi.fn()}
        onContinue={vi.fn()}
      />,
    );

    expect(screen.getByRole('heading', {name: 'Where should this flow belong?'})).toBeInTheDocument();
    expect(screen.getByText(/will own this flow/)).toBeInTheDocument();
    expect(screen.getByTestId('ou-picker')).toHaveAttribute('data-value', 'ou-a');
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
  });

  it('reports a picked organization unit through onChange', () => {
    const onChange = vi.fn();
    render(
      <FlowOrganizationUnitStep isLoading={false} value="" onChange={onChange} onBack={vi.fn()} onContinue={vi.fn()} />,
    );

    fireEvent.click(screen.getByRole('button', {name: 'pick b'}));

    expect(onChange).toHaveBeenCalledWith('ou-b');
  });

  it('calls onBack and onContinue from the picker actions', () => {
    const onBack = vi.fn();
    const onContinue = vi.fn();
    render(
      <FlowOrganizationUnitStep
        isLoading={false}
        value="ou-a"
        onChange={vi.fn()}
        onBack={onBack}
        onContinue={onContinue}
      />,
    );

    fireEvent.click(screen.getByRole('button', {name: 'Back'}));
    fireEvent.click(screen.getByRole('button', {name: 'Continue'}));

    expect(onBack).toHaveBeenCalledTimes(1);
    expect(onContinue).toHaveBeenCalledTimes(1);
  });
});
