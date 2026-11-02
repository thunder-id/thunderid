// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationUnit} from '@thunderid/configure-organization-units';
import {screen, fireEvent, renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import type {AccessSwitch} from '../../models/organization-access';
import {EMPTY_ORGANIZATION_ACCESS_DRAFT} from '../../utils/sharingPolicyToDraft';
import SwitchAccessConfirmDialog from '../SwitchAccessConfirmDialog';

const unitsById = new Map<string, OrganizationUnit>([
  ['sandbox', {id: 'sandbox', handle: 'sandbox', name: 'Partner Sandbox'}],
  ['retail', {id: 'retail', handle: 'retail', name: 'Acme Retail'}],
]);

const accessSwitch = (overrides: Partial<AccessSwitch>): AccessSwitch => ({
  field: 'childUnitsAccess',
  to: 'selected',
  next: EMPTY_ORGANIZATION_ACCESS_DRAFT,
  cleared: [],
  losesEveryUnit: false,
  losesEveryTree: false,
  ...overrides,
});

const renderDialog = (
  value: AccessSwitch | null,
  extra: {hasUnitSettings?: boolean; wordingNamespace?: string} = {},
) => {
  const onCancel = vi.fn();
  const onConfirm = vi.fn();
  renderWithProviders(
    <SwitchAccessConfirmDialog
      accessSwitch={value}
      fromLabel="Every organization unit"
      toLabel="Selected child units"
      ownerName="Acme Corp"
      resourceName="Storefront"
      unitsById={unitsById}
      onCancel={onCancel}
      onConfirm={onConfirm}
      {...extra}
    />,
  );
  return {onCancel, onConfirm};
};

describe('SwitchAccessConfirmDialog', () => {
  it('renders nothing without a pending switch', () => {
    renderDialog(null);

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('lists what the switch clears, with unit names', () => {
    renderDialog(
      accessSwitch({
        cleared: [
          {kind: 'everyUnit', ouIds: ['sandbox']},
          {kind: 'childUnits', ouIds: ['retail']},
        ],
        losesEveryUnit: true,
      }),
    );

    expect(screen.getByText('Switch to Selected child units?')).toBeInTheDocument();
    expect(
      screen.getByText('The "Every organization unit" policy, with its exceptions: Partner Sandbox'),
    ).toBeInTheDocument();
    expect(screen.getByText('Granted child units: Acme Retail')).toBeInTheDocument();
    expect(screen.getByText(/can't be chosen again here/)).toBeInTheDocument();
  });

  it('words leaving every organization unit as deleting that policy and changing the scope', () => {
    renderDialog(
      accessSwitch({field: 'scope', to: 'chosen', cleared: [{kind: 'everyUnit', ouIds: []}], losesEveryUnit: true}),
    );

    expect(screen.getByText('Change the scope?')).toBeInTheDocument();
    expect(screen.getByText(/Access then starts as All child units of Acme Corp/)).toBeInTheDocument();
    expect(screen.getByText('The "Every organization unit" policy')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Delete policy and change scope'})).toBeInTheDocument();
  });

  it('words choosing only the owner as stopping sharing', () => {
    renderDialog(accessSwitch({field: 'scope', to: 'owner', cleared: [{kind: 'trees', ouIds: ['x']}]}));

    expect(screen.getByText('Stop sharing Storefront?')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Stop sharing'})).toBeInTheDocument();
  });

  it('does not word a section switch to none as stopping sharing', () => {
    renderDialog(
      accessSwitch({field: 'childUnitsAccess', to: 'none', cleared: [{kind: 'childUnits', ouIds: ['retail']}]}),
    );

    expect(screen.getByText('Switch to Selected child units?')).toBeInTheDocument();
    expect(screen.queryByText('Stop sharing Storefront?')).not.toBeInTheDocument();
  });

  it('mentions unit settings only for resources that have them', () => {
    renderDialog(accessSwitch({cleared: [{kind: 'trees', ouIds: ['x']}]}), {hasUnitSettings: true});

    expect(screen.getByText(/Unit settings are cleared too/)).toBeInTheDocument();
  });

  it('confirms and cancels', () => {
    const value = accessSwitch({cleared: [{kind: 'trees', ouIds: ['x']}]});
    const {onCancel, onConfirm} = renderDialog(value);

    fireEvent.click(screen.getByRole('button', {name: 'Switch and clear'}));
    expect(onConfirm).toHaveBeenCalledWith(value);

    fireEvent.click(screen.getByRole('button', {name: 'Keep current choice'}));
    expect(onCancel).toHaveBeenCalled();
  });

  it("says stopping sharing in the resource's own words", () => {
    renderDialog(accessSwitch({field: 'scope', to: 'owner'}), {wordingNamespace: 'users'});

    expect(screen.getByText('Keep Storefront in Acme Corp only?')).toBeInTheDocument();
    expect(screen.getByText(/Storefront will be affiliated with Acme Corp only\./)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Remove affiliations'})).toBeInTheDocument();
    expect(screen.queryByText('Stop sharing Storefront?')).not.toBeInTheDocument();
  });
});
