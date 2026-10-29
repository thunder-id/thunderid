// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, within} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import executors from '../../../../../data/executors.json';
import type {Resource} from '../../../../../models/resources';
import {IDENTITY_GOVERNANCE_MODES} from '../constants';
import IdentityGovernanceProperties from '../IdentityGovernanceProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue ?? key,
  }),
}));

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useParams: () => ({flowId: 'flow-1'}),
}));

const mockUseGetFlowById = vi.fn<(flowId?: string) => unknown>();
vi.mock('../../../../../api/useGetFlowById', () => ({
  default: (flowId?: string): unknown => mockUseGetFlowById(flowId),
}));

function inFlowOfType(flowType?: string): void {
  mockUseGetFlowById.mockReturnValue({data: flowType ? {flowType} : undefined});
}

/**
 * Opens the operation selector and returns the operations it offers.
 */
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
        executor: {name: 'IdentityGovernanceExecutor', ...(mode ? {mode} : {})},
        type: 'EXECUTOR',
      },
      display: {label: 'Suspend Identity'},
    },
    id: 'access_governance',
  } as unknown as Resource;
}

// Every Identity Governance entry in the executor catalog, with the fields these tests assert on.
function governanceCatalogEntries(): {mode?: string; image?: string; showOnResourcePanel?: boolean}[] {
  return (
    executors as {
      display?: {image?: string; showOnResourcePanel?: boolean};
      data?: {action?: {executor?: {name?: string; mode?: string}}};
    }[]
  )
    .filter((entry) => entry.data?.action?.executor?.name === 'IdentityGovernanceExecutor')
    .map((entry) => ({
      image: entry.display?.image,
      mode: entry.data?.action?.executor?.mode,
      showOnResourcePanel: entry.display?.showOnResourcePanel,
    }));
}

describe('IdentityGovernanceProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    inFlowOfType('ADMINISTRATION');
  });

  it('should read the type of the flow being edited', () => {
    render(<IdentityGovernanceProperties resource={makeResource('unlock')} onChange={mockOnChange} />);

    expect(mockUseGetFlowById).toHaveBeenCalledWith('flow-1');
  });

  it('should offer every operation in an administration flow', () => {
    render(<IdentityGovernanceProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(offeredOperations()).toEqual(['Suspend Identity', 'Unsuspend Identity', 'Unlock Identity']);
  });

  // Suspend and unsuspend are administrative; the server refuses them in any other flow.
  it('should offer only Unlock in a recovery flow', () => {
    inFlowOfType('RECOVERY');
    render(<IdentityGovernanceProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(offeredOperations()).toEqual(['Unlock Identity']);
  });

  it('should keep an operation already on the node listed in a recovery flow, so it can be changed', () => {
    inFlowOfType('RECOVERY');
    render(<IdentityGovernanceProperties resource={makeResource('suspend')} onChange={mockOnChange} />);

    expect(offeredOperations()).toEqual(['Suspend Identity', 'Unlock Identity']);
  });

  it('should offer every operation until the flow loads', () => {
    inFlowOfType(undefined);
    render(<IdentityGovernanceProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(offeredOperations()).toEqual(['Suspend Identity', 'Unsuspend Identity', 'Unlock Identity']);
  });

  it('should not say only Unlock runs in a recovery flow, since the list already shows it', () => {
    render(<IdentityGovernanceProperties resource={makeResource('unlock')} onChange={mockOnChange} />);

    expect(screen.getByText(/Suspend and unlock act on different holds/)).not.toHaveTextContent('recovery flow');
  });

  it('should render the operation selector', () => {
    render(<IdentityGovernanceProperties resource={makeResource('suspend')} onChange={mockOnChange} />);

    expect(screen.getByText('Operation')).toBeInTheDocument();
  });

  it('should show the operation configured on the node', () => {
    render(<IdentityGovernanceProperties resource={makeResource('unsuspend')} onChange={mockOnChange} />);

    expect(screen.getByText('Unsuspend Identity')).toBeInTheDocument();
  });

  // The executor declares no default mode, so a node without one is unsaved work rather than a
  // suspend node.
  it('should render a placeholder when the node carries no operation', () => {
    render(<IdentityGovernanceProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(screen.getByText('Select an operation')).toBeInTheDocument();
  });

  // A mode the backend does not support would save a flow that fails validation.
  it('should only offer operations the executor supports', () => {
    expect(IDENTITY_GOVERNANCE_MODES.map((mode) => mode.value)).toEqual(['suspend', 'unsuspend', 'unlock']);
  });

  // The canvas takes a node's label and icon from the catalog entry for its mode.
  it('should have a catalog entry with the same icon backing every operation', () => {
    expect(governanceCatalogEntries().map(({image, mode}) => ({image, mode}))).toEqual(
      IDENTITY_GOVERNANCE_MODES.map((mode) => ({image: mode.image, mode: mode.value})),
    );
  });

  // Suspend and unsuspend share one palette entry; the dropdown switches between them.
  it('should offer suspend and unlock on the palette, and reach unsuspend through the dropdown', () => {
    const onPalette = governanceCatalogEntries()
      .filter((entry) => entry.showOnResourcePanel !== false)
      .map((entry) => entry.mode);

    expect(onPalette).toEqual(['suspend', 'unlock']);
  });
});
