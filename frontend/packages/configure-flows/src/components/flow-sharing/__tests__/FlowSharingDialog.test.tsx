// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {FlowVariableDefinition} from '@thunderid/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FlowSharingDialog from '../FlowSharingDialog';

const mockAccess = {
  applySwitch: vi.fn(),
  draft: {
    childUnitExclusions: [],
    childUnits: [],
    childUnitsAccess: 'all',
    everyTreeExclusions: [],
    everyUnitExclusions: [],
    otherTreesAccess: 'none',
    scope: 'chosen',
    trees: [],
  } as Record<string, unknown>,
  hasChanges: false,
  isDeclared: false,
  isLoading: false,
  isSaving: false,
  reset: vi.fn(),
  save: vi.fn(),
  saveError: null as Error | null,
  updateDraft: vi.fn(),
};

const mockUseOrganizationAccess = vi.fn<(options: {resourcePath: string; enabled?: boolean}) => typeof mockAccess>(
  () => mockAccess,
);

vi.mock('@thunderid/configure-resource-sharing', () => ({
  OrganizationAccessSettings: ({
    childUnitTerms = undefined,
    childUnitsAction = undefined,
    ownerOuId,
    readOnly = false,
  }: {
    childUnitTerms?: {
      expandedOuId?: string | null;
      onEdit: (ouId: string, name: string) => void;
      renderPanel?: (ouId: string, name: string) => ReactNode;
    };
    childUnitsAction?: ReactNode;
    ownerOuId: string;
    readOnly?: boolean;
  }) => (
    <div data-testid="organization-access" data-owner={ownerOuId} data-read-only={String(readOnly)}>
      {childUnitsAction}
      {childUnitTerms && (
        <>
          <button type="button" onClick={() => childUnitTerms.onEdit('retail', 'Acme Retail')}>
            edit-retail
          </button>
          {childUnitTerms.expandedOuId && childUnitTerms.renderPanel?.(childUnitTerms.expandedOuId, 'Acme Retail')}
        </>
      )}
    </div>
  ),
  useOrganizationAccess: (options: {resourcePath: string; enabled?: boolean}) => mockUseOrganizationAccess(options),
}));

vi.mock('../UnitTermsDrawer', () => ({
  default: ({
    applyLabel,
    onApply,
    rules,
    title,
  }: {
    applyLabel: string;
    onApply: (rules: unknown) => void;
    rules: unknown;
    title: string;
  }) => (
    <div data-testid="unit-terms-drawer" data-rules={JSON.stringify(rules ?? null)}>
      <span>{title}</span>
      <button type="button" onClick={() => onApply({'variables.connection1': {editable: true}})}>
        {applyLabel}
      </button>
    </div>
  ),
}));

vi.mock('../VariableSharingRules', () => ({
  default: ({
    onChange,
    rules,
    variables,
  }: {
    onChange: (rules: unknown) => void;
    rules: unknown;
    variables: unknown[];
  }) => (
    <div data-testid="variable-sharing-rules" data-count={variables.length} data-rules={JSON.stringify(rules ?? null)}>
      <button type="button" onClick={() => onChange({'variables.a': {editable: true}})}>
        change-rules
      </button>
    </div>
  ),
}));

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string, options?: Record<string, unknown>) =>
      Object.entries(options ?? {}).reduce(
        (text: string, [name, value]) => text.replace(`{{${name}}}`, String(value)),
        defaultValue ?? key,
      ),
  }),
}));

const flow = {id: 'flow-1', name: 'Enterprise sign-in', ouId: 'ou-root'};
const variables: FlowVariableDefinition[] = [{defaultValue: '', identifier: 'connection1', type: 'CONNECTION_SELECT'}];

describe('FlowSharingDialog', () => {
  const onClose = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockAccess.draft = {...mockAccess.draft, childUnitsAccess: 'all', scope: 'chosen'};
    mockAccess.hasChanges = false;
    mockAccess.isDeclared = false;
    mockAccess.isLoading = false;
    mockAccess.isSaving = false;
    mockAccess.saveError = null;
    mockAccess.save.mockResolvedValue(undefined);
  });

  const renderDialog = (props: Partial<React.ComponentProps<typeof FlowSharingDialog>> = {}) =>
    render(<FlowSharingDialog open flow={flow} variables={variables} onClose={onClose} {...props} />);

  it('manages the sharing policies of the flow, and only while it is open', () => {
    renderDialog({open: false});

    expect(mockUseOrganizationAccess).toHaveBeenCalledWith({enabled: false, resourcePath: 'flows/flow-1'});
  });

  it('shows who the flow is shared with from the flow owner', () => {
    renderDialog();

    expect(screen.getByText('Share flow')).toBeInTheDocument();
    expect(screen.getByTestId('organization-access')).toHaveAttribute('data-owner', 'ou-root');
  });

  it('shows what the units may do with the variables once child units are reached', () => {
    renderDialog();

    expect(screen.getByTestId('variable-sharing-rules')).toHaveAttribute('data-count', '1');
    expect(screen.queryByTestId('flow-sharing-variables-hint')).not.toBeInTheDocument();
  });

  it('asks to share with child units first when none is reached', () => {
    mockAccess.draft = {...mockAccess.draft, childUnitsAccess: 'none'};
    renderDialog();

    expect(screen.queryByTestId('variable-sharing-rules')).not.toBeInTheDocument();
    expect(screen.getByTestId('flow-sharing-variables-hint')).toBeInTheDocument();
  });

  it('says no terms are available for a flow with no variables', () => {
    renderDialog({variables: []});

    expect(screen.queryByTestId('variable-sharing-rules')).not.toBeInTheDocument();
    expect(screen.queryByTestId('flow-sharing-variables-hint')).not.toBeInTheDocument();
    expect(screen.getByTestId('flow-sharing-no-terms')).toBeInTheDocument();
  });

  it('puts the rules into the draft', async () => {
    const user = userEvent.setup();

    renderDialog();
    await user.click(screen.getByText('change-rules'));

    const update = mockAccess.updateDraft.mock.calls[0][0] as (
      draft: Record<string, unknown>,
    ) => Record<string, unknown>;

    expect(update({scope: 'chosen'})).toEqual({childUnitRules: {'variables.a': {editable: true}}, scope: 'chosen'});
  });

  describe('with selected child units', () => {
    beforeEach(() => {
      mockAccess.draft = {
        ...mockAccess.draft,
        childUnits: [
          {ouId: 'retail', reach: 'unit', rules: {'variables.connection1': {editable: true}}},
          {ouId: 'wholesale', reach: 'subtree'},
        ],
        childUnitsAccess: 'selected',
      };
    });

    it('shows no shared terms card until the terms of a unit are opened', () => {
      renderDialog();

      expect(screen.queryByTestId('variable-sharing-rules')).not.toBeInTheDocument();
    });

    it('opens the terms of a unit under its row and writes them to that unit only', async () => {
      const user = userEvent.setup();

      renderDialog();

      expect(screen.queryByTestId('variable-sharing-rules')).not.toBeInTheDocument();

      await user.click(screen.getByText('edit-retail'));

      expect(screen.getByTestId('variable-sharing-rules')).toHaveAttribute(
        'data-rules',
        JSON.stringify({'variables.connection1': {editable: true}}),
      );

      await user.click(screen.getByText('change-rules'));

      const update = mockAccess.updateDraft.mock.calls[0][0] as (draft: Record<string, unknown>) => {
        childUnits: unknown[];
      };

      expect(update(mockAccess.draft).childUnits).toEqual([
        {ouId: 'retail', reach: 'unit', rules: {'variables.a': {editable: true}}},
        {ouId: 'wholesale', reach: 'subtree'},
      ]);
    });

    it('closes the terms of a unit when its row is edited again', async () => {
      const user = userEvent.setup();

      renderDialog();
      await user.click(screen.getByText('edit-retail'));
      await user.click(screen.getByText('edit-retail'));

      expect(screen.queryByTestId('variable-sharing-rules')).not.toBeInTheDocument();
    });

    it('copies the starting terms to every unit', async () => {
      const user = userEvent.setup();

      renderDialog();
      await user.click(screen.getByTestId('flow-sharing-starting-terms'));
      await user.click(screen.getByText('Copy to 2 units'));

      const update = mockAccess.updateDraft.mock.calls[0][0] as (draft: Record<string, unknown>) => {
        childUnits: unknown[];
      };

      expect(update(mockAccess.draft).childUnits).toEqual([
        {ouId: 'retail', reach: 'unit', rules: {'variables.connection1': {editable: true}}},
        {ouId: 'wholesale', reach: 'subtree', rules: {'variables.connection1': {editable: true}}},
      ]);
    });
  });

  it('warns that unsaved changes to the flow keep a new variable from being shared', () => {
    renderDialog({isDirty: true});

    expect(screen.getByTestId('flow-sharing-unsaved')).toBeInTheDocument();
  });

  it('saves only when something changed, then closes', async () => {
    const user = userEvent.setup();

    renderDialog();

    expect(screen.getByTestId('flow-sharing-save')).toBeDisabled();

    mockAccess.hasChanges = true;
    renderDialog();
    await user.click(screen.getAllByTestId('flow-sharing-save')[1]);

    expect(mockAccess.save).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it('stays open and shows the error when the save fails', async () => {
    const user = userEvent.setup();

    mockAccess.hasChanges = true;
    mockAccess.save.mockRejectedValue(new Error('nope'));
    mockAccess.saveError = new Error('The variable is not declared');
    renderDialog();
    await user.click(screen.getByTestId('flow-sharing-save'));

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByText('The variable is not declared')).toBeInTheDocument();
  });

  it('discards the draft when it is closed', async () => {
    const user = userEvent.setup();

    renderDialog();
    await user.click(screen.getByText('Cancel'));

    expect(mockAccess.reset).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it('leaves a read-only flow as it is', () => {
    mockAccess.hasChanges = true;
    renderDialog({flow: {...flow, isReadOnly: true}});

    expect(screen.getByTestId('organization-access')).toHaveAttribute('data-read-only', 'true');
    expect(screen.getByTestId('flow-sharing-save')).toBeDisabled();
  });
});
