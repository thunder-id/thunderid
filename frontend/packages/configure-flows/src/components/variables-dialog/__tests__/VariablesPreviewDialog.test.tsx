// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, within} from '@testing-library/react';
import {I18nProvider, ThemeProvider} from '@thunderid/react';
import type {FlowVariableDefinition} from '@thunderid/react';
import type {ReactElement} from 'react';
import {describe, it, expect, vi} from 'vitest';
import VariablesPreviewDialog from '../VariablesPreviewDialog';

vi.mock('@thunderid/configure-connections', () => ({
  ConnectionInstanceCategories: {IDENTITY_PROVIDER: 'identity-provider'},
  useConnections: () => ({
    data: {
      connections: [
        {id: 'idp-1', name: 'Google login', type: 'google', categories: ['identity-provider']},
        {id: 'idp-2', name: 'GitHub login', type: 'github', categories: ['identity-provider']},
      ],
    },
  }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (_key: string, defaultValue?: string) => defaultValue ?? _key}),
}));

const variables: FlowVariableDefinition[] = [
  {
    category: 'IDENTITY_PROVIDER',
    defaultValue: 'idp-1',
    description: 'select your connection',
    identifier: 'connection1',
    label: 'connection1',
    type: 'CONNECTION_SELECT',
  },
];

const renderDialog = (ui: ReactElement): ReturnType<typeof render> =>
  render(
    <I18nProvider>
      <ThemeProvider>{ui}</ThemeProvider>
    </I18nProvider>,
  );

describe('VariablesPreviewDialog', () => {
  it('should show the variables as the delegated admin sees them', () => {
    renderDialog(<VariablesPreviewDialog open variables={variables} onClose={vi.fn()} />);

    const dialog = within(screen.getByTestId('variables-preview-dialog'));

    expect(dialog.getByText('How a delegated admin will see it')).toBeInTheDocument();
    expect(dialog.getByText('connection1')).toBeInTheDocument();
    expect(dialog.getByText('select your connection')).toBeInTheDocument();
  });

  it('should note that a delegated admin sees only the connections of their own unit', () => {
    renderDialog(<VariablesPreviewDialog open variables={variables} onClose={vi.fn()} />);

    expect(screen.getByTestId('variables-preview-connections-note')).toBeInTheDocument();
  });

  it('should leave the connections note out when no variable takes a connection', () => {
    renderDialog(
      <VariablesPreviewDialog
        open
        variables={[{identifier: 'supportEmail', type: 'EMAIL_INPUT'} as FlowVariableDefinition]}
        onClose={vi.fn()}
      />,
    );

    expect(screen.queryByTestId('variables-preview-connections-note')).not.toBeInTheDocument();
  });

  it('should say there is nothing to preview when no variable is declared', () => {
    renderDialog(<VariablesPreviewDialog open variables={[]} onClose={vi.fn()} />);

    expect(screen.getByText('Add a variable to see it here.')).toBeInTheDocument();
  });

  it('should call onClose from the Close button', () => {
    const onClose = vi.fn();

    renderDialog(<VariablesPreviewDialog open variables={variables} onClose={onClose} />);
    fireEvent.click(screen.getByRole('button', {name: 'Close'}));

    expect(onClose).toHaveBeenCalled();
  });

  it('should render nothing while closed', () => {
    renderDialog(<VariablesPreviewDialog open={false} variables={variables} onClose={vi.fn()} />);

    expect(screen.queryByTestId('variables-preview-dialog')).not.toBeInTheDocument();
  });
});
