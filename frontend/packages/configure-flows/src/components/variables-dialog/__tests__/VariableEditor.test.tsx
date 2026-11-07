// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import type {FlowVariableDefinition} from '@thunderid/react';
import {describe, expect, it, vi} from 'vitest';
import VariableEditor from '../VariableEditor';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({t: (_key: string, defaultValue?: string) => defaultValue ?? _key}),
}));

vi.mock('../variableDefinitions', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../variableDefinitions')>()),
  getConnectionTypes: () => [{label: 'Google Login', value: 'google'}],
}));

const renderEditor = (definition: FlowVariableDefinition) =>
  render(
    <VariableEditor
      connectionOptions={[{label: 'Google login', value: 'c-1'}]}
      definitions={[definition]}
      index={0}
      onChange={vi.fn()}
      onRename={vi.fn()}
    />,
  );

describe('VariableEditor', () => {
  it('names the type select by its label', () => {
    renderEditor({defaultValue: '', identifier: 'a', type: 'TEXT_INPUT'});

    expect(screen.getByRole('combobox', {name: 'Type'})).toBeInTheDocument();
  });

  it('names the default select of a choice variable', () => {
    renderEditor({defaultValue: 'x', identifier: 'a', options: ['x', 'y'], type: 'SELECT'});

    expect(screen.getByRole('combobox', {name: 'Default value'})).toBeInTheDocument();
  });

  it('names the default select of a connection variable', () => {
    renderEditor({category: 'IDENTITY_PROVIDER', defaultValue: '', identifier: 'a', type: 'CONNECTION_SELECT'});

    expect(screen.getByRole('combobox', {name: 'Default value'})).toBeInTheDocument();
  });
});
