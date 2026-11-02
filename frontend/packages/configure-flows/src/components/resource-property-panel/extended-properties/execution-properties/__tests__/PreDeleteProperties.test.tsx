// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import {REVOCATION_MODES} from '../constants';
import PreDeleteProperties from '../PreDeleteProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue ?? key,
  }),
}));

function makeResource(mode?: string): Resource {
  return {
    data: {
      action: {
        executor: {name: 'PreDeleteExecutor', ...(mode ? {mode} : {})},
        type: 'EXECUTOR',
      },
      display: {label: 'Validate and Plan Full Revocation'},
    },
    id: 'administrative-pre',
  } as unknown as Resource;
}

describe('PreDeleteProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render the revocation mode selector', () => {
    render(<PreDeleteProperties resource={makeResource('revoke_all')} onChange={mockOnChange} />);

    expect(screen.getByText('Revocation mode')).toBeInTheDocument();
  });

  it('should show the mode configured on the node', () => {
    render(<PreDeleteProperties resource={makeResource('revoke_all')} onChange={mockOnChange} />);

    expect(screen.getByText('Validate and Plan Full Revocation')).toBeInTheDocument();
  });

  it('should render a placeholder when the node carries no mode', () => {
    render(<PreDeleteProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(screen.getByText('Select a revocation mode')).toBeInTheDocument();
  });

  // Offering a mode the backend does not declare in SupportedModes would let the builder save a flow
  // that fails validation, so the option list must not drift ahead of the executor.
  it('should only offer modes the executor supports', () => {
    expect(REVOCATION_MODES.map((mode) => mode.value)).toEqual(['revoke_all']);
  });

  it('should default the display label when the node carries no display block', async () => {
    const user = userEvent.setup();
    const resource = {
      data: {
        action: {executor: {name: 'PreDeleteExecutor'}, type: 'EXECUTOR'},
      },
      id: 'administrative-pre',
    } as unknown as Resource;

    render(<PreDeleteProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'Validate and Plan Full Revocation'}));

    expect(mockOnChange).toHaveBeenCalledWith(
      'data',
      expect.objectContaining({
        display: {label: 'Validate and Plan Full Revocation'},
      }),
      resource,
    );
  });
});
