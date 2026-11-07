// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationUnit} from '@thunderid/configure-organization-units';
import {screen, fireEvent, renderWithProviders, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {OrganizationAccessState} from '../../hooks/useOrganizationAccess';
import type {AccessSwitch, OrganizationAccessDraft} from '../../models/organization-access';
import {EMPTY_ORGANIZATION_ACCESS_DRAFT} from '../../utils/sharingPolicyToDraft';
import OrganizationAccessSettings from '../OrganizationAccessSettings';

const units: Record<string, string> = {
  acme: 'Acme Corp',
  trials: 'Acme Trials',
  retail: 'Acme Retail',
  northwind: 'Northwind Partners',
  contoso: 'Contoso Resellers',
};
vi.mock('@thunderid/configure-organization-units', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-organization-units')>()),
  useGetOrganizationUnitsByIds: (ids: string[]) =>
    new Map<string, OrganizationUnit>(
      ids.filter((id) => units[id]).map((id) => [id, {id, handle: id, name: units[id]} as OrganizationUnit]),
    ),
}));

const mockCandidates = vi.hoisted(() => ({
  roles: [
    {id: 'r1', name: 'Auditor', ouId: 'acme'},
    {id: 'r2', name: 'Support', ouId: 'acme'},
  ],
}));
vi.mock('@/api/useGetMandatableRoles', () => ({
  default: vi.fn((ouId: string) => ({
    data:
      ouId === 'retail'
        ? {candidates: mockCandidates.roles, names: {r1: 'Auditor', r2: 'Support', r3: 'Legacy'}}
        : {candidates: [], names: {}},
    isLoading: false,
    isError: false,
  })),
}));

vi.mock('@/components/OrganizationUnitSelectDialog', () => ({
  default: ({title, onConfirm}: {title: string; onConfirm: (selection: {ouId: string}[]) => void}) => (
    <div role="dialog" aria-label={title}>
      <button type="button" onClick={() => onConfirm([{ouId: 'labs'}])}>
        pick labs
      </button>
    </div>
  ),
}));

function accessWith(
  draft: Partial<OrganizationAccessDraft>,
  overrides: Partial<OrganizationAccessState> = {},
): OrganizationAccessState {
  const full: OrganizationAccessDraft = {...EMPTY_ORGANIZATION_ACCESS_DRAFT, scope: 'chosen', ...draft};

  return {
    isLoading: false,
    loadError: null,
    reload: vi.fn(),
    savedPolicy: undefined,
    draft: full,
    offers: {everyUnit: false, everyTree: false},
    assignmentsOff: {everyUnit: false, allChildUnits: false, everyTree: false, ouIds: []},
    isDeclared: false,
    hasChanges: false,
    updateDraft: vi.fn(),
    previewSwitch: vi.fn(
      (field: AccessSwitch['field'], to: AccessSwitch['to']): AccessSwitch => ({
        field,
        to,
        next: {...full, [field]: to},
        cleared: full.childUnitExclusions.length
          ? [{kind: 'childUnitExclusions', ouIds: full.childUnitExclusions}]
          : [],
        losesEveryUnit: false,
        losesEveryTree: false,
        losesAssignmentsOff: false,
      }),
    ),
    applySwitch: vi.fn(),
    reset: vi.fn(),
    save: vi.fn(),
    isSaving: false,
    saveError: null,
    ...overrides,
  };
}

const renderSettings = (access: OrganizationAccessState) =>
  renderWithProviders(<OrganizationAccessSettings access={access} ownerOuId="acme" resourceName="Storefront" />);

const radio = (group: string, name: RegExp) =>
  within(screen.getByRole('radiogroup', {name: group})).getByRole('radio', {name});

describe('OrganizationAccessSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows a loading indicator while the policies load', () => {
    renderSettings(accessWith({}, {isLoading: true}));

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('shows a retryable notice when the policies fail to load', () => {
    const access = accessWith({}, {loadError: new Error('down')});
    renderSettings(access);

    expect(screen.getByText('Failed to load organization access')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: /retry|refresh|try again/i}));
    expect(access.reload).toHaveBeenCalled();
  });

  it("uses the resource's own wording from its namespace", () => {
    renderWithProviders(
      <OrganizationAccessSettings
        access={accessWith({scope: 'owner'}, {wordingNamespace: 'users'})}
        ownerOuId="acme"
        resourceName="Alice"
      />,
    );

    expect(
      radio(
        'Where Alice is affiliated',
        /Acme Corp organization unit only Alice isn't affiliated with any other organization unit\./,
      ),
    ).toBeChecked();
    expect(
      screen.getByText(/Alice can sign in and act within the organization units selected here\./),
    ).toBeInTheDocument();
    expect(screen.queryByText('Who can use Alice')).not.toBeInTheDocument();
    expect(screen.queryByText('No child organization unit and no other tree can use it.')).not.toBeInTheDocument();
  });

  it('keeps the generic wording for a sentence the namespace leaves out', () => {
    // applications:organizationAccess.* sets the heading but not the load error.
    renderWithProviders(
      <OrganizationAccessSettings
        access={accessWith({}, {loadError: new Error('down'), wordingNamespace: 'applications'})}
        ownerOuId="acme"
        resourceName="Storefront"
      />,
    );

    expect(screen.getByText('Failed to load organization access')).toBeInTheDocument();
  });

  it('hides both sections when only the owner can use the resource', () => {
    renderSettings(accessWith({scope: 'owner'}));

    expect(radio('Who can use Storefront', /Acme Corp organization unit only/)).toBeChecked();
    expect(screen.queryByText('Child organization units')).not.toBeInTheDocument();
    expect(screen.queryByText('Other organization trees')).not.toBeInTheDocument();
  });

  it('sets the owner name in a chip in the scope titles only', () => {
    renderSettings(accessWith({scope: 'chosen', childUnitsAccess: 'all', otherTreesAccess: 'none'}));

    for (const title of [/Acme Corp organization unit only/, /Acme Corp and chosen organization units/]) {
      const choice = radio('Who can use Storefront', title).closest('label')!;
      expect(within(choice).getByText('Acme Corp').closest('.MuiChip-root')).not.toBeNull();
    }
    expect(
      screen.getByText("Organization units in Acme Corp's own hierarchy.").querySelector('.MuiChip-root'),
    ).toBeNull();
  });

  it('decides child units and other trees independently', () => {
    renderSettings(accessWith({childUnitsAccess: 'none', otherTreesAccess: 'selected', trees: ['northwind']}));

    expect(radio('Child organization units', /None/)).toBeChecked();
    expect(radio('Other organization trees', /Selected trees/)).toBeChecked();
    expect(screen.getByText('Northwind Partners')).toBeInTheDocument();
  });

  it('keeps None read-only in one section while the other section is None', () => {
    renderSettings(accessWith({childUnitsAccess: 'all'}));

    expect(radio('Child organization units', /None/)).toBeDisabled();
    expect(screen.getByText('Not available while other trees is set to None.')).toBeInTheDocument();
    expect(radio('Other organization trees', /None/)).toBeChecked();
  });

  it('keeps None read-only in other trees while child units is None', () => {
    renderSettings(accessWith({otherTreesAccess: 'selected', trees: ['northwind']}));

    expect(radio('Other organization trees', /None/)).toBeDisabled();
    expect(screen.getByText('Not available while child organization units is set to None.')).toBeInTheDocument();
  });

  it('says when chosen units grant nothing yet', () => {
    renderSettings(accessWith({childUnitsAccess: 'selected'}));

    expect(screen.getByText(/Nothing is granted below yet/)).toBeInTheDocument();
  });

  it('shows the exceptions of all child units and adds picked ones', () => {
    const access = accessWith({childUnitsAccess: 'all', childUnitExclusions: ['trials']});
    renderSettings(access);

    expect(screen.getByText('Acme Trials')).toBeInTheDocument();
    fireEvent.click(screen.getByText('Add exception'));
    fireEvent.click(screen.getByRole('button', {name: 'pick labs'}));

    const update = (access.updateDraft as ReturnType<typeof vi.fn>).mock.calls[0][0] as (
      draft: OrganizationAccessDraft,
    ) => OrganizationAccessDraft;
    expect(update({...EMPTY_ORGANIZATION_ACCESS_DRAFT, childUnitExclusions: ['trials']}).childUnitExclusions).toEqual([
      'trials',
      'labs',
    ]);
  });

  it('lists the granted child units with their reach', () => {
    const access = accessWith({childUnitsAccess: 'selected', childUnits: [{ouId: 'retail', reach: 'unit'}]});
    renderSettings(access);

    expect(screen.getByText('Acme Retail')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: 'Include all OUs below'}));
    expect(access.updateDraft).toHaveBeenCalled();
  });

  it('confirms a switch that clears settings before applying it', () => {
    const access = accessWith({childUnitsAccess: 'all', childUnitExclusions: ['trials']});
    renderSettings(access);

    fireEvent.click(radio('Child organization units', /Selected child organization units/));

    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText('Switch to Selected child organization units?')).toBeInTheDocument();
    expect(within(dialog).getByText('Child organization unit exceptions: Acme Trials')).toBeInTheDocument();
    expect(access.applySwitch).not.toHaveBeenCalled();

    fireEvent.click(within(dialog).getByRole('button', {name: 'Switch and clear'}));
    expect(access.applySwitch).toHaveBeenCalled();
  });

  it('notes where assignments are turned off, without a way to change it', () => {
    renderSettings(
      accessWith(
        {
          childUnitsAccess: 'selected',
          childUnits: [
            {ouId: 'retail', reach: 'unit'},
            {ouId: 'trials', reach: 'subtree'},
          ],
          otherTreesAccess: 'every',
        },
        {
          offers: {everyUnit: false, everyTree: true},
          assignmentsOff: {everyUnit: false, allChildUnits: false, everyTree: true, ouIds: ['retail']},
        },
      ),
    );

    expect(screen.getAllByText('Assignments are turned off for this organization unit.')).toHaveLength(1);
    expect(screen.getByText('Assignments are turned off for these organization units.')).toBeInTheDocument();
    expect(screen.queryByRole('checkbox', {name: /assign/i})).not.toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
  });

  it('confirms a switch that only drops a rule turning assignments off', () => {
    const access = accessWith({childUnitsAccess: 'all'});
    vi.mocked(access.previewSwitch).mockImplementation((field, to) => ({
      field,
      to,
      next: {...access.draft, [field]: to},
      cleared: [],
      losesEveryUnit: false,
      losesEveryTree: false,
      losesAssignmentsOff: true,
    }));
    renderSettings(access);

    fireEvent.click(radio('Child organization units', /Selected child organization units/));

    const dialog = screen.getByRole('dialog');
    expect(
      within(dialog).getByText(/Assignments are turned off for some organization units this drops\./),
    ).toBeInTheDocument();
    expect(within(dialog).queryByText(/Switching clears:/)).not.toBeInTheDocument();
    expect(access.applySwitch).not.toHaveBeenCalled();
  });

  it('keeps the current choice when the switch is cancelled', () => {
    const access = accessWith({childUnitsAccess: 'all', childUnitExclusions: ['trials']});
    renderSettings(access);

    fireEvent.click(radio('Child organization units', /Selected child organization units/));
    fireEvent.click(screen.getByRole('button', {name: 'Keep current choice'}));

    expect(access.applySwitch).not.toHaveBeenCalled();
  });

  it('applies a switch that clears nothing straight away', () => {
    const access = accessWith({scope: 'owner'});
    renderSettings(access);

    fireEvent.click(radio('Who can use Storefront', /Acme Corp and chosen organization units/));

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(access.previewSwitch).toHaveBeenCalledWith('scope', 'chosen');
    expect(access.applySwitch).toHaveBeenCalled();
  });

  it('does not offer every organization unit unless the saved policy holds it', () => {
    renderSettings(accessWith({scope: 'owner'}));

    expect(radio('Who can use Storefront', /Acme Corp organization unit only/)).toBeChecked();
    expect(radio('Who can use Storefront', /Acme Corp and chosen organization units/)).toBeEnabled();
    expect(screen.queryByRole('radio', {name: /^Every organization unit/})).not.toBeInTheDocument();
  });

  it('shows every organization unit as the only choice, with its exceptions before changing the scope', () => {
    const access = accessWith(
      {scope: 'everyUnit', everyUnitExclusions: ['trials']},
      {offers: {everyUnit: true, everyTree: false}},
    );
    renderSettings(access);

    expect(radio('Who can use Storefront', /Every organization unit/)).toBeChecked();
    expect(screen.queryByRole('radio', {name: /Acme Corp organization unit only/})).not.toBeInTheDocument();
    expect(screen.queryByRole('radio', {name: /Acme Corp and chosen organization units/})).not.toBeInTheDocument();
    expect(screen.queryByText('Other organization trees')).not.toBeInTheDocument();

    const exception = screen.getByText('Acme Trials');
    const changeScope = screen.getByRole('button', {name: 'Change scope'});
    expect(exception.compareDocumentPosition(changeScope) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    fireEvent.click(changeScope);
    expect(access.previewSwitch).toHaveBeenCalledWith('scope', 'chosen');
  });

  it('does not offer changing the scope of a declared every organization unit policy', () => {
    renderSettings(accessWith({scope: 'everyUnit'}, {offers: {everyUnit: true, everyTree: false}, isDeclared: true}));

    expect(radio('Who can use Storefront', /Every organization unit/)).toBeChecked();
    expect(screen.queryByRole('button', {name: 'Change scope'})).not.toBeInTheDocument();
  });

  it('warns that a dropped deployment-wide choice cannot come back', () => {
    renderSettings(accessWith({childUnitsAccess: 'all'}, {offers: {everyUnit: true, everyTree: false}}));

    expect(screen.getByText(/Saving deletes the "Every organization unit" policy/)).toBeInTheDocument();
    expect(screen.queryByRole('radio', {name: /^Every organization unit/})).not.toBeInTheDocument();
  });

  it('shows every tree with its exceptions while the saved policy holds it', () => {
    renderSettings(
      accessWith(
        {childUnitsAccess: 'all', otherTreesAccess: 'every', everyTreeExclusions: ['contoso']},
        {offers: {everyUnit: false, everyTree: true}},
      ),
    );

    expect(radio('Other organization trees', /Every tree/)).toBeChecked();
    expect(screen.getByText('Contoso Resellers')).toBeInTheDocument();
    expect(screen.queryByText('Add a tree')).not.toBeInTheDocument();
  });

  it('is read-only for a declared policy', () => {
    renderSettings(
      accessWith(
        {
          childUnitsAccess: 'selected',
          childUnits: [{ouId: 'retail', reach: 'subtree'}],
          otherTreesAccess: 'selected',
          trees: ['northwind'],
        },
        {isDeclared: true},
      ),
    );

    expect(radio('Who can use Storefront', /Acme Corp organization unit only/)).toBeDisabled();
    expect(radio('Child organization units', /None/)).toBeDisabled();
    expect(screen.queryByText('Grant access')).not.toBeInTheDocument();
    expect(screen.queryByText('Add a tree')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /Revoke access/})).not.toBeInTheDocument();
  });

  describe('content a resource draws under each granted unit', () => {
    const extra = (ouId: string, name: string) => <div data-testid={`extra-${ouId}`}>{`Terms for ${name}`}</div>;
    const access = () =>
      accessWith({
        childUnitsAccess: 'selected',
        childUnits: [
          {ouId: 'retail', reach: 'unit'},
          {ouId: 'trials', reach: 'subtree'},
        ],
      });

    it('shows it under the row of each named child unit, with the unit name', () => {
      renderWithProviders(
        <OrganizationAccessSettings
          access={access()}
          ownerOuId="acme"
          resourceName="Storefront"
          renderChildUnitExtra={extra}
        />,
      );

      expect(
        within(screen.getByText('Acme Retail').closest('li') as HTMLElement).getByTestId('extra-retail'),
      ).toHaveTextContent('Terms for Acme Retail');
      expect(
        within(screen.getByText('Acme Trials').closest('li') as HTMLElement).getByTestId('extra-trials'),
      ).toBeInTheDocument();
    });

    it('keeps the roles picker beside it for a resource that mandates roles', () => {
      renderWithProviders(
        <OrganizationAccessSettings
          access={access()}
          ownerOuId="acme"
          resourceName="Storefront"
          mandateRoles
          renderChildUnitExtra={extra}
        />,
      );

      const row = screen.getByText('Acme Retail').closest('li') as HTMLElement;

      expect(within(row).getByRole('combobox', {name: 'Mandated roles'})).toBeInTheDocument();
      expect(within(row).getByTestId('extra-retail')).toBeInTheDocument();
    });

    it('draws nothing extra when the resource gives none', () => {
      renderSettings(access());

      expect(screen.queryByTestId('extra-retail')).not.toBeInTheDocument();
    });
  });

  describe('mandated roles', () => {
    const named = (reach: 'unit' | 'subtree', roles?: string[]) =>
      accessWith({childUnitsAccess: 'selected', childUnits: [{ouId: 'retail', reach, roles}]});
    const renderMandating = (access: OrganizationAccessState) =>
      renderWithProviders(
        <OrganizationAccessSettings access={access} ownerOuId="acme" resourceName="Storefront" mandateRoles />,
      );

    // Every unit's picker carries the same label, so it is found inside that unit's row.
    const rolesPicker = (unitName: string): HTMLElement =>
      within(screen.getByText(unitName).closest('li') as HTMLElement).getByRole('combobox', {name: 'Mandated roles'});

    it('shows a role picker for each unit named with a unit-only reach', () => {
      renderMandating(
        accessWith({
          childUnitsAccess: 'selected',
          childUnits: [
            {ouId: 'retail', reach: 'unit'},
            {ouId: 'trials', reach: 'unit'},
          ],
        }),
      );

      expect(rolesPicker('Acme Retail')).toBeInTheDocument();
      expect(rolesPicker('Acme Trials')).toBeInTheDocument();
    });

    it('lists only the candidates the roles API returned for the unit', () => {
      renderMandating(named('unit'));

      fireEvent.mouseDown(rolesPicker('Acme Retail'));

      expect(screen.getByRole('option', {name: 'Auditor'})).toBeInTheDocument();
      expect(screen.getByRole('option', {name: 'Support'})).toBeInTheDocument();
    });

    it('offers nothing for a unit none of the issuer roles are shared with', () => {
      renderMandating(accessWith({childUnitsAccess: 'selected', childUnits: [{ouId: 'trials', reach: 'unit'}]}));

      fireEvent.mouseDown(rolesPicker('Acme Trials'));

      expect(screen.getByText('No roles of yours are shared with this organization unit.')).toBeInTheDocument();
    });

    it('hides the picker for a subtree reach and says why', () => {
      renderMandating(named('subtree'));

      expect(screen.queryByRole('combobox', {name: 'Mandated roles'})).not.toBeInTheDocument();
      expect(screen.getByText('Roles can be mandated only when access is set to Only this OU.')).toBeInTheDocument();
    });

    it('hides the picker when every child unit is granted', () => {
      renderMandating(accessWith({childUnitsAccess: 'all'}));

      expect(screen.queryByRole('combobox', {name: 'Mandated roles'})).not.toBeInTheDocument();
    });

    it('hides the picker for a resource that does not mandate roles', () => {
      renderSettings(named('unit'));

      expect(screen.queryByRole('combobox', {name: 'Mandated roles'})).not.toBeInTheDocument();
    });

    it('shows the chosen roles and writes a pick to the unit', () => {
      const access = named('unit', ['r1']);
      renderMandating(access);

      expect(screen.getByText('Auditor')).toBeInTheDocument();

      fireEvent.mouseDown(rolesPicker('Acme Retail'));
      fireEvent.click(screen.getByRole('option', {name: 'Support'}));

      const update = vi.mocked(access.updateDraft).mock.calls[0][0];
      expect(update(access.draft).childUnits).toEqual([{ouId: 'retail', reach: 'unit', roles: ['r1', 'r2']}]);
    });

    it('drops the roles when the reach widens to the subtree', () => {
      const access = named('unit', ['r1']);
      renderMandating(access);

      fireEvent.click(screen.getByRole('button', {name: 'Include all OUs below'}));

      const update = vi.mocked(access.updateDraft).mock.calls[0][0];
      expect(update(access.draft).childUnits).toEqual([{ouId: 'retail', reach: 'subtree'}]);
    });
  });
});
