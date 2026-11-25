// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {OrganizationAccessState} from '@thunderid/configure-resource-sharing';
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

vi.mock('@thunderid/configure-resource-sharing', () => ({
  OrganizationAccessSettings: ({
    childUnitsAction = undefined,
    ownerOuId,
    readOnly = false,
    renderChildUnitExtra = undefined,
  }: {
    childUnitsAction?: ReactNode;
    ownerOuId: string;
    readOnly?: boolean;
    renderChildUnitExtra?: (ouId: string, name: string) => ReactNode;
  }) => (
    <div data-testid="organization-access" data-owner={ownerOuId} data-read-only={String(readOnly)}>
      {childUnitsAction}
      {renderChildUnitExtra?.('retail', 'Acme Retail')}
    </div>
  ),
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

const errorCatalog: Record<string, string> = {'flows:errors.FLM-1234': 'The flow could not be shared.'};

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, second?: string | Record<string, unknown>, third?: Record<string, unknown>) => {
      const options = typeof second === 'object' ? second : third;
      const defaultValue = typeof second === 'string' ? second : (options?.['defaultValue'] as string | undefined);

      return Object.entries(options ?? {}).reduce(
        (text: string, [name, value]) => text.replace(`{{${name}}}`, String(value)),
        errorCatalog[key] ?? defaultValue ?? key,
      );
    },
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
    render(
      <FlowSharingDialog
        open
        access={mockAccess as unknown as OrganizationAccessState}
        flow={flow}
        variables={variables}
        onClose={onClose}
        {...props}
      />,
    );

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
    expect(screen.queryByRole('button', {name: 'Edit terms for Acme Retail'})).not.toBeInTheDocument();
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

      await user.click(screen.getByRole('button', {name: 'Edit terms for Acme Retail'}));

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

    it('closes the terms of a unit when its button is pressed again', async () => {
      const user = userEvent.setup();

      renderDialog();
      await user.click(screen.getByRole('button', {name: 'Edit terms for Acme Retail'}));
      await user.click(screen.getByRole('button', {name: 'Edit terms for Acme Retail'}));

      expect(screen.queryByTestId('variable-sharing-rules')).not.toBeInTheDocument();
    });

    it('asks before the starting terms replace terms a unit already has', async () => {
      const user = userEvent.setup();

      renderDialog();
      await user.click(screen.getByTestId('flow-sharing-starting-terms'));
      await user.click(screen.getByText('Copy to 2 organization units'));

      expect(screen.getByTestId('variable-change-dialog')).toHaveTextContent(
        'replaces them for every organization unit',
      );
      expect(mockAccess.updateDraft).not.toHaveBeenCalled();

      await user.click(within(screen.getByTestId('variable-change-dialog')).getByRole('button', {name: 'Cancel'}));

      expect(mockAccess.updateDraft).not.toHaveBeenCalled();
      expect(screen.getByTestId('unit-terms-drawer')).toBeInTheDocument();
    });

    it('copies the starting terms to every unit once the replacement is confirmed', async () => {
      const user = userEvent.setup();

      renderDialog();
      await user.click(screen.getByTestId('flow-sharing-starting-terms'));
      await user.click(screen.getByText('Copy to 2 organization units'));
      await user.click(screen.getByTestId('variable-change-confirm'));

      const update = mockAccess.updateDraft.mock.calls[0][0] as (draft: Record<string, unknown>) => {
        childUnits: unknown[];
      };

      expect(update(mockAccess.draft).childUnits).toEqual([
        {ouId: 'retail', reach: 'unit', rules: {'variables.connection1': {editable: true}}},
        {ouId: 'wholesale', reach: 'subtree', rules: {'variables.connection1': {editable: true}}},
      ]);
    });
  });

  it('copies the starting terms without asking when no unit has terms yet', async () => {
    const user = userEvent.setup();

    mockAccess.draft = {
      ...mockAccess.draft,
      childUnits: [
        {ouId: 'retail', reach: 'unit'},
        {ouId: 'wholesale', reach: 'subtree'},
      ],
      childUnitsAccess: 'selected',
    };
    renderDialog();
    await user.click(screen.getByTestId('flow-sharing-starting-terms'));
    await user.click(screen.getByText('Copy to 2 organization units'));

    expect(screen.queryByTestId('variable-change-dialog')).not.toBeInTheDocument();
    expect(mockAccess.updateDraft).toHaveBeenCalledTimes(1);
  });

  it('shows the loading animation, with its accessible name, while the sharing policies load', () => {
    mockAccess.isLoading = true;
    renderDialog();

    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(screen.getByLabelText('Loading content')).toBeInTheDocument();
    expect(screen.queryByTestId('organization-access')).not.toBeInTheDocument();
  });

  it("has no Save button of its own, since the flow's Save saves the sharing too", () => {
    mockAccess.hasChanges = true;
    renderDialog();

    expect(screen.queryByRole('button', {name: 'Save'})).not.toBeInTheDocument();
    expect(screen.queryByTestId('flow-sharing-save')).not.toBeInTheDocument();
  });

  it('says its changes are saved with the flow once there are some', () => {
    renderDialog();

    expect(screen.queryByTestId('flow-sharing-saved-with-flow')).not.toBeInTheDocument();

    mockAccess.hasChanges = true;
    renderDialog();

    expect(screen.getByTestId('flow-sharing-saved-with-flow')).toHaveTextContent(
      'These changes are saved when you save the flow.',
    );
  });

  it("closes and keeps the draft, for the flow's Save to save", async () => {
    const user = userEvent.setup();

    mockAccess.hasChanges = true;
    renderDialog();
    await user.click(screen.getByTestId('flow-sharing-done'));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(mockAccess.reset).not.toHaveBeenCalled();
    expect(mockAccess.save).not.toHaveBeenCalled();
  });

  it('closes from Escape and the close button without asking, as nothing is thrown away', async () => {
    const user = userEvent.setup();

    mockAccess.hasChanges = true;
    renderDialog();
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('button', {name: 'Close'}));

    expect(onClose).toHaveBeenCalledTimes(2);
    expect(mockAccess.reset).not.toHaveBeenCalled();
    expect(screen.queryByTestId('discard-changes-dialog')).not.toBeInTheDocument();
  });

  it('shows why the last save was refused, with a generic message and not the server text', () => {
    mockAccess.saveError = new Error('The variable is not declared');
    renderDialog();

    expect(screen.getByText('Failed to update how the flow is shared. Please try again.')).toBeInTheDocument();
    expect(screen.queryByText('The variable is not declared')).not.toBeInTheDocument();
  });

  it('shows the catalog message for the error code the server answered with', () => {
    mockAccess.saveError = Object.assign(new Error('The variable is not declared'), {
      response: {data: {code: 'FLM-1234'}},
    });
    renderDialog();

    expect(screen.getByText('The flow could not be shared.')).toBeInTheDocument();
  });

  it('leaves a read-only flow as it is', () => {
    mockAccess.hasChanges = true;
    renderDialog({flow: {...flow, isReadOnly: true}});

    expect(screen.getByTestId('organization-access')).toHaveAttribute('data-read-only', 'true');
    expect(screen.queryByTestId('flow-sharing-saved-with-flow')).not.toBeInTheDocument();
  });
});
