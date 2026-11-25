// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, within} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import FlowTypeContext from '../../../../../context/FlowTypeContext';
import executors from '../../../../../data/executors.json';
import type {FlowType} from '../../../../../models/flows';
import type {Resource} from '../../../../../models/resources';
import CredentialSetterProperties from '../CredentialSetterProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue ?? key,
  }),
}));

let flowType: FlowType | undefined;

function inFlowOfType(type?: string): void {
  flowType = type as FlowType | undefined;
}

const mockOnChange = vi.fn();

function renderProperties(mode?: string): void {
  render(
    <FlowTypeContext.Provider value={flowType}>
      <CredentialSetterProperties resource={makeResource(mode)} onChange={mockOnChange} />
    </FlowTypeContext.Provider>,
  );
}

function offeredOperations(): string[] {
  fireEvent.mouseDown(screen.getByRole('combobox'));

  return within(screen.getByRole('listbox'))
    .getAllByRole('option')
    .filter((option) => option.getAttribute('aria-disabled') !== 'true')
    .map((option) => option.textContent ?? '');
}

function makeResource(mode?: string): Resource {
  return {
    data: {
      action: {
        executor: {name: 'CredentialSetter', ...(mode ? {mode} : {})},
        type: 'EXECUTOR',
      },
      display: {label: 'Set Credentials'},
    },
    id: 'set_credential',
  } as unknown as Resource;
}

function credentialSetterCatalogEntries(): {mode?: string; label?: string; showOnResourcePanel?: boolean}[] {
  return (
    executors as {
      display?: {label?: string; showOnResourcePanel?: boolean};
      data?: {action?: {executor?: {name?: string; mode?: string}}};
    }[]
  )
    .filter((entry) => entry.data?.action?.executor?.name === 'CredentialSetter')
    .map((entry) => ({
      label: entry.display?.label,
      mode: entry.data?.action?.executor?.mode,
      showOnResourcePanel: entry.display?.showOnResourcePanel,
    }));
}

describe('CredentialSetterProperties', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    inFlowOfType('ADMIN_INITIATED');
  });

  it('should offer every operation in an administrator-initiated flow', () => {
    renderProperties();

    expect(offeredOperations()).toEqual(['Set Credentials', 'Validate Credential Reset', 'Remove Credential']);
  });

  it('should show a node without a mode as setting the credential', () => {
    renderProperties();

    expect(screen.getByRole('combobox')).toHaveTextContent('Set Credentials');
  });

  it('should switch the node to the chosen operation and label it after it', () => {
    renderProperties();

    fireEvent.mouseDown(screen.getByRole('combobox'));
    fireEvent.click(within(screen.getByRole('listbox')).getByText('Remove Credential'));

    expect(mockOnChange).toHaveBeenCalledWith(
      'data',
      expect.objectContaining({
        action: expect.objectContaining({executor: {name: 'CredentialSetter', mode: 'invalidate'}}) as unknown,
        display: expect.objectContaining({label: 'Remove Credential'}) as unknown,
      }),
      expect.anything(),
    );
  });

  it('should leave a node switched back to setting the credential without a mode, as the template has it', () => {
    renderProperties('invalidate');

    fireEvent.mouseDown(screen.getByRole('combobox'));
    fireEvent.click(within(screen.getByRole('listbox')).getByText('Set Credentials'));

    const [, data] = mockOnChange.mock.calls[0] as [string, {action: {executor: Record<string, unknown>}}];
    expect(data.action.executor).toEqual({name: 'CredentialSetter'});
  });

  // Validate and invalidate run only in an administrator-initiated flow; the server refuses them elsewhere.
  it('should have nothing to configure in another flow', () => {
    inFlowOfType('RECOVERY');
    renderProperties();

    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    expect(screen.getByText('flows:core.executions.noConfig.description')).toBeInTheDocument();
  });

  it('should keep a mode already on the node listed in another flow, so it can be changed back', () => {
    inFlowOfType('RECOVERY');
    renderProperties('invalidate');

    expect(offeredOperations()).toEqual(['Set Credentials', 'Remove Credential']);
  });

  it('should offer every operation outside the flow builder', () => {
    inFlowOfType(undefined);
    renderProperties();

    expect(offeredOperations()).toEqual(['Set Credentials', 'Validate Credential Reset', 'Remove Credential']);
  });

  // The canvas takes a node's label from the catalog entry for its mode; the palette keeps one entry.
  it('should back every operation with a catalog entry, with only Set on the palette', () => {
    expect(credentialSetterCatalogEntries()).toEqual([
      {label: 'Set Credentials', mode: undefined, showOnResourcePanel: true},
      {label: 'Validate Credential Reset', mode: 'validate', showOnResourcePanel: false},
      {label: 'Remove Credential', mode: 'invalidate', showOnResourcePanel: false},
    ]);
  });
});
