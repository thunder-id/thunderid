// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AuthorizationMappingSection from '../AuthorizationMappingSection';

vi.mock('@thunderid/components', () => ({
  SettingsCard: ({title, children}: {title: string; children: React.ReactNode}) => (
    <section aria-label={title}>{children}</section>
  ),
  ReadOnlyFieldset: ({readOnly, children}: {readOnly: boolean; children: React.ReactNode}) => (
    <fieldset disabled={readOnly}>{children}</fieldset>
  ),
}));

// The target pickers are out of scope for this component; these are mocked only so the always-present
// Roles/Groups/Permissions grant fields and the resource server select don't need a real React Query
// provider or network calls.
const {useGetRolesForMapping, useGetGroupsForMapping, useGetResourceServers, SERVERS_PAGE, ROLES_PAGE} = vi.hoisted(
  () => {
    const serversPage = {data: {totalResults: 1, resourceServers: [{id: 'rs-1', name: 'Orders API'}]}};
    const rolesPage = {
      data: {
        roles: [
          {id: 'role-1', name: 'Admins'},
          {id: 'role-2', name: 'Editors'},
        ],
        totalResults: 2,
      },
    };
    return {
      ROLES_PAGE: rolesPage,
      SERVERS_PAGE: serversPage,
      useGetResourceServers: vi.fn(() => serversPage),
      useGetRolesForMapping: vi.fn(() => rolesPage),
      useGetGroupsForMapping: vi.fn(() => ({data: {groups: [{id: 'group-1', name: 'Engineering'}], totalResults: 1}})),
    };
  },
);
vi.mock('../../api/useGetRolesForMapping', () => ({default: useGetRolesForMapping}));
vi.mock('../../api/useGetGroupsForMapping', () => ({default: useGetGroupsForMapping}));
vi.mock('@thunderid/configure-resource-servers', () => ({
  PermissionCatalog: ({ouId = undefined, readOnly = false}: {ouId?: string; readOnly?: boolean}) => (
    <div data-testid="permission-catalog" data-ou-id={ouId} data-readonly={String(readOnly)} />
  ),
  removePermissions: () => [],
  useGetResourceServers,
}));

const ADD_MAPPING = 'Add mapping';
const ADD_VALUE = 'Add value';
const CLAIM = 'Key';
const VALUE_TYPE = 'Value type';
const DELIMITER = 'Delimiter';
const OPERATOR = 'Operator';
const EXTERNAL_VALUE = 'External value';
const TARGET_TYPE = 'Target type';
const RESOURCE_SERVER = 'Resource server';
const ADVANCED_TOGGLE = 'Advanced rules';
const COMMA_PRESET = 'Comma ( , )';
const NO_DELIMITER_PRESET = 'No delimiter (single value)';

/** Opens a Select identified by its aria-label and picks the option with the given accessible name. */
function selectOption(comboboxLabel: string, optionName: string): void {
  fireEvent.mouseDown(screen.getByRole('combobox', {name: comboboxLabel}));
  fireEvent.click(screen.getByRole('option', {name: optionName}));
}

function operatorText(): string {
  return screen.getByRole('combobox', {name: OPERATOR}).textContent ?? '';
}

describe('AuthorizationMappingSection', () => {
  const onRuleChange = vi.fn();
  const onDirectChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    useGetRolesForMapping.mockReturnValue(ROLES_PAGE);
    useGetResourceServers.mockReturnValue(SERVERS_PAGE);
  });

  it('defaults a new mapping to the simple (non-advanced) side, Role target type, no resource server picker', () => {
    render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
    fireEvent.click(screen.getByRole('button', {name: ADD_MAPPING}));

    expect(screen.getByLabelText(CLAIM)).toBeInTheDocument();
    expect(screen.getByRole('switch', {name: ADVANCED_TOGGLE})).not.toBeChecked();
    expect(screen.getByRole('combobox', {name: TARGET_TYPE})).toHaveTextContent('Role');
    expect(screen.queryByRole('combobox', {name: RESOURCE_SERVER})).not.toBeInTheDocument();
    expect(screen.queryByLabelText(VALUE_TYPE)).not.toBeInTheDocument();
  });

  it('opens a row already on the advanced side when the connection has only rule-based mappings', () => {
    render(
      <AuthorizationMappingSection
        initialRuleConfig={[{claim: 'groups', values: []}]}
        onRuleChange={onRuleChange}
        onDirectChange={onDirectChange}
      />,
    );

    expect(screen.getByRole('switch', {name: ADVANCED_TOGGLE})).toBeChecked();
    expect(screen.getByLabelText(VALUE_TYPE)).toBeInTheDocument();
  });

  it('locks every control but keeps the permission tree outside the lock when read-only', () => {
    render(
      <AuthorizationMappingSection
        ouId="ou-1"
        initialRuleConfig={[
          {claim: 'groups', values: [{operator: 'equals', value: 'x', targets: [{type: 'role', id: 'role-1'}]}]},
        ]}
        onRuleChange={onRuleChange}
        onDirectChange={onDirectChange}
        readOnly
      />,
    );

    expect(screen.getByRole('textbox', {name: CLAIM})).toBeDisabled();
    expect(screen.getByRole('switch', {name: ADVANCED_TOGGLE})).toBeDisabled();
    expect(screen.queryByRole('button', {name: ADD_MAPPING})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: ADD_VALUE})).not.toBeInTheDocument();
    const catalog = screen.getByTestId('permission-catalog');
    expect(catalog).toHaveAttribute('data-readonly', 'true');
    expect(catalog.closest('fieldset:disabled')).toBeNull();
  });

  it('opens a row on the simple side when the connection has only direct mappings', () => {
    render(
      <AuthorizationMappingSection
        initialDirectConfig={[{claim: 'groups', targetType: 'role'}]}
        onRuleChange={onRuleChange}
        onDirectChange={onDirectChange}
      />,
    );

    expect(screen.getByRole('switch', {name: ADVANCED_TOGGLE})).not.toBeChecked();
    expect(screen.getByRole('combobox', {name: TARGET_TYPE})).toBeInTheDocument();
  });

  it('renders one row per mechanism when the connection has both rule and direct mappings configured', () => {
    render(
      <AuthorizationMappingSection
        initialRuleConfig={[
          {claim: 'groups', values: [{operator: 'equals', value: 'x', targets: [{type: 'role', id: 'role-1'}]}]},
        ]}
        initialDirectConfig={[{claim: 'perms', targetType: 'permission', resourceServerId: 'rs-1'}]}
        onRuleChange={onRuleChange}
        onDirectChange={onDirectChange}
      />,
    );

    const toggles = screen.getAllByRole('switch', {name: ADVANCED_TOGGLE});
    expect(toggles).toHaveLength(2);
    expect(toggles[0]).not.toBeChecked();
    expect(toggles[1]).toBeChecked();
  });

  it('switches a row to the advanced side when the toggle is turned on, revealing Value Type and the rules table', () => {
    render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
    fireEvent.click(screen.getByRole('button', {name: ADD_MAPPING}));

    fireEvent.click(screen.getByRole('switch', {name: ADVANCED_TOGGLE}));

    expect(screen.getByLabelText(VALUE_TYPE)).toBeInTheDocument();
    expect(screen.queryByRole('combobox', {name: TARGET_TYPE})).not.toBeInTheDocument();
    expect(screen.getByLabelText(EXTERNAL_VALUE)).toBeInTheDocument();
  });

  it('preserves the target type chosen on the simple side across a toggle to advanced and back', () => {
    render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
    fireEvent.click(screen.getByRole('button', {name: ADD_MAPPING}));
    selectOption(TARGET_TYPE, 'Group');

    fireEvent.click(screen.getByRole('switch', {name: ADVANCED_TOGGLE}));
    fireEvent.click(screen.getByRole('switch', {name: ADVANCED_TOGGLE}));

    expect(screen.getByRole('combobox', {name: TARGET_TYPE})).toHaveTextContent('Group');
  });

  it('preserves claim and delimiter across a toggle to advanced', () => {
    render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
    fireEvent.click(screen.getByRole('button', {name: ADD_MAPPING}));
    fireEvent.change(screen.getByLabelText(CLAIM), {target: {value: 'groups'}});
    selectOption(DELIMITER, COMMA_PRESET);

    fireEvent.click(screen.getByRole('switch', {name: ADVANCED_TOGGLE}));

    expect(screen.getByLabelText(CLAIM)).toHaveValue('groups');
    expect(screen.getByRole('combobox', {name: DELIMITER})).toHaveTextContent(COMMA_PRESET);
  });

  it('resets a seeded rule left on "Equals" to "Includes" when toggling to advanced with a delimiter already set', () => {
    render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
    fireEvent.click(screen.getByRole('button', {name: ADD_MAPPING}));
    fireEvent.change(screen.getByLabelText(CLAIM), {target: {value: 'groups'}});
    // Delimiter is set while still on the simple side, where there is no rule table to validate the
    // operator against - the mismatch can only surface once Advanced is switched on.
    selectOption(DELIMITER, COMMA_PRESET);

    fireEvent.click(screen.getByRole('switch', {name: ADVANCED_TOGGLE}));

    expect(operatorText()).toBe('Includes');
  });

  it('shows the resource server picker only for the Permission target type, and clears it when switching away', () => {
    render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
    fireEvent.click(screen.getByRole('button', {name: ADD_MAPPING}));

    expect(screen.queryByRole('combobox', {name: RESOURCE_SERVER})).not.toBeInTheDocument();

    selectOption(TARGET_TYPE, 'Permission');
    expect(screen.getByRole('combobox', {name: RESOURCE_SERVER})).toBeInTheDocument();

    selectOption(RESOURCE_SERVER, 'Orders API');
    selectOption(TARGET_TYPE, 'Group');
    expect(screen.queryByRole('combobox', {name: RESOURCE_SERVER})).not.toBeInTheDocument();
  });

  it('marks a direct mapping to a resource server the unit cannot use as unavailable', () => {
    render(
      <AuthorizationMappingSection
        ouId="ou-1"
        initialDirectConfig={[{claim: 'perms', targetType: 'permission', resourceServerId: 'rs-other-unit'}]}
        onRuleChange={onRuleChange}
        onDirectChange={onDirectChange}
      />,
    );

    expect(screen.getByText('Unavailable: rs-other-unit')).toBeInTheDocument();
  });

  it('shows a stored resource server missing from a partial list by its id, without marking it unavailable', () => {
    useGetResourceServers.mockReturnValue({data: {...SERVERS_PAGE.data, totalResults: 250}});
    render(
      <AuthorizationMappingSection
        ouId="ou-1"
        initialDirectConfig={[{claim: 'perms', targetType: 'permission', resourceServerId: 'rs-later-page'}]}
        onRuleChange={onRuleChange}
        onDirectChange={onDirectChange}
      />,
    );

    expect(screen.getByText('rs-later-page')).toBeInTheDocument();
    expect(screen.queryByText('Unavailable: rs-later-page')).not.toBeInTheDocument();
  });

  it('reports a complete simple-side mapping via onDirectChange, and nothing via onRuleChange', () => {
    render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
    fireEvent.click(screen.getByRole('button', {name: ADD_MAPPING}));

    fireEvent.change(screen.getByLabelText(CLAIM), {target: {value: 'groups'}});

    expect(onDirectChange).toHaveBeenLastCalledWith([{claim: 'groups', targetType: 'role'}], true);
    expect(onRuleChange).toHaveBeenLastCalledWith(undefined, true);
  });

  it('flags an incomplete permission mapping (claim set, no resource server chosen) as invalid', () => {
    render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
    fireEvent.click(screen.getByRole('button', {name: ADD_MAPPING}));
    selectOption(TARGET_TYPE, 'Permission');

    fireEvent.change(screen.getByLabelText(CLAIM), {target: {value: 'scope'}});

    expect(onDirectChange).toHaveBeenLastCalledWith(undefined, false);
    expect(onRuleChange).toHaveBeenLastCalledWith(undefined, false);
  });

  it('flags the claim field itself once the row is attempted but the claim is left blank', () => {
    render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
    fireEvent.click(screen.getByRole('button', {name: ADD_MAPPING}));

    expect(screen.getByLabelText(CLAIM)).toHaveAttribute('aria-invalid', 'false');

    selectOption(TARGET_TYPE, 'Group');

    expect(screen.getByLabelText(CLAIM)).toHaveAttribute('aria-invalid', 'true');
  });

  it('removes a mapping row and reflects the change via onChange', () => {
    render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
    fireEvent.click(screen.getByRole('button', {name: ADD_MAPPING}));
    fireEvent.change(screen.getByLabelText(CLAIM), {target: {value: 'groups'}});

    fireEvent.click(screen.getByRole('button', {name: 'Remove mapping'}));

    expect(screen.queryByLabelText(CLAIM)).not.toBeInTheDocument();
    expect(onDirectChange).toHaveBeenLastCalledWith(undefined, true);
  });

  it('explains what the toggle does regardless of its current state', () => {
    render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
    fireEvent.click(screen.getByRole('button', {name: ADD_MAPPING}));

    const helper = /each value is matched by name.*turn this on to compare values against explicit rules/i;
    expect(screen.getByText(helper)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('switch', {name: ADVANCED_TOGGLE}));

    expect(screen.getByText(helper)).toBeInTheDocument();
  });

  describe('advanced side', () => {
    function addAdvancedRow(): void {
      fireEvent.click(screen.getByRole('button', {name: ADD_MAPPING}));
      fireEvent.click(screen.getByRole('switch', {name: ADVANCED_TOGGLE}));
    }

    it("offers only the roles and groups of the connection's organization unit", () => {
      render(<AuthorizationMappingSection ouId="ou-1" onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
      addAdvancedRow();

      expect(useGetRolesForMapping).toHaveBeenCalledWith({limit: 100, ouId: 'ou-1'});
      expect(useGetGroupsForMapping).toHaveBeenCalledWith({limit: 100, ouId: 'ou-1'});
    });

    it('shows the Delimiter field only for value type String, hiding it for Number, Boolean, and Array', () => {
      render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
      addAdvancedRow();

      expect(screen.getByLabelText(DELIMITER)).toBeInTheDocument();

      selectOption(VALUE_TYPE, 'Number');
      expect(screen.queryByLabelText(DELIMITER)).not.toBeInTheDocument();

      selectOption(VALUE_TYPE, 'Boolean');
      expect(screen.queryByLabelText(DELIMITER)).not.toBeInTheDocument();

      selectOption(VALUE_TYPE, 'Array');
      expect(screen.queryByLabelText(DELIMITER)).not.toBeInTheDocument();

      selectOption(VALUE_TYPE, 'String');
      expect(screen.getByLabelText(DELIMITER)).toBeInTheDocument();
    });

    it('narrows the operator list to Includes/Not includes for value type Array', () => {
      render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
      addAdvancedRow();
      selectOption(VALUE_TYPE, 'Array');

      fireEvent.mouseDown(screen.getByRole('combobox', {name: OPERATOR}));
      expect(screen.getByRole('option', {name: 'Includes'})).toBeInTheDocument();
      expect(screen.getByRole('option', {name: 'Not includes'})).toBeInTheDocument();
      expect(screen.queryByRole('option', {name: 'Equals'})).not.toBeInTheDocument();
      expect(screen.queryByRole('option', {name: 'Greater than'})).not.toBeInTheDocument();
    });

    it('resets a rule left on Equals to Includes when switching Value Type to Array, and back to Equals when switching away', () => {
      render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
      addAdvancedRow();
      expect(operatorText()).toBe('Equals');

      selectOption(VALUE_TYPE, 'Array');
      expect(operatorText()).toBe('Includes');

      selectOption(VALUE_TYPE, 'String');
      expect(operatorText()).toBe('Equals');
    });

    it('resets a rule to Includes when a delimiter is typed into a String mapping, and back to Equals once cleared', () => {
      render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
      addAdvancedRow();
      expect(operatorText()).toBe('Equals');

      selectOption(DELIMITER, COMMA_PRESET);
      expect(operatorText()).toBe('Includes');

      selectOption(DELIMITER, NO_DELIMITER_PRESET);
      expect(operatorText()).toBe('Equals');
    });

    it('uses a plain text field, not the True/False picker, for the Array value type', () => {
      render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
      addAdvancedRow();
      selectOption(VALUE_TYPE, 'Array');

      const valueField = screen.getByLabelText(EXTERNAL_VALUE);
      expect(valueField.tagName).toBe('INPUT');
      fireEvent.change(valueField, {target: {value: 'engineering'}});
      expect((valueField as HTMLInputElement).value).toBe('engineering');
    });

    it('reports invalid once a rule has a value but no role, group, or permission selected, the same condition that paints its error styling', () => {
      render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
      addAdvancedRow();

      fireEvent.change(screen.getByLabelText(EXTERNAL_VALUE), {target: {value: 'engineering'}});

      expect(onRuleChange).toHaveBeenLastCalledWith(undefined, false);
      expect(onDirectChange).toHaveBeenLastCalledWith(undefined, false);
    });

    it('selects more than one role as chips within the same Roles field, reporting them all as separate role targets', () => {
      render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
      addAdvancedRow();
      fireEvent.change(screen.getByLabelText(CLAIM), {target: {value: 'groups'}});
      fireEvent.change(screen.getByLabelText(EXTERNAL_VALUE), {target: {value: 'engineering'}});

      fireEvent.mouseDown(screen.getByRole('combobox', {name: 'Role'}));
      fireEvent.click(screen.getByRole('option', {name: 'Admins'}));
      fireEvent.mouseDown(screen.getByRole('combobox', {name: 'Role'}));
      fireEvent.click(screen.getByRole('option', {name: 'Editors'}));

      expect(screen.getByText('Admins')).toBeInTheDocument();
      expect(screen.getByText('Editors')).toBeInTheDocument();
      expect(onRuleChange).toHaveBeenLastCalledWith(
        [
          {
            claim: 'groups',
            values: [
              {
                operator: 'equals',
                value: 'engineering',
                targets: [
                  {type: 'role', id: 'role-1'},
                  {type: 'role', id: 'role-2'},
                ],
              },
            ],
          },
        ],
        true,
      );
    });

    it('preserves a stored role id that is not in the fetched page across an unrelated edit', () => {
      render(
        <AuthorizationMappingSection
          onRuleChange={onRuleChange}
          onDirectChange={onDirectChange}
          initialRuleConfig={[
            {
              claim: 'groups',
              values: [{operator: 'equals', value: 'engineering', targets: [{type: 'role', id: 'role-999'}]}],
            },
          ]}
        />,
      );

      // An unrelated edit re-emits the whole mapping; role-999 must survive even though it never
      // appeared among the fetched options.
      fireEvent.change(screen.getByLabelText(EXTERNAL_VALUE), {target: {value: 'engineering-team'}});

      expect(onRuleChange).toHaveBeenLastCalledWith(
        [
          {
            claim: 'groups',
            values: [{operator: 'equals', value: 'engineering-team', targets: [{type: 'role', id: 'role-999'}]}],
          },
        ],
        true,
      );
    });

    it("marks a stored role missing from the organization unit's complete list as unavailable", () => {
      render(
        <AuthorizationMappingSection
          onRuleChange={onRuleChange}
          onDirectChange={onDirectChange}
          initialRuleConfig={[
            {
              claim: 'groups',
              values: [{operator: 'equals', value: 'engineering', targets: [{type: 'role', id: 'role-999'}]}],
            },
          ]}
        />,
      );

      expect(screen.getByText('Unavailable: role-999')).toBeInTheDocument();
    });

    it('shows a stored role missing from a partial list by its id, without marking it unavailable', () => {
      useGetRolesForMapping.mockReturnValue({data: {...ROLES_PAGE.data, totalResults: 250}});
      render(
        <AuthorizationMappingSection
          onRuleChange={onRuleChange}
          onDirectChange={onDirectChange}
          initialRuleConfig={[
            {
              claim: 'groups',
              values: [{operator: 'equals', value: 'engineering', targets: [{type: 'role', id: 'role-999'}]}],
            },
          ]}
        />,
      );

      expect(screen.getByText('role-999')).toBeInTheDocument();
      expect(screen.queryByText('Unavailable: role-999')).not.toBeInTheDocument();
    });

    it('renders the permission hierarchy picker for the Permissions grant area, scoped to the unit', () => {
      render(<AuthorizationMappingSection ouId="ou-1" onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
      addAdvancedRow();

      expect(screen.getByTestId('permission-catalog')).toHaveAttribute('data-ou-id', 'ou-1');
    });

    it('does not render the permission hierarchy picker until the unit is known', () => {
      render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
      addAdvancedRow();

      expect(screen.queryByTestId('permission-catalog')).not.toBeInTheDocument();
    });

    it("adds another value row defaulting to the mapping's current operator behavior", () => {
      render(<AuthorizationMappingSection onRuleChange={onRuleChange} onDirectChange={onDirectChange} />);
      addAdvancedRow();
      selectOption(VALUE_TYPE, 'Array');

      fireEvent.click(screen.getByRole('button', {name: ADD_VALUE}));
      const operatorSelects = screen.getAllByRole('combobox', {name: OPERATOR});
      expect(operatorSelects).toHaveLength(2);
      operatorSelects.forEach((select) => expect(select.textContent).toBe('Includes'));
    });
  });
});
