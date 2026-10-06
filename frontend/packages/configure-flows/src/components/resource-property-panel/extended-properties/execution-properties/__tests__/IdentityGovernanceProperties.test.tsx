// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
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

// governanceCatalogEntries is every Identity Governance entry in the executor catalog, with the two
// fields these tests assert on.
function governanceCatalogEntries(): {mode?: string; showOnResourcePanel?: boolean}[] {
  return (
    executors as {
      display?: {showOnResourcePanel?: boolean};
      data?: {action?: {executor?: {name?: string; mode?: string}}};
    }[]
  )
    .filter((entry) => entry.data?.action?.executor?.name === 'IdentityGovernanceExecutor')
    .map((entry) => ({
      mode: entry.data?.action?.executor?.mode,
      showOnResourcePanel: entry.display?.showOnResourcePanel,
    }));
}

describe('IdentityGovernanceProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
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

  // Offering a mode the backend does not declare in SupportedModes would let the builder save a flow
  // that fails validation, so the option list must not drift ahead of the executor.
  it('should only offer operations the executor supports', () => {
    expect(IDENTITY_GOVERNANCE_MODES.map((mode) => mode.value)).toEqual(['suspend', 'unsuspend', 'unlock']);
  });

  // The canvas reads display metadata resolved from the executor catalog by name and mode. A mode
  // missing there leaves a saved node with no label or icon — which the shipped unsuspend flow would
  // hit, since its nodes carry no display of their own.
  it('should have a catalog entry backing every operation', () => {
    expect(governanceCatalogEntries().map((entry) => entry.mode)).toEqual(
      IDENTITY_GOVERNANCE_MODES.map((mode) => mode.value),
    );
  });

  // Suspend and unsuspend are two directions of one hold, so the palette offers the pair once and
  // the operation dropdown switches between them. Unsuspend keeps its catalog entry for the test
  // above; it is hidden from the panel rather than absent from the catalog.
  it('should offer suspend and unlock on the palette, and reach unsuspend through the dropdown', () => {
    const onPalette = governanceCatalogEntries()
      .filter((entry) => entry.showOnResourcePanel !== false)
      .map((entry) => entry.mode);

    expect(onPalette).toEqual(['suspend', 'unlock']);
  });
});
