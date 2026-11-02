// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {AuthorizationDirectMapping, AuthorizationRuleMapping} from '../../models/connection';
import {
  defaultOperatorFor,
  fromAuthorizationMappings,
  isMultiValued,
  type KeyedAuthorizationMapping,
  mappingIsAttempted,
  mappingIsComplete,
  newAuthorizationMapping,
  newValueRow,
  operatorsFor,
  ruleHasGrants,
  ruleRowAttempted,
  ruleRowIsComplete,
  toAuthorizationMappings,
} from '../authorizationMapping';

/** A fresh simple-side (direct) row, for tests to override piecemeal. */
function directRow(overrides: Partial<KeyedAuthorizationMapping> = {}): KeyedAuthorizationMapping {
  return {...newAuthorizationMapping(), ...overrides};
}

/** A fresh advanced-side (rule) row, for tests to override piecemeal. */
function advancedRow(overrides: Partial<KeyedAuthorizationMapping> = {}): KeyedAuthorizationMapping {
  return {...newAuthorizationMapping(), isAdvanced: true, ...overrides};
}

describe('toAuthorizationMappings', () => {
  it('returns undefined for both when the list is empty', () => {
    expect(toAuthorizationMappings([])).toEqual({rules: undefined, direct: undefined});
  });

  it('drops a row with a blank claim, on either side', () => {
    const rows = [
      directRow({claim: '  ', targetType: 'role'}),
      advancedRow({
        claim: '  ',
        values: [{key: 1, operator: 'equals', value: 'x', roleIds: ['role-1'], groupIds: [], permissions: []}],
      }),
    ];
    expect(toAuthorizationMappings(rows)).toEqual({rules: undefined, direct: undefined});
  });

  it('routes a simple-side row into direct, trimming the claim and including delimiter/resourceServerId only when set', () => {
    const rows = [directRow({claim: ' groups ', delimiter: ',', targetType: 'permission', resourceServerId: ' rs-1 '})];
    expect(toAuthorizationMappings(rows)).toEqual({
      rules: undefined,
      direct: [{claim: 'groups', delimiter: ',', targetType: 'permission', resourceServerId: 'rs-1'}],
    });
  });

  it('drops a simple-side permission row with no resource server chosen', () => {
    const rows = [directRow({claim: 'scope', targetType: 'permission', resourceServerId: ''})];
    expect(toAuthorizationMappings(rows)).toEqual({rules: undefined, direct: undefined});
  });

  it('omits resourceServerId for role/group target types even if one was left over from switching away from Permission', () => {
    const rows = [directRow({claim: 'groups', targetType: 'role', resourceServerId: 'rs-1'})];
    expect(toAuthorizationMappings(rows).direct).toEqual([{claim: 'groups', targetType: 'role'}]);
  });

  it('routes an advanced-side row into rules, dropping incomplete values but keeping the claim if another value has grants', () => {
    const rows = [
      advancedRow({
        claim: 'groups',
        values: [
          {key: 1, operator: 'equals', value: 'incomplete', roleIds: [], groupIds: [], permissions: []},
          {key: 2, operator: 'equals', value: 'engineering', roleIds: ['role-eng'], groupIds: [], permissions: []},
        ],
      }),
    ];
    expect(toAuthorizationMappings(rows)).toEqual({
      direct: undefined,
      rules: [
        {
          claim: 'groups',
          values: [{operator: 'equals', value: 'engineering', targets: [{type: 'role', id: 'role-eng'}]}],
        },
      ],
    });
  });

  it('flattens multiple roles, groups, and permissions for one advanced value into separate targets', () => {
    const rows = [
      advancedRow({
        claim: 'groups',
        values: [
          {
            key: 1,
            operator: 'equals',
            value: 'platform-admins',
            roleIds: ['role-admin', 'role-owner'],
            groupIds: ['group-platform'],
            permissions: [{resourceServerId: 'rs-1', permissions: ['read', 'write']}],
          },
        ],
      }),
    ];
    expect(toAuthorizationMappings(rows).rules).toEqual([
      {
        claim: 'groups',
        values: [
          {
            operator: 'equals',
            value: 'platform-admins',
            targets: [
              {type: 'role', id: 'role-admin'},
              {type: 'role', id: 'role-owner'},
              {type: 'group', id: 'group-platform'},
              {type: 'permission', resourceServerId: 'rs-1', permission: 'read'},
              {type: 'permission', resourceServerId: 'rs-1', permission: 'write'},
            ],
          },
        ],
      },
    ]);
  });

  it('omits valueType from the output when it is the default "string", and includes it otherwise', () => {
    const stringRow = advancedRow({
      claim: 'groups',
      values: [{key: 1, operator: 'equals', value: 'a', roleIds: ['role-1'], groupIds: [], permissions: []}],
    });
    const numberRow = advancedRow({
      claim: 'level',
      valueType: 'number',
      values: [{key: 1, operator: 'greater_than', value: '5', roleIds: ['role-1'], groupIds: [], permissions: []}],
    });
    expect(toAuthorizationMappings([stringRow]).rules?.[0]).not.toHaveProperty('valueType');
    expect(toAuthorizationMappings([numberRow]).rules?.[0]).toMatchObject({valueType: 'number'});
  });

  it('splits a mixed list of rows into both rules and direct', () => {
    const rows = [
      directRow({claim: 'perms', targetType: 'permission', resourceServerId: 'rs-1'}),
      advancedRow({
        claim: 'groups',
        values: [
          {key: 1, operator: 'equals', value: 'engineering', roleIds: ['role-eng'], groupIds: [], permissions: []},
        ],
      }),
    ];
    const {rules, direct} = toAuthorizationMappings(rows);
    expect(direct).toEqual([{claim: 'perms', targetType: 'permission', resourceServerId: 'rs-1'}]);
    expect(rules).toEqual([
      {
        claim: 'groups',
        values: [{operator: 'equals', value: 'engineering', targets: [{type: 'role', id: 'role-eng'}]}],
      },
    ]);
  });
});

describe('fromAuthorizationMappings', () => {
  it('returns an empty array when both are undefined', () => {
    expect(fromAuthorizationMappings(undefined, undefined)).toEqual([]);
  });

  it('converts a direct mapping into a simple-side row', () => {
    const direct: AuthorizationDirectMapping[] = [
      {claim: 'groups', targetType: 'permission', resourceServerId: 'rs-1'},
    ];
    const [row] = fromAuthorizationMappings(direct, undefined);
    expect(row.isAdvanced).toBe(false);
    expect(row.claim).toBe('groups');
    expect(row.targetType).toBe('permission');
    expect(row.resourceServerId).toBe('rs-1');
  });

  it('converts a rule mapping into an advanced-side row, defaulting valueType to "string" when omitted', () => {
    const rules: AuthorizationRuleMapping[] = [
      {claim: 'groups', values: [{operator: 'equals', value: 'a', targets: [{type: 'role', id: 'r1'}]}]},
    ];
    const [row] = fromAuthorizationMappings(undefined, rules);
    expect(row.isAdvanced).toBe(true);
    expect(row.valueType).toBe('string');
    expect(row.values[0].roleIds).toEqual(['r1']);
  });

  it('splits mixed advanced-row targets into roleIds, groupIds, and permissions grouped by resource server', () => {
    const rules: AuthorizationRuleMapping[] = [
      {
        claim: 'groups',
        values: [
          {
            operator: 'equals',
            value: 'platform-admins',
            targets: [
              {type: 'role', id: 'role-admin'},
              {type: 'group', id: 'group-platform'},
              {type: 'permission', resourceServerId: 'rs-1', permission: 'read'},
              {type: 'permission', resourceServerId: 'rs-1', permission: 'write'},
              {type: 'permission', resourceServerId: 'rs-2', permission: 'read'},
            ],
          },
        ],
      },
    ];
    const [row] = fromAuthorizationMappings(undefined, rules);
    expect(row.values[0].roleIds).toEqual(['role-admin']);
    expect(row.values[0].groupIds).toEqual(['group-platform']);
    expect(row.values[0].permissions).toEqual([
      {resourceServerId: 'rs-1', permissions: ['read', 'write']},
      {resourceServerId: 'rs-2', permissions: ['read']},
    ]);
  });

  it('renders direct rows before rule rows, one row per entry across both lists', () => {
    const direct: AuthorizationDirectMapping[] = [{claim: 'perms', targetType: 'permission', resourceServerId: 'rs-1'}];
    const rules: AuthorizationRuleMapping[] = [
      {claim: 'groups', values: [{operator: 'equals', value: 'a', targets: [{type: 'role', id: 'r1'}]}]},
    ];
    const rows = fromAuthorizationMappings(direct, rules);
    expect(rows).toHaveLength(2);
    expect(rows[0].isAdvanced).toBe(false);
    expect(rows[1].isAdvanced).toBe(true);
  });

  it('round-trips a direct mapping through toAuthorizationMappings unchanged', () => {
    const original: AuthorizationDirectMapping[] = [
      {claim: 'groups', delimiter: ',', targetType: 'permission', resourceServerId: 'rs-1'},
    ];
    const roundTripped = toAuthorizationMappings(fromAuthorizationMappings(original, undefined));
    expect(roundTripped.direct).toEqual(original);
    expect(roundTripped.rules).toBeUndefined();
  });

  it('round-trips a rule mapping through toAuthorizationMappings unchanged', () => {
    const original: AuthorizationRuleMapping[] = [
      {
        claim: 'groups',
        delimiter: ',',
        values: [
          {operator: 'equals', value: 'engineering', targets: [{type: 'role', id: 'role-eng'}]},
          {
            operator: 'equals',
            value: 'platform-admins',
            targets: [
              {type: 'role', id: 'role-admin'},
              {type: 'group', id: 'group-platform'},
            ],
          },
        ],
      },
    ];
    const roundTripped = toAuthorizationMappings(fromAuthorizationMappings(undefined, original));
    expect(roundTripped.rules).toEqual(original);
    expect(roundTripped.direct).toBeUndefined();
  });

  it('assigns distinct keys to every row and value for stable list rendering', () => {
    const direct: AuthorizationDirectMapping[] = [{claim: 'perms', targetType: 'role'}];
    const rules: AuthorizationRuleMapping[] = [
      {
        claim: 'groups',
        values: [
          {operator: 'equals', value: 'a', targets: [{type: 'role', id: 'r1'}]},
          {operator: 'equals', value: 'b', targets: [{type: 'role', id: 'r2'}]},
        ],
      },
    ];
    const rows = fromAuthorizationMappings(direct, rules);
    const allKeys = [...rows.map((r) => r.key), ...rows.flatMap((r) => r.values.map((v) => v.key))];
    expect(new Set(allKeys).size).toBe(allKeys.length);
  });
});

describe('newAuthorizationMapping / newValueRow defaults', () => {
  it('defaults a new row to the simple (non-advanced) side, target type "role"', () => {
    const row = newAuthorizationMapping();
    expect(row.isAdvanced).toBe(false);
    expect(row.targetType).toBe('role');
    expect(row.resourceServerId).toBe('');
  });

  it('still seeds a single empty value row, ready if the admin toggles to advanced', () => {
    const row = newAuthorizationMapping();
    expect(row.valueType).toBe('string');
    expect(row.values).toHaveLength(1);
    expect(row.values[0].operator).toBe('equals');
  });

  it('defaults a new value row to operator "equals" with no grants', () => {
    const row = newValueRow();
    expect(row.operator).toBe('equals');
    expect(row.roleIds).toEqual([]);
    expect(row.groupIds).toEqual([]);
    expect(row.permissions).toEqual([]);
  });

  it('accepts an explicit operator override, for appending to an already multi-valued mapping', () => {
    expect(newValueRow('includes').operator).toBe('includes');
  });
});

describe('mappingIsAttempted', () => {
  it('is false for a freshly-added row', () => {
    expect(mappingIsAttempted(newAuthorizationMapping())).toBe(false);
  });

  it('is true once a claim is typed', () => {
    expect(mappingIsAttempted(directRow({claim: 'groups'}))).toBe(true);
  });

  it('is true on the simple side once the target type changes from the default, or a resource server is chosen', () => {
    expect(mappingIsAttempted(directRow({targetType: 'group'}))).toBe(true);
    expect(mappingIsAttempted(directRow({resourceServerId: 'rs-1'}))).toBe(true);
  });

  it('is true on the advanced side once any value row is attempted, ignoring untouched simple-side leftovers', () => {
    const row = advancedRow({
      values: [{key: 1, operator: 'equals', value: 'engineering', roleIds: [], groupIds: [], permissions: []}],
    });
    expect(mappingIsAttempted(row)).toBe(true);
  });

  it('ignores an untouched advanced side when the row is currently on the simple side', () => {
    const row = directRow({
      values: [{key: 1, operator: 'equals', value: '', roleIds: [], groupIds: [], permissions: []}],
    });
    expect(mappingIsAttempted(row)).toBe(false);
  });
});

describe('mappingIsComplete', () => {
  it('is false when the claim is blank, regardless of side', () => {
    expect(mappingIsComplete(directRow({claim: ''}))).toBe(false);
    expect(mappingIsComplete(advancedRow({claim: '  '}))).toBe(false);
  });

  it('on the simple side, is true for role/group with no resource server, false for permission without one', () => {
    expect(mappingIsComplete(directRow({claim: 'groups', targetType: 'role'}))).toBe(true);
    expect(mappingIsComplete(directRow({claim: 'scope', targetType: 'permission', resourceServerId: ''}))).toBe(false);
    expect(mappingIsComplete(directRow({claim: 'scope', targetType: 'permission', resourceServerId: 'rs-1'}))).toBe(
      true,
    );
  });

  it('on the advanced side, is true once at least one value row is complete', () => {
    const complete = advancedRow({
      claim: 'groups',
      values: [{key: 1, operator: 'equals', value: 'engineering', roleIds: ['role-1'], groupIds: [], permissions: []}],
    });
    const incomplete = advancedRow({
      claim: 'groups',
      values: [{key: 1, operator: 'equals', value: 'engineering', roleIds: [], groupIds: [], permissions: []}],
    });
    expect(mappingIsComplete(complete)).toBe(true);
    expect(mappingIsComplete(incomplete)).toBe(false);
  });

  it('on the advanced side, is false when a second value row is attempted but incomplete, even if the first is done', () => {
    const mixed = advancedRow({
      claim: 'groups',
      values: [
        {key: 1, operator: 'equals', value: 'engineering', roleIds: ['role-1'], groupIds: [], permissions: []},
        {key: 2, operator: 'equals', value: 'platform', roleIds: [], groupIds: [], permissions: []},
      ],
    });
    expect(mappingIsComplete(mixed)).toBe(false);
  });

  it('on the advanced side, ignores an untouched extra value row alongside a complete one', () => {
    const withSpareRow = advancedRow({
      claim: 'groups',
      values: [
        {key: 1, operator: 'equals', value: 'engineering', roleIds: ['role-1'], groupIds: [], permissions: []},
        {key: 2, operator: 'equals', value: '', roleIds: [], groupIds: [], permissions: []},
      ],
    });
    expect(mappingIsComplete(withSpareRow)).toBe(true);
  });
});

describe('isMultiValued', () => {
  it('is true for valueType "array" regardless of delimiter', () => {
    expect(isMultiValued('array', '')).toBe(true);
    expect(isMultiValued('array', ',')).toBe(true);
  });

  it('is true for valueType "string" only when a delimiter is set', () => {
    expect(isMultiValued('string', '')).toBe(false);
    expect(isMultiValued('string', ',')).toBe(true);
  });

  it('is false for "number" and "boolean" regardless of delimiter', () => {
    expect(isMultiValued('number', '')).toBe(false);
    expect(isMultiValued('number', ',')).toBe(false);
    expect(isMultiValued('boolean', '')).toBe(false);
    expect(isMultiValued('boolean', ',')).toBe(false);
  });
});

describe('operatorsFor', () => {
  it('narrows to includes/not_includes for valueType "array"', () => {
    expect(operatorsFor('array', '')).toEqual(['includes', 'not_includes']);
  });

  it('narrows to includes/not_includes for a "string" mapping with a delimiter set', () => {
    expect(operatorsFor('string', ',')).toEqual(['includes', 'not_includes']);
  });

  it('offers equals/not_equals for a "string" mapping with no delimiter', () => {
    expect(operatorsFor('string', '')).toEqual(['equals', 'not_equals']);
  });

  it('offers equality and ordering operators for "number", regardless of delimiter', () => {
    const expected = [
      'equals',
      'not_equals',
      'greater_than',
      'less_than',
      'greater_than_or_equal',
      'less_than_or_equal',
    ];
    expect(operatorsFor('number', '')).toEqual(expected);
    expect(operatorsFor('number', ',')).toEqual(expected);
  });

  it('offers only equals/not_equals for "boolean", regardless of delimiter', () => {
    expect(operatorsFor('boolean', '')).toEqual(['equals', 'not_equals']);
    expect(operatorsFor('boolean', ',')).toEqual(['equals', 'not_equals']);
  });
});

describe('defaultOperatorFor', () => {
  it('is "includes" once the mapping is multi-valued', () => {
    expect(defaultOperatorFor('array', '')).toBe('includes');
    expect(defaultOperatorFor('string', ',')).toBe('includes');
  });

  it('is "equals" when the mapping is single-valued', () => {
    expect(defaultOperatorFor('string', '')).toBe('equals');
    expect(defaultOperatorFor('number', '')).toBe('equals');
    expect(defaultOperatorFor('boolean', '')).toBe('equals');
  });
});

describe('ruleHasGrants', () => {
  const base = {key: 1, operator: 'equals' as const, value: 'x', roleIds: [], groupIds: [], permissions: []};

  it('is false when nothing is selected', () => {
    expect(ruleHasGrants(base)).toBe(false);
  });

  it('is true when at least one role is selected', () => {
    expect(ruleHasGrants({...base, roleIds: ['role-1']})).toBe(true);
  });

  it('is true when at least one group is selected', () => {
    expect(ruleHasGrants({...base, groupIds: ['group-1']})).toBe(true);
  });

  it('is true when at least one permission is selected', () => {
    expect(ruleHasGrants({...base, permissions: [{resourceServerId: 'rs-1', permissions: ['read']}]})).toBe(true);
  });

  it('is false when a permission entry exists for a resource server but selects nothing', () => {
    expect(ruleHasGrants({...base, permissions: [{resourceServerId: 'rs-1', permissions: []}]})).toBe(false);
  });
});

describe('ruleRowAttempted', () => {
  const base = {key: 1, operator: 'equals' as const, value: '', roleIds: [], groupIds: [], permissions: []};

  it('is false for a fresh row with no value and no grants', () => {
    expect(ruleRowAttempted(base)).toBe(false);
  });

  it('is true once a value is typed, even with no grants', () => {
    expect(ruleRowAttempted({...base, value: 'engineering'})).toBe(true);
  });

  it('is true once something is granted, even with no value', () => {
    expect(ruleRowAttempted({...base, roleIds: ['role-1']})).toBe(true);
  });
});

describe('ruleRowIsComplete', () => {
  const grantedRow = {
    key: 1,
    operator: 'equals' as const,
    value: 'engineering',
    roleIds: ['role-1'],
    groupIds: [],
    permissions: [],
  };

  it('is true once the claim, value, and a grant are all present', () => {
    expect(ruleRowIsComplete('groups', grantedRow)).toBe(true);
  });

  it('is false when the claim is blank, even with a value and grants selected', () => {
    expect(ruleRowIsComplete('', grantedRow)).toBe(false);
    expect(ruleRowIsComplete('   ', grantedRow)).toBe(false);
  });

  it('is false when the value is blank', () => {
    expect(ruleRowIsComplete('groups', {...grantedRow, value: ''})).toBe(false);
  });

  it('is false when there are no grants', () => {
    expect(ruleRowIsComplete('groups', {...grantedRow, roleIds: []})).toBe(false);
  });
});
