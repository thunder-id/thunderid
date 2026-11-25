// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  allowedValuesOf,
  DEFAULT_MAX_LENGTH,
  effectiveModification,
  modificationsFor,
  modifyingParties,
  newField,
  PARTIES,
  sampleData,
  setOwnProperty,
  supportsLimits,
  valueTypeFor,
} from './authorization-detail-schema';
import type {
  FieldKind,
  Modification,
  ModifyingParty,
  SchemaField,
  ValueType,
} from '../models/authorization-detail-type';

/** The drag type of a field of the consent card. */
export const FIELD_DRAG_TYPE = 'field';

/** The kinds of field the composer offers to add, including presets built from several fields. */
export type PaletteItem = 'string' | 'number' | 'date' | 'dateTime' | 'boolean' | 'array' | 'object' | 'money';

/** The kinds of field to add, in the order the composer lists them. */
export const PALETTE_ITEMS: PaletteItem[] = [
  'string',
  'number',
  'date',
  'dateTime',
  'boolean',
  'array',
  'object',
  'money',
];

/** Where a field sits: the object it belongs to (null at the top level) and its position there. */
export interface FieldPosition {
  parentKey: string | null;
  index: number;
}

/** A field with where it sits among the fields, as the overview lists it. */
export interface FlatField extends FieldPosition {
  field: SchemaField;
  depth: number;
  /** The dotted path of the field's name. */
  path: string;
  /** Whether the processor supplies this field, itself or through an object it belongs to. */
  processorSupplied: boolean;
}

/** Returns a new field of a kind, with the defaults the composer starts it with. */
export function createPaletteField(item: PaletteItem): SchemaField {
  if (item === 'money') {
    return {
      ...newField('object'),
      children: [
        {...newField('number'), name: 'amount', title: 'Amount', required: true},
        {...newField('string'), name: 'currency', title: 'Currency', required: true, maxLength: '3'},
      ],
    };
  }
  const field = newField(item);
  return item === 'string' ? {...field, maxLength: DEFAULT_MAX_LENGTH} : field;
}

/** Lists every field depth first, with where it sits. */
export function flattenFields(fields: SchemaField[]): FlatField[] {
  const out: FlatField[] = [];
  const walk = (list: SchemaField[], parentKey: string | null, depth: number, prefix: string, inherited: boolean) => {
    list.forEach((field, index) => {
      const name = field.name.trim();
      const path = prefix ? `${prefix}.${name}` : name;
      const processorSupplied = inherited || field.policy.suppliedBy === 'processor';
      out.push({field, parentKey, index, depth, path, processorSupplied});
      if (field.kind === 'object') {
        walk(field.children, field.key, depth + 1, path, processorSupplied);
      }
    });
  };
  walk(fields, null, 0, '', false);
  return out;
}

/** Returns the field with the given key, wherever it is. */
export function findField(fields: SchemaField[], key: string): SchemaField | undefined {
  for (const field of fields) {
    if (field.key === key) return field;
    const nested = findField(field.children, key);
    if (nested) return nested;
  }
  return undefined;
}

/** Returns where the field with the given key sits, or undefined when there is no such field. */
export function findPosition(
  fields: SchemaField[],
  key: string,
  parentKey: string | null = null,
): FieldPosition | undefined {
  for (let index = 0; index < fields.length; index += 1) {
    if (fields[index].key === key) return {parentKey, index};
    const nested = findPosition(fields[index].children, key, fields[index].key);
    if (nested) return nested;
  }
  return undefined;
}

/** Returns the fields of the object with the given key, or the top-level fields for null. */
function siblingsOf(fields: SchemaField[], parentKey: string | null): SchemaField[] | undefined {
  if (parentKey === null) return fields;
  const parent = findField(fields, parentKey);
  return parent?.kind === 'object' ? parent.children : undefined;
}

/** Replaces the fields of the object with the given key, or the top-level fields for null. */
function withSiblings(
  fields: SchemaField[],
  parentKey: string | null,
  change: (siblings: SchemaField[]) => SchemaField[],
): SchemaField[] {
  if (parentKey === null) return change(fields);
  return fields.map((field) => {
    if (field.key === parentKey) return {...field, children: change(field.children)};
    return field.children.length ? {...field, children: withSiblings(field.children, parentKey, change)} : field;
  });
}

/**
 * Applies a change to the field with the given key, wherever it is. The fields that do not hold it are
 * kept as they are, so the card redraws only the changed one.
 */
export function updateField(
  fields: SchemaField[],
  key: string,
  change: (field: SchemaField) => SchemaField,
): SchemaField[] {
  let changed = false;
  const next = fields.map((field) => {
    if (field.key === key) {
      changed = true;
      return change(field);
    }
    const children = field.children.length ? updateField(field.children, key, change) : field.children;
    if (children === field.children) return field;
    changed = true;
    return {...field, children};
  });
  return changed ? next : fields;
}

/** Removes the field with the given key, and the fields inside it. */
export function removeField(fields: SchemaField[], key: string): SchemaField[] {
  return fields
    .filter((field) => field.key !== key)
    .map((field) => (field.children.length ? {...field, children: removeField(field.children, key)} : field));
}

/**
 * Inserts a field into the object with the given key, or at the top level for null, before the field
 * at the index. Leaves the fields as they are when there is no such object.
 */
export function insertField(
  fields: SchemaField[],
  field: SchemaField,
  parentKey: string | null,
  index: number,
): SchemaField[] {
  if (!siblingsOf(fields, parentKey)) return fields;
  return withSiblings(fields, parentKey, (siblings) => {
    const at = Math.max(0, Math.min(index, siblings.length));
    return [...siblings.slice(0, at), field, ...siblings.slice(at)];
  });
}

/**
 * Moves a field into the object with the given key, or to the top level for null, before the field
 * at the index as the fields are before the move. Leaves the fields as they are when the move would
 * put a field inside itself, or there is no such field or object.
 */
export function moveField(fields: SchemaField[], key: string, parentKey: string | null, index: number): SchemaField[] {
  const field = findField(fields, key);
  const from = findPosition(fields, key);
  if (!field || !from) return fields;
  if (parentKey !== null && (parentKey === key || findField(field.children, parentKey))) return fields;
  if (!siblingsOf(fields, parentKey)) return fields;
  const at = from.parentKey === parentKey && from.index < index ? index - 1 : index;
  if (from.parentKey === parentKey && from.index === at) return fields;
  return insertField(removeField(fields, key), field, parentKey, at);
}

/** Copies a field and the fields inside it, each with a new key. */
function cloneField(field: SchemaField): SchemaField {
  return {
    ...field,
    key: newField().key,
    policy: {...field.policy, ...(field.policy.modifiableBy && {modifiableBy: [...field.policy.modifiableBy]})},
    ...(field.allowedValues && {allowedValues: [...field.allowedValues]}),
    children: field.children.map(cloneField),
  };
}

/** Returns a name, based on the given one, that none of the siblings has. */
function copyName(name: string, siblings: SchemaField[]): string {
  if (!name.trim()) return name;
  const taken = new Set(siblings.map((sibling) => sibling.name.trim()));
  let candidate = `${name.trim()}_copy`;
  for (let n = 2; taken.has(candidate); n += 1) candidate = `${name.trim()}_copy${n}`;
  return candidate;
}

/** Copies the field with the given key right after it, under a name its siblings do not have. */
export function duplicateField(fields: SchemaField[], key: string): {fields: SchemaField[]; key: string} | undefined {
  const field = findField(fields, key);
  const position = findPosition(fields, key);
  if (!field || !position) return undefined;
  const siblings = siblingsOf(fields, position.parentKey) ?? [];
  const copy = {...cloneField(field), name: copyName(field.name, siblings)};
  return {fields: insertField(fields, copy, position.parentKey, position.index + 1), key: copy.key};
}

/** The moves the composer offers for a field without dragging it. */
export type FieldMove = 'up' | 'down' | 'indent' | 'outdent';

/** Whether a field can be moved each way, without dragging it. */
export type MoveAbility = Record<FieldMove, boolean>;

/**
 * Returns the moves each field can make without dragging, keyed by the field, in one pass, as
 * moveTarget finds them one at a time.
 */
export function moveAbilities(fields: SchemaField[]): Map<string, MoveAbility> {
  const out = new Map<string, MoveAbility>();
  const walk = (siblings: SchemaField[], nested: boolean): void => {
    siblings.forEach((field, index) => {
      out.set(field.key, {
        up: index > 0,
        down: index < siblings.length - 1,
        indent: siblings[index - 1]?.kind === 'object',
        outdent: nested,
      });
      walk(field.children, true);
    });
  };
  walk(fields, false);
  return out;
}

/** Returns where a move would put a field, or undefined when the field cannot move that way. */
export function moveTarget(fields: SchemaField[], key: string, move: FieldMove): FieldPosition | undefined {
  const position = findPosition(fields, key);
  if (!position) return undefined;
  const siblings = siblingsOf(fields, position.parentKey) ?? [];
  switch (move) {
    case 'up':
      return position.index > 0 ? {parentKey: position.parentKey, index: position.index - 1} : undefined;
    case 'down':
      return position.index < siblings.length - 1
        ? {parentKey: position.parentKey, index: position.index + 2}
        : undefined;
    case 'indent': {
      const previous = siblings[position.index - 1];
      return previous?.kind === 'object' ? {parentKey: previous.key, index: previous.children.length} : undefined;
    }
    default: {
      if (position.parentKey === null) return undefined;
      const parent = findPosition(fields, position.parentKey);
      return parent ? {parentKey: parent.parentKey, index: parent.index + 1} : undefined;
    }
  }
}

/** Moves a field up or down among its siblings, into the object above it, or out of its object. */
export function applyMove(fields: SchemaField[], key: string, move: FieldMove): SchemaField[] {
  const target = moveTarget(fields, key, move);
  return target ? moveField(fields, key, target.parentKey, target.index) : fields;
}

/** What is being dragged: a field of the consent card. */
export interface DragSource {
  key: string;
}

/**
 * Applies a drop: moves the dragged field to the given position. Returns the new fields, or undefined
 * when the drop changes nothing.
 */
export function applyDrop(fields: SchemaField[], source: DragSource, target: FieldPosition): SchemaField[] | undefined {
  const next = moveField(fields, source.key, target.parentKey, target.index);
  return next === fields ? undefined : next;
}

/** Changes a field's kind, dropping what the new kind cannot carry. */
export function changeKind(field: SchemaField, kind: FieldKind): SchemaField {
  const next: SchemaField = {
    ...field,
    kind,
    children: kind === 'object' ? field.children : [],
    maxLength: kind === 'string' ? field.maxLength : undefined,
    allowedValues: kind === 'array' ? field.allowedValues : undefined,
    minItems: kind === 'array' ? field.minItems : undefined,
  };
  const keepsOrder = supportsLimits({...next, policy: {}});
  next.policy = {
    ...field.policy,
    minimum: keepsOrder ? field.policy.minimum : undefined,
    maximum: keepsOrder ? field.policy.maximum : undefined,
  };
  if (!modificationsFor(next).includes(field.policy.modification ?? 'none')) next.policy.modification = undefined;
  return next;
}

/** A setting a field loses when its kind changes to one that cannot carry it. */
export type DroppedSetting = 'fields' | 'maxLength' | 'allowedValues' | 'minItems' | 'limits' | 'change';

/** Returns the settings, with a value, that a field lost when its kind changed. */
export function droppedSettings(before: SchemaField, after: SchemaField): DroppedSetting[] {
  const has = (value?: number | string): boolean => value !== undefined && String(value).trim() !== '';
  const dropped: DroppedSetting[] = [];
  if (before.children.length > 0 && after.children.length === 0) dropped.push('fields');
  if (has(before.maxLength) && !has(after.maxLength)) dropped.push('maxLength');
  if ((before.allowedValues?.length ?? 0) > 0 && !after.allowedValues?.length) dropped.push('allowedValues');
  if (has(before.minItems) && !has(after.minItems)) dropped.push('minItems');
  if (
    (has(before.policy.minimum) && !has(after.policy.minimum)) ||
    (has(before.policy.maximum) && !has(after.policy.maximum))
  ) {
    dropped.push('limits');
  }
  if (effectiveModification(before) !== 'none' && effectiveModification(after) === 'none') dropped.push('change');
  return dropped;
}

/** Returns a field, and the fields inside it, without a rule for changing them. */
function withoutChangeRule(field: SchemaField): SchemaField {
  return {
    ...field,
    policy: {...field.policy, modification: undefined, modifiableBy: undefined},
    children: field.children.map(withoutChangeRule),
  };
}

/**
 * Sets who supplies a field. A value the processor supplies cannot change after the request, nor can
 * any value inside an object it supplies, so their change rules are cleared.
 */
export function setSuppliedBy(field: SchemaField, processor: boolean): SchemaField {
  if (!processor) return {...field, policy: {...field.policy, suppliedBy: undefined}};
  const fixed = withoutChangeRule(field);
  return {...fixed, policy: {...fixed.policy, suppliedBy: 'processor'}};
}

/** Adds or removes a party that may change a field, keeping the parties in the server's order. */
export function toggleParty(field: SchemaField, party: ModifyingParty): SchemaField {
  const current = field.policy.modifiableBy ?? [];
  const next = current.includes(party) ? current.filter((p) => p !== party) : [...current, party];
  return {...field, policy: {...field.policy, modifiableBy: PARTIES.filter((p) => next.includes(p))}};
}

/**
 * Returns the sample values a client sends for the fields: the processor fills in its own fields, which
 * a client may not send, and an object left with no value, which the consent screen would show as {},
 * is sent only when it is required.
 */
function sampleObject(fields: SchemaField[]): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const field of fields) {
    const name = field.name.trim();
    if (!name || field.policy.suppliedBy === 'processor') continue;
    if (field.kind !== 'object') {
      setOwnProperty(out, name, sampleData(field));
      continue;
    }
    const value = sampleObject(field.children);
    if (Object.keys(value).length > 0 || field.required) setOwnProperty(out, name, value);
  }
  return out;
}

/**
 * Returns an example authorization detail a client sends for the type, with a sample value for each
 * named field the client sets. The `type` member always carries the type identifier.
 */
export function exampleDetail(fields: SchemaField[], typeValue: string): Record<string, unknown> {
  const values = sampleObject(fields);
  delete values['type'];
  return {type: typeValue.trim() || 'type', ...values};
}

/** One field of a detail as the consent screen shows it, mirroring the server's consent detail row. */
export interface PreviewRow {
  path: string;
  label: string;
  value: unknown;
  modification?: Exclude<Modification, 'none'>;
  as?: ValueType;
  origin?: 'server';
  options?: unknown[];
}

/**
 * Returns the row the consent screen shows for a field of the example detail, as the server builds it:
 * the field's display name or path, its sample value, the change the user may make, the values the user
 * may add to a list, and whether the processor supplies the value.
 */
export function previewRow({
  field,
  path,
  processorSupplied,
}: Pick<FlatField, 'field' | 'path' | 'processorSupplied'>): PreviewRow {
  const row: PreviewRow = {path, label: field.title.trim() || path, value: sampleData(field)};
  // A value the processor supplies, itself or through an object it belongs to, cannot change.
  const modification = processorSupplied ? 'none' : effectiveModification(field);
  if (modification !== 'none' && modifyingParties(field).includes('user')) {
    row.modification = modification;
    const as = valueTypeFor(field);
    if (as) row.as = as;
    const allowed = allowedValuesOf(field);
    if (modification === 'addItems' && allowed.length) {
      row.options = field.itemKind === 'number' ? allowed.map(Number) : allowed;
    }
  }
  if (processorSupplied) row.origin = 'server';
  return row;
}

/**
 * Returns the labels the consent screen would show on more than one row, where the user could not tell
 * the rows apart, since it shows a field by its display name or path and never names its object.
 */
export function duplicateLabels(fields: SchemaField[]): string[] {
  const counts = new Map<string, number>();
  for (const row of flattenFields(fields)) {
    // The consent screen leaves out the type member, which every detail carries.
    if (!row.field.name.trim() || row.field.kind === 'object' || row.path === 'type') continue;
    const label = row.field.title.trim() || row.path;
    counts.set(label, (counts.get(label) ?? 0) + 1);
  }
  return [...counts].filter(([, count]) => count > 1).map(([label]) => label);
}

/** The paths each field has had, keyed by the field, so a summary naming an old one can be updated. */
export type PathHistory = Record<string, string[]>;

/** Whether every part of a path has a name without spaces, as a field the summary can name has. */
const isNamedPath = (path: string): boolean => path.split('.').every((part) => part !== '' && !/\s/.test(part));

/** Returns the history with the current path of each field added, or the same history when it has them. */
export function recordPaths(history: PathHistory, flat: FlatField[]): PathHistory {
  const missing = flat.filter((row) => isNamedPath(row.path) && !history[row.field.key]?.includes(row.path));
  if (missing.length === 0) return history;
  const next = {...history};
  for (const row of missing) next[row.field.key] = [...(next[row.field.key] ?? []), row.path];
  return next;
}

/** A path the summary still names after the field it named was renamed or moved, and where it is now. */
export interface SummaryRename {
  from: string;
  to: string;
  /** The fields whose rename or move left the summary naming the old path. */
  keys: string[];
}

/**
 * Returns the renames that would make the summary name its fields again: for each placeholder that
 * names no field, the field that had its path, or the object it was in, and the path it has now.
 */
export function summaryRenames(unknownPaths: string[], flat: FlatField[], history: PathHistory): SummaryRename[] {
  const named = flat.filter((row) => isNamedPath(row.path));
  const current = new Set(named.map((row) => row.path));
  const renames: SummaryRename[] = [];
  for (const path of unknownPaths) {
    const candidates = named.flatMap((row) =>
      (history[row.field.key] ?? [])
        .filter((base) => base !== row.path && (path === base || path.startsWith(`${base}.`)))
        .filter((base) => current.has(`${row.path}${path.slice(base.length)}`))
        .map((base) => ({base, to: row.path, key: row.field.key})),
    );
    if (candidates.length === 0) continue;
    // The outermost rename covers every placeholder inside it, as an object renamed with its fields.
    const outer = candidates.reduce((best, candidate) => (candidate.base.length < best.base.length ? candidate : best));
    const keys = candidates.map((candidate) => candidate.key);
    const known = renames.find((rename) => rename.from === outer.base && rename.to === outer.to);
    if (known) known.keys = [...new Set([...known.keys, ...keys])];
    else renames.push({from: outer.base, to: outer.to, keys: [...new Set(keys)]});
  }
  return renames;
}
