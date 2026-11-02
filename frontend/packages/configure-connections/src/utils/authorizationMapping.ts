// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ResourcePermissions} from '@thunderid/configure-resource-servers';
import type {
  AuthorizationDirectMapping,
  AuthorizationOperator,
  AuthorizationRuleMapping,
  AuthorizationTarget,
  AuthorizationTargetType,
  AuthorizationValueType,
} from '../models/connection';

/**
 * A single rule row within one claim's mapping. Grants are three fixed sets rather than a dynamic
 * list of rows: every role/group id and resource server's permissions the rule confers when it
 * matches, each editable as a multi-select.
 */
export interface KeyedAuthorizationValue {
  key: number;
  operator: AuthorizationOperator;
  value: string;
  roleIds: string[];
  groupIds: string[];
  permissions: ResourcePermissions[];
}

/**
 * One claim's authorization mapping, as edited in the form. A single row covers both mechanisms the
 * backend supports for that claim: by default (`isAdvanced` false) it feeds each of the claim's
 * values directly onto local roles, groups, or permissions of `targetType`, matched by name. Toggling
 * `isAdvanced` on switches the same row to an explicit value-to-target rule table (`valueType`,
 * `values`) instead. Both sets of fields are always present so toggling never loses data already
 * entered on the side that's currently hidden; only the active side is sent to the backend.
 */
export interface KeyedAuthorizationMapping {
  key: number;
  claim: string;
  delimiter: string;
  isAdvanced: boolean;
  targetType: AuthorizationTargetType;
  resourceServerId: string;
  valueType: AuthorizationValueType;
  values: KeyedAuthorizationValue[];
}

const DEFAULT_VALUE_TYPE: AuthorizationValueType = 'string';
const DEFAULT_OPERATOR: AuthorizationOperator = 'equals';
const DEFAULT_TARGET_TYPE: AuthorizationTargetType = 'role';

const MULTI_VALUED_OPERATORS: AuthorizationOperator[] = ['includes', 'not_includes'];
const EQUALITY_OPERATORS: AuthorizationOperator[] = ['equals', 'not_equals'];
/** Ordering comparisons only make sense for a declared numeric value; the backend rejects them for
 * any other value type at config-validation time, so the UI only ever offers them for `number`. */
const ORDERING_OPERATORS: AuthorizationOperator[] = [
  'greater_than',
  'less_than',
  'greater_than_or_equal',
  'less_than_or_equal',
];

/** Whether a mapping with this `valueType`/`delimiter` resolves the claim to a set of values rather
 * than a single value, mirroring the backend's `AuthorizationRuleMapping.IsMultiValued()`: true for
 * `array`, or `string` with a non-empty delimiter; false otherwise. */
export function isMultiValued(valueType: AuthorizationValueType, delimiter: string): boolean {
  return valueType === 'array' || (valueType === 'string' && delimiter !== '');
}

/** The operators valid for a mapping with this `valueType`/`delimiter`, matching the backend's
 * validation exactly: a multi-valued claim only accepts `includes`/`not_includes`; a single-valued
 * claim accepts `equals`/`not_equals`, plus the ordering operators when `valueType` is `number`. */
export function operatorsFor(valueType: AuthorizationValueType, delimiter: string): AuthorizationOperator[] {
  if (isMultiValued(valueType, delimiter)) {
    return MULTI_VALUED_OPERATORS;
  }
  return valueType === 'number' ? [...EQUALITY_OPERATORS, ...ORDERING_OPERATORS] : EQUALITY_OPERATORS;
}

/** The operator a rule should fall back to once its current operator is no longer valid for a
 * mapping's `valueType`/`delimiter`: `includes` for a multi-valued claim, `equals` otherwise. */
export function defaultOperatorFor(valueType: AuthorizationValueType, delimiter: string): AuthorizationOperator {
  return isMultiValued(valueType, delimiter) ? 'includes' : DEFAULT_OPERATOR;
}

/** A rule row grants something once at least one role, group, or permission is selected. */
export function ruleHasGrants(entry: KeyedAuthorizationValue): boolean {
  return (
    entry.roleIds.length > 0 || entry.groupIds.length > 0 || entry.permissions.some((rp) => rp.permissions.length > 0)
  );
}

/** A rule row has been touched once it has an external value typed or something granted. Used to
 * tell "never started" (fine to drop silently) apart from "started but not finished." */
export function ruleRowAttempted(entry: KeyedAuthorizationValue): boolean {
  return entry.value.trim() !== '' || ruleHasGrants(entry);
}

/** A rule row is only complete once its owning claim is named too: a value and grants with no claim
 * would otherwise be dropped silently on save (the whole mapping is skipped when the claim is blank),
 * with nothing telling the admin their grants never got saved. */
export function ruleRowIsComplete(claim: string, entry: KeyedAuthorizationValue): boolean {
  return claim.trim() !== '' && entry.value.trim() !== '' && ruleHasGrants(entry);
}

/** Flattens a rule row's three grant sets into the wire `AuthorizationTarget[]` shape. */
function targetsFor(entry: KeyedAuthorizationValue): AuthorizationTarget[] {
  return [
    ...entry.roleIds.map((id): AuthorizationTarget => ({type: 'role', id})),
    ...entry.groupIds.map((id): AuthorizationTarget => ({type: 'group', id})),
    ...entry.permissions.flatMap((rp) =>
      rp.permissions.map(
        (permission): AuthorizationTarget => ({
          type: 'permission',
          resourceServerId: rp.resourceServerId,
          permission,
        }),
      ),
    ),
  ];
}

/** Unflattens the wire `AuthorizationTarget[]` shape back into the three grant sets a rule edits. */
function splitTargets(
  targets: AuthorizationTarget[],
): Pick<KeyedAuthorizationValue, 'roleIds' | 'groupIds' | 'permissions'> {
  const roleIds: string[] = [];
  const groupIds: string[] = [];
  const permissionsByServer = new Map<string, string[]>();

  for (const target of targets) {
    if (target.type === 'role' && target.id) {
      roleIds.push(target.id);
    } else if (target.type === 'group' && target.id) {
      groupIds.push(target.id);
    } else if (target.type === 'permission' && target.resourceServerId && target.permission) {
      const existing = permissionsByServer.get(target.resourceServerId) ?? [];
      existing.push(target.permission);
      permissionsByServer.set(target.resourceServerId, existing);
    }
  }

  const permissions: ResourcePermissions[] = Array.from(permissionsByServer.entries()).map(
    ([resourceServerId, perms]) => ({resourceServerId, permissions: perms}),
  );

  return {roleIds, groupIds, permissions};
}

function directHalfIsComplete(mapping: KeyedAuthorizationMapping): boolean {
  return mapping.targetType !== 'permission' || mapping.resourceServerId.trim() !== '';
}

function directHalfIsAttempted(mapping: KeyedAuthorizationMapping): boolean {
  return mapping.targetType !== DEFAULT_TARGET_TYPE || mapping.resourceServerId.trim() !== '';
}

function advancedHalfIsComplete(mapping: KeyedAuthorizationMapping): boolean {
  const hasCompleteRow = mapping.values.some((v) => ruleRowIsComplete(mapping.claim, v));
  const hasIncompleteAttemptedRow = mapping.values.some(
    (v) => ruleRowAttempted(v) && !ruleRowIsComplete(mapping.claim, v),
  );
  return hasCompleteRow && !hasIncompleteAttemptedRow;
}

function advancedHalfIsAttempted(mapping: KeyedAuthorizationMapping): boolean {
  return mapping.values.some(ruleRowAttempted);
}

/** A mapping row has been touched once it differs from a freshly-added row in any way, checking
 * whichever side (`isAdvanced` or not) is currently active — leftover data on the hidden side
 * shouldn't force an "incomplete" error for fields the admin isn't using. Used to tell "never
 * started" (fine to drop silently) apart from "started but not finished" (must block save). */
export function mappingIsAttempted(mapping: KeyedAuthorizationMapping): boolean {
  return (
    mapping.claim.trim() !== '' ||
    mapping.delimiter !== '' ||
    (mapping.isAdvanced ? advancedHalfIsAttempted(mapping) : directHalfIsAttempted(mapping))
  );
}

/** A mapping row is complete once it names a claim, and — on whichever side is active — a resource
 * server when the target type is "permission" (direct), or at least one complete rule with no other
 * rule left attempted-but-incomplete (advanced): a sibling row that's still missing its value or its
 * grants must not be silently dropped just because another row in the same mapping is done. */
export function mappingIsComplete(mapping: KeyedAuthorizationMapping): boolean {
  if (mapping.claim.trim() === '') {
    return false;
  }
  return mapping.isAdvanced ? advancedHalfIsComplete(mapping) : directHalfIsComplete(mapping);
}

/** Build the API `authorizationMapping.rules`/`.direct` from the form state, split by each row's
 * active side. Incomplete rows are dropped. */
export function toAuthorizationMappings(mappings: KeyedAuthorizationMapping[]): {
  rules: AuthorizationRuleMapping[] | undefined;
  direct: AuthorizationDirectMapping[] | undefined;
} {
  const rules: AuthorizationRuleMapping[] = [];
  const direct: AuthorizationDirectMapping[] = [];

  for (const mapping of mappings) {
    const claim = mapping.claim.trim();
    if (claim === '') {
      continue;
    }

    if (mapping.isAdvanced) {
      const values: AuthorizationRuleMapping['values'] = [];
      for (const entry of mapping.values) {
        const value = entry.value.trim();
        const targets = targetsFor(entry);
        if (value !== '' && targets.length > 0) {
          values.push({operator: entry.operator, value, targets});
        }
      }
      if (values.length > 0) {
        rules.push({
          claim,
          ...(mapping.valueType !== DEFAULT_VALUE_TYPE ? {valueType: mapping.valueType} : {}),
          // The delimiter is meaningful whitespace, such as a single space for a scope string, so it
          // is never trimmed the way the claim and value are.
          ...(mapping.delimiter !== '' ? {delimiter: mapping.delimiter} : {}),
          values,
        });
      }
    } else if (directHalfIsComplete(mapping)) {
      direct.push({
        claim,
        ...(mapping.delimiter !== '' ? {delimiter: mapping.delimiter} : {}),
        targetType: mapping.targetType,
        ...(mapping.targetType === 'permission' ? {resourceServerId: mapping.resourceServerId.trim()} : {}),
      });
    }
  }

  return {rules: rules.length > 0 ? rules : undefined, direct: direct.length > 0 ? direct : undefined};
}

let keySeq = 0;
const nextKey = (): number => {
  keySeq += 1;
  return keySeq;
};

/** A fresh, empty rule row, for appending to an existing claim mapping. `operator` defaults to
 * `equals`; pass the owning mapping's `defaultOperatorFor(valueType, delimiter)` when appending to a
 * mapping that is already multi-valued, so the new row doesn't start on an operator its own mapping
 * no longer offers. */
export function newValueRow(operator: AuthorizationOperator = DEFAULT_OPERATOR): KeyedAuthorizationValue {
  return {key: nextKey(), operator, value: '', roleIds: [], groupIds: [], permissions: []};
}

/** A fresh, empty mapping row, for appending to the list. Defaults to the simple (direct) side; an
 * initial rule row is seeded in `values` too, ready if the admin toggles `isAdvanced` on. */
export function newAuthorizationMapping(): KeyedAuthorizationMapping {
  return {
    key: nextKey(),
    claim: '',
    delimiter: '',
    isAdvanced: false,
    targetType: DEFAULT_TARGET_TYPE,
    resourceServerId: '',
    valueType: DEFAULT_VALUE_TYPE,
    values: [newValueRow()],
  };
}

/** Convert the API shapes into unified form state, keying every row for stable React list rendering.
 * Direct mappings become simple rows; rule mappings become advanced rows. */
export function fromAuthorizationMappings(
  direct: AuthorizationDirectMapping[] | undefined,
  rules: AuthorizationRuleMapping[] | undefined,
): KeyedAuthorizationMapping[] {
  const directRows: KeyedAuthorizationMapping[] = (direct ?? []).map((mapping) => ({
    key: nextKey(),
    claim: mapping.claim,
    delimiter: mapping.delimiter ?? '',
    isAdvanced: false,
    targetType: mapping.targetType,
    resourceServerId: mapping.resourceServerId ?? '',
    valueType: DEFAULT_VALUE_TYPE,
    values: [newValueRow()],
  }));

  const ruleRows: KeyedAuthorizationMapping[] = (rules ?? []).map((mapping) => ({
    key: nextKey(),
    claim: mapping.claim,
    delimiter: mapping.delimiter ?? '',
    isAdvanced: true,
    targetType: DEFAULT_TARGET_TYPE,
    resourceServerId: '',
    valueType: mapping.valueType ?? DEFAULT_VALUE_TYPE,
    values: mapping.values.map((rule) => ({
      key: nextKey(),
      operator: rule.operator,
      value: rule.value,
      ...splitTargets(rule.targets),
    })),
  }));

  return [...directRows, ...ruleRows];
}
