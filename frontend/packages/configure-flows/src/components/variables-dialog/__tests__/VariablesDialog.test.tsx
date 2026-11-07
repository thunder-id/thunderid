// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {I18nProvider, ThemeProvider} from '@thunderid/react';
import type {FlowVariableDefinition} from '@thunderid/react';
import type {ReactElement} from 'react';
import {describe, it, expect, vi} from 'vitest';
import type {VariablesGroup} from '../../../context/FlowVariablesContext';
import VariablesDialog from '../VariablesDialog';

vi.mock('@thunderid/configure-connections', () => ({
  useIdentityProviders: () => ({data: [{id: 'idp-1', name: 'Google login', type: 'GOOGLE'}]}),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, defaultValue?: string, options?: Record<string, string>) =>
      Object.entries(options ?? {}).reduce(
        (text, [name, value]) => text.replace(`{{${name}}}`, value),
        defaultValue ?? _key,
      ),
  }),
}));

const variables: FlowVariableDefinition[] = [
  {
    category: 'IDENTITY_PROVIDER',
    defaultValue: 'idp-1',
    identifier: 'connection1',
    label: 'connection1',
    type: 'CONNECTION_SELECT',
  },
];

const groups: VariablesGroup[] = [{nodeId: 'vars', variables}];

const renderDialog = (ui: ReactElement): ReturnType<typeof render> =>
  render(
    <I18nProvider>
      <ThemeProvider>{ui}</ThemeProvider>
    </I18nProvider>,
  );

describe('VariablesDialog', () => {
  it('should add a variable to a node and apply it', () => {
    const onApply = vi.fn();

    renderDialog(<VariablesDialog open groups={groups} onApply={onApply} onClose={vi.fn()} />);
    fireEvent.click(screen.getByTestId('variables-add-vars'));
    fireEvent.click(screen.getByTestId('variables-apply'));

    expect(onApply).toHaveBeenCalledTimes(1);
    expect(onApply).toHaveBeenCalledWith([
      expect.objectContaining({nodeId: 'vars', variables: expect.arrayContaining([variables[0]]) as unknown}),
    ]);
    expect((onApply.mock.calls[0] as [VariablesGroup[]])[0][0].variables).toHaveLength(2);
  });

  it('should label the type and connection type fields', () => {
    renderDialog(<VariablesDialog open groups={groups} onApply={vi.fn()} onClose={vi.fn()} />);

    expect(screen.getByText('Type')).toBeInTheDocument();
    expect(screen.getByText('Allowed connection types')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('All')).toBeInTheDocument();
  });
});
