// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {
  FieldKind,
  FieldPolicy,
  ItemKind,
  Modification,
  ModifyingParty,
  SchemaField,
  ValueType,
} from '../models/authorization-detail-type';

/** The RFC 9396 type member, which every detail carries and which only the client sets. */
const TYPE_FIELD = 'type';

/** The parties that may change a field, in the order the server lists them. */
export const PARTIES: ModifyingParty[] = ['processor', 'user', 'client'];

/** The keyword that carries a property's policy. */
const POLICY_KEYWORD = 'policy';

/** The maximum length a new text field starts with. */
export const DEFAULT_MAX_LENGTH = '100';

const FIELD_KEYS = [
  'type',
  'format',
  'title',
  'description',
  POLICY_KEYWORD,
  'items',
  'properties',
  'required',
  'enum',
  'const',
  'minimum',
  'maximum',
  'exclusiveMinimum',
  'exclusiveMaximum',
  'multipleOf',
  'minLength',
  'maxLength',
  'pattern',
  'minItems',
  'maxItems',
  'uniqueItems',
];

/** The kinds of field that can carry a list of allowed values on the field itself. */
const ALLOWED_VALUE_KINDS: FieldKind[] = ['string', 'number'];

/** The kinds of field that can carry a fixed value. */
const FIXED_VALUE_KINDS: FieldKind[] = ['string', 'number', 'boolean'];

/** Reports whether a field can carry a list of allowed values: a list's items, or a text or number field. */
export function supportsAllowedValues(field: Pick<SchemaField, 'kind'>): boolean {
  return field.kind === 'array' || ALLOWED_VALUE_KINDS.includes(field.kind);
}

/** Reports whether a field can carry a fixed value. */
export function supportsFixedValue(field: Pick<SchemaField, 'kind'>): boolean {
  return FIXED_VALUE_KINDS.includes(field.kind);
}

/** Returns the kind of the values a field's allowed values hold: its items' kind for a list, else its own. */
export function allowedValueKind(field: Pick<SchemaField, 'kind' | 'itemKind'>): ItemKind {
  if (field.kind === 'array') return field.itemKind;
  return field.kind === 'number' ? 'number' : 'string';
}
const POLICY_KEYS = ['suppliedBy', 'modification', 'modifiableBy', 'minimum', 'maximum'];
const MODIFICATIONS: Modification[] = ['none', 'decrease', 'increase', 'removeItems', 'addItems'];
const ITEM_KINDS: ItemKind[] = ['string', 'number'];

let nextKey = 0;

/** Returns a key that is unique for the lifetime of the page. */
function newFieldKey(): string {
  nextKey += 1;
  return `field-${nextKey}`;
}

/** Returns an empty field of the given kind. */
export function newField(kind: FieldKind = 'string'): SchemaField {
  return {key: newFieldKey(), name: '', title: '', kind, required: false, itemKind: 'string', policy: {}, children: []};
}

/** The RFC 9396 common fields a type usually declares, which clients can send only when it does. */
const STANDARD_FIELDS: {name: string; title: string}[] = [
  {name: 'locations', title: 'Locations'},
  {name: 'actions', title: 'Actions'},
];

/** Returns the standard fields, a list of text each, that the given fields do not declare yet. */
export function missingStandardFields(fields: SchemaField[]): SchemaField[] {
  const names = new Set(fields.map((field) => trimSpace(field.name)));
  return STANDARD_FIELDS.filter((standard) => !names.has(standard.name)).map((standard) => ({
    ...newField('array'),
    name: standard.name,
    title: standard.title,
  }));
}

/** Reports whether a field's limits are the schema's own `minimum` and `maximum` keywords. */
function hasSchemaLimits(kind: FieldKind): boolean {
  return kind === 'number';
}

/**
 * Returns how a field's values are ordered when it is lowered, raised, or limited, or undefined when
 * it has no order. As on the server, the order follows from the field's type.
 */
export function valueTypeFor(field: SchemaField): ValueType | undefined {
  switch (field.kind) {
    case 'number':
      return 'number';
    case 'date':
      return 'date';
    case 'dateTime':
      return 'dateTime';
    default:
      return undefined;
  }
}

/** Reports whether a field can carry a minimum and maximum. */
export function supportsLimits(field: SchemaField): boolean {
  return valueTypeFor(field) !== undefined;
}

/**
 * Returns the changes a field can allow: an ordered value can be lowered or raised, and a list can
 * lose items, or gain them when its items take permission away. Any other field cannot change.
 */
export function modificationsFor(field: SchemaField): Modification[] {
  if (field.kind === 'array') return ['none', 'removeItems', 'addItems'];
  if (supportsLimits(field)) return ['none', 'decrease', 'increase'];
  return ['none'];
}

/** Returns the parties a field's policy lets change it, as far as its change allows. */
export function modifyingParties(field: SchemaField): ModifyingParty[] {
  if (effectiveModification(field) === 'none') return [];
  return PARTIES.filter((p) => field.policy.modifiableBy?.includes(p));
}

/** Returns the change a field allows, which is none when the processor supplies it. */
export function effectiveModification(field: SchemaField): Modification {
  if (field.policy.suppliedBy === 'processor') return 'none';
  const modification = field.policy.modification ?? 'none';
  return modificationsFor(field).includes(modification) ? modification : 'none';
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

// The white space the server trims, Go's unicode.IsSpace: unlike String.prototype.trim it counts U+0085 and
// not U+FEFF, so a name or value the server keeps is never changed here.
const SPACE = '\\t\\n\\v\\f\\r \\u0085\\u00a0\\u1680\\u2000-\\u200a\\u2028\\u2029\\u202f\\u205f\\u3000';
const SURROUNDING_SPACE = new RegExp(`^[${SPACE}]+|[${SPACE}]+$`, 'g');

/** Returns text without the white space the server trims from its start and end. */
export function trimSpace(value: string): string {
  return value.replace(SURROUNDING_SPACE, '');
}

/** Reports whether a value is text with nothing but white space in it, which the builder treats as no value. */
function isBlankText(value: unknown): boolean {
  return typeof value === 'string' && trimSpace(value) === '';
}

function onlyKeys(value: Record<string, unknown>, allowed: string[]): boolean {
  return Object.keys(value).every((key) => allowed.includes(key));
}

function parseLimit(value: unknown): string | null {
  return typeof value === 'number' || typeof value === 'string' ? String(value) : null;
}

function parsePolicy(raw: unknown): FieldPolicy | null {
  if (raw === undefined) return {};
  if (!isRecord(raw) || !onlyKeys(raw, POLICY_KEYS)) return null;
  const policy: FieldPolicy = {};
  const suppliedBy = raw['suppliedBy'];
  if (suppliedBy !== undefined) {
    if (suppliedBy !== 'client' && suppliedBy !== 'processor') return null;
    policy.suppliedBy = suppliedBy;
  }
  const modification = raw['modification'];
  if (modification !== undefined) {
    if (!MODIFICATIONS.includes(modification as Modification)) return null;
    policy.modification = modification as Modification;
  }
  const modifiableBy = raw['modifiableBy'];
  if (modifiableBy !== undefined) {
    if (!Array.isArray(modifiableBy) || !modifiableBy.every((p) => PARTIES.includes(p as ModifyingParty))) {
      return null;
    }
    policy.modifiableBy = modifiableBy as ModifyingParty[];
  }
  for (const limit of ['minimum', 'maximum'] as const) {
    const value = raw[limit];
    if (value === undefined) continue;
    const parsed = parseLimit(value);
    if (parsed === null) return null;
    policy[limit] = parsed;
  }
  return policy;
}

/** Reads the values a list's items must be one of, or null when they are not a list of the item kind. */
function parseAllowedValues(items: Record<string, unknown>, itemKind: ItemKind): string[] | null {
  const values = items['enum'];
  if (values === undefined) return [];
  // An empty list allows no value, and a blank value reads as no value, which the builder cannot show.
  if (
    !Array.isArray(values) ||
    values.length === 0 ||
    !values.every((value) => typeof value === itemKind && !isBlankText(value))
  ) {
    return null;
  }
  return values.map((value) => String(value));
}

function parseKind(
  property: Record<string, unknown>,
): {kind: FieldKind; itemKind: ItemKind; allowedValues?: string[]} | null {
  const type = property['type'];
  const format = property['format'];
  const items = property['items'];
  const hasObjectKeys = property['properties'] !== undefined || property['required'] !== undefined;
  const scalar = items === undefined && !hasObjectKeys;
  if (format !== undefined && type !== 'string') return null;
  switch (type) {
    case 'string':
      if (!scalar) return null;
      if (format === 'date') return {kind: 'date', itemKind: 'string'};
      if (format === 'date-time') return {kind: 'dateTime', itemKind: 'string'};
      return format === undefined ? {kind: 'string', itemKind: 'string'} : null;
    case 'number':
    case 'boolean':
      return scalar ? {kind: type, itemKind: 'string'} : null;
    case 'array':
      if (hasObjectKeys || !isRecord(items) || !onlyKeys(items, ['type', 'enum'])) return null;
      if (!ITEM_KINDS.includes(items['type'] as ItemKind)) return null;
      {
        const itemKind = items['type'] as ItemKind;
        const allowedValues = parseAllowedValues(items, itemKind);
        return allowedValues ? {kind: 'array', itemKind, allowedValues} : null;
      }
    case 'object':
      return items === undefined ? {kind: 'object', itemKind: 'string'} : null;
    default:
      return null;
  }
}

/** Reports whether a value is a count a schema keyword such as maxLength can hold. */
function isCount(value: unknown): value is number {
  return Number.isInteger(value) && (value as number) >= 0;
}

/**
 * Reads the keywords that restrict a field's value, each only on the kinds of field the server allows it
 * on: a number's range, step and exclusive bounds, a text's length and pattern, a list's item counts and
 * uniqueness, the allowed values of a text or number field, and a fixed value. A number's range is kept
 * in the policy while editing, as the builder edits every range there. Returns null when a keyword is on
 * a kind of field that cannot carry it, or holds a value the builder cannot show.
 */
function parseValueRules(
  property: Record<string, unknown>,
  kind: FieldKind,
  policy: FieldPolicy,
): {settings: Partial<SchemaField>; allowedValues?: string[]} | null {
  const settings: Partial<SchemaField> = {};
  const present = (key: string): boolean => property[key] !== undefined;

  // A number is limited by the schema's own minimum and maximum, inclusive or exclusive, and other
  // ordered values by their policy.
  const numberKeys = ['minimum', 'maximum', 'exclusiveMinimum', 'exclusiveMaximum', 'multipleOf'];
  if (hasSchemaLimits(kind)) {
    if (policy.minimum !== undefined || policy.maximum !== undefined) return null;
    for (const [limit, exclusive] of [
      ['minimum', 'exclusiveMinimum'],
      ['maximum', 'exclusiveMaximum'],
    ] as const) {
      // A bound is either inclusive or exclusive; a schema that sets both cannot be shown as one bound.
      if (present(limit) && present(exclusive)) return null;
      const key = present(exclusive) ? exclusive : limit;
      if (!present(key)) continue;
      if (typeof property[key] !== 'number') return null;
      policy[limit] = String(property[key]);
      if (key === exclusive) settings[exclusive] = true;
    }
    const step = property['multipleOf'];
    if (step !== undefined) {
      if (typeof step !== 'number' || step <= 0) return null;
      settings.step = String(step);
    }
  } else if (numberKeys.some(present)) {
    return null;
  }

  const stringKeys = ['minLength', 'maxLength', 'pattern'];
  if (kind === 'string') {
    for (const key of ['minLength', 'maxLength'] as const) {
      if (!present(key)) continue;
      if (!isCount(property[key])) return null;
      settings[key] = String(property[key]);
    }
    const pattern = property['pattern'];
    if (pattern !== undefined) {
      // A blank pattern reads as no pattern, which would drop it on save.
      if (typeof pattern !== 'string' || isBlankText(pattern)) return null;
      settings.pattern = pattern;
    }
  } else if (stringKeys.some(present)) {
    return null;
  }

  const listKeys = ['minItems', 'maxItems', 'uniqueItems'];
  if (kind === 'array') {
    for (const key of ['minItems', 'maxItems'] as const) {
      if (!present(key)) continue;
      if (!isCount(property[key])) return null;
      settings[key] = String(property[key]);
    }
    const unique = property['uniqueItems'];
    if (unique !== undefined) {
      if (typeof unique !== 'boolean') return null;
      // Kept as written, false included, so saving does not change the schema.
      settings.uniqueItems = unique;
    }
  } else if (listKeys.some(present)) {
    return null;
  }

  // A list's allowed values are on its items, read with its kind.
  let allowedValues: string[] | undefined;
  const values = property['enum'];
  if (values !== undefined) {
    if (!ALLOWED_VALUE_KINDS.includes(kind)) return null;
    const valueKind = kind === 'number' ? 'number' : 'string';
    // An empty list allows no value, and a blank value reads as no value, which the builder cannot show.
    if (
      !Array.isArray(values) ||
      values.length === 0 ||
      !values.every((value) => typeof value === valueKind && !isBlankText(value))
    ) {
      return null;
    }
    allowedValues = values.map((value) => String(value));
  }

  const fixed = property['const'];
  if (fixed !== undefined) {
    // A blank fixed value reads as no fixed value, which would drop it on save.
    if (!FIXED_VALUE_KINDS.includes(kind) || typeof fixed !== kind || isBlankText(fixed)) return null;
    settings.fixedValue = String(fixed as string | number | boolean);
  }
  return {settings, ...(allowedValues && {allowedValues})};
}

function parseProperties(properties: unknown, required: unknown): SchemaField[] | null {
  if (properties === undefined) return [];
  if (!isRecord(properties)) return null;
  if (required !== undefined && (!Array.isArray(required) || !required.every((r) => typeof r === 'string'))) {
    return null;
  }
  const requiredNames = Array.isArray(required) ? required : [];
  const fields: SchemaField[] = [];
  for (const [name, property] of Object.entries(properties)) {
    if (!isRecord(property) || !onlyKeys(property, FIELD_KEYS)) return null;
    const title = property['title'];
    // A blank title reads as no title, which would drop it on save.
    if (title !== undefined && (typeof title !== 'string' || isBlankText(title))) return null;
    const policy = parsePolicy(property[POLICY_KEYWORD]);
    if (!policy) return null;
    const kind = parseKind(property);
    if (!kind) return null;
    // An object only groups fields; a change rule on it is one the builder cannot show.
    if (kind.kind === 'object' && (policy.modification ?? 'none') !== 'none') return null;
    const description = property['description'];
    // A blank description reads as no description, which would drop it on save.
    if (description !== undefined && (typeof description !== 'string' || isBlankText(description))) return null;
    const rules = parseValueRules(property, kind.kind, policy);
    if (!rules) return null;
    const children = kind.kind === 'object' ? parseProperties(property['properties'], property['required']) : [];
    if (!children) return null;
    const allowedValues = kind.kind === 'array' ? kind.allowedValues : rules.allowedValues;
    fields.push({
      key: newFieldKey(),
      name,
      title: title ?? '',
      kind: kind.kind,
      itemKind: kind.itemKind,
      ...(allowedValues?.length && {allowedValues}),
      required: requiredNames.includes(name),
      policy,
      ...(description !== undefined && {description}),
      ...rules.settings,
      children,
    });
  }
  return fields;
}

/**
 * Reads a schema into builder fields. Returns null when the schema uses anything the builder cannot
 * show, such as a list of objects, a list of lists, or a title or description on the schema itself.
 */
export function schemaToFields(schema: unknown): SchemaField[] | null {
  if (!isRecord(schema) || schema['type'] !== 'object' || !onlyKeys(schema, ['type', 'properties', 'required'])) {
    return null;
  }
  return parseProperties(schema['properties'], schema['required']);
}

/**
 * Returns where a schema uses something the composer cannot show: the dotted path of the first such
 * field, an empty string when it is the schema itself (for example a title on the schema), or null when
 * the composer can show it all.
 */
export function composerUnsupportedPath(schema: unknown, prefix = ''): string | null {
  if (!isRecord(schema)) return prefix;
  if (!prefix && (schema['type'] !== 'object' || !onlyKeys(schema, ['type', 'properties', 'required']))) return '';
  const properties = schema['properties'];
  if (properties === undefined) return null;
  if (!isRecord(properties)) return prefix;
  for (const [name, property] of Object.entries(properties)) {
    const path = prefix ? `${prefix}.${name}` : name;
    if (parseProperties({[name]: property}, undefined) !== null) continue;
    // An object the composer can show as a group points at the field inside it it cannot show.
    const inner = isRecord(property) && property['type'] === 'object' ? composerUnsupportedPath(property, path) : null;
    return inner ?? path;
  }
  return parseProperties(properties, schema['required']) === null ? prefix : null;
}

/** Reads a limit as typed, as a JSON number when the field holds numbers, or as text otherwise. */
function toLimit(value: number | string | undefined, asNumber: boolean): number | string | undefined {
  if (value === undefined || String(value).trim() === '') return undefined;
  const text = String(value).trim();
  if (asNumber) {
    const parsed = Number(text);
    return Number.isFinite(parsed) ? parsed : text;
  }
  return text;
}

function policyToSchema(field: SchemaField): Record<string, unknown> | undefined {
  const {policy} = field;
  const out: Record<string, unknown> = {};
  if (policy.suppliedBy === 'processor') out['suppliedBy'] = 'processor';
  const modification = effectiveModification(field);
  if (modification !== 'none') {
    out['modification'] = modification;
    const parties = modifyingParties(field);
    if (parties.length) out['modifiableBy'] = parties;
  }
  // How values compare is inferred from the property's type and format, so it is never written.
  // A number's limits are the property's own keywords, so only other ordered values carry them here.
  const valueType = valueTypeFor(field);
  if (valueType && !hasSchemaLimits(field.kind)) {
    // A date and a date and time are written as text, so their limits are too.
    const minimum = toLimit(policy.minimum, false);
    const maximum = toLimit(policy.maximum, false);
    if (minimum !== undefined) out['minimum'] = minimum;
    if (maximum !== undefined) out['maximum'] = maximum;
  }
  return Object.keys(out).length > 0 ? out : undefined;
}

/** Writes a count or other number setting as typed, as a JSON number when it reads as one. */
function writeNumber(property: Record<string, unknown>, key: string, value: string | undefined): void {
  const text = value?.trim();
  if (!text) return;
  const parsed = Number(text);
  property[key] = Number.isFinite(parsed) ? parsed : text;
}

/** Returns a fixed value as the JSON value its field's kind holds, or the text as typed when it is not one. */
function fixedValueOf(field: SchemaField): unknown {
  const text = field.fixedValue ?? '';
  if (field.kind === 'number') {
    const parsed = Number(text.trim());
    return text.trim() !== '' && Number.isFinite(parsed) ? parsed : text;
  }
  if (field.kind === 'boolean') {
    if (text.trim() === 'true') return true;
    if (text.trim() === 'false') return false;
  }
  return text;
}

/**
 * Writes the standard keywords that constrain a field's value: a number's range, exclusive or not, and
 * step; a text's length and pattern; a list's item counts and uniqueness; the allowed values of a text
 * or number field; and a fixed value.
 */
function writeConstraints(field: SchemaField, property: Record<string, unknown>): void {
  if (hasSchemaLimits(field.kind)) {
    const minimum = toLimit(field.policy.minimum, true);
    const maximum = toLimit(field.policy.maximum, true);
    if (minimum !== undefined) property[field.exclusiveMinimum ? 'exclusiveMinimum' : 'minimum'] = minimum;
    if (maximum !== undefined) property[field.exclusiveMaximum ? 'exclusiveMaximum' : 'maximum'] = maximum;
    writeNumber(property, 'multipleOf', field.step);
  }
  if (field.kind === 'string') {
    writeNumber(property, 'minLength', field.minLength);
    writeNumber(property, 'maxLength', field.maxLength);
    if (field.pattern && trimSpace(field.pattern)) property['pattern'] = field.pattern;
  }
  if (field.kind === 'array') {
    writeNumber(property, 'minItems', field.minItems);
    writeNumber(property, 'maxItems', field.maxItems);
    if (field.uniqueItems !== undefined) property['uniqueItems'] = field.uniqueItems;
  }
  if (ALLOWED_VALUE_KINDS.includes(field.kind)) {
    const allowed = allowedValuesOf(field);
    if (allowed.length) property['enum'] = field.kind === 'number' ? allowed.map(Number) : allowed;
  }
  if (supportsFixedValue(field) && trimSpace(field.fixedValue ?? '') !== '') {
    property['const'] = fixedValueOf(field);
  }
}

/** Returns a field's allowed values, trimmed as the server trims them and without blanks. */
export function allowedValuesOf(field: SchemaField): string[] {
  return (field.allowedValues ?? []).map((value) => trimSpace(value)).filter(Boolean);
}

function schemaProperty(kind: FieldKind): Record<string, unknown> {
  switch (kind) {
    case 'date':
      return {type: 'string', format: 'date'};
    case 'dateTime':
      return {type: 'string', format: 'date-time'};
    default:
      return {type: kind};
  }
}

/** Sets a property even when its name is `__proto__`, which plain assignment would treat as the prototype. */
export function setOwnProperty(target: Record<string, unknown>, name: string, value: unknown): void {
  Object.defineProperty(target, name, {value, enumerable: true, configurable: true, writable: true});
}

function fieldsToProperties(fields: SchemaField[]): {properties: Record<string, unknown>; required: string[]} {
  const properties: Record<string, unknown> = {};
  const required: string[] = [];
  for (const field of fields) {
    const property: Record<string, unknown> = schemaProperty(field.kind);
    if (trimSpace(field.title)) property['title'] = field.title;
    if (field.description && trimSpace(field.description)) property['description'] = field.description;
    if (field.kind === 'array') {
      const items: Record<string, unknown> = {type: field.itemKind};
      const allowed = allowedValuesOf(field);
      if (allowed.length) items['enum'] = field.itemKind === 'number' ? allowed.map(Number) : allowed;
      property['items'] = items;
    }
    if (field.kind === 'object') {
      const nested = fieldsToProperties(field.children);
      if (nested.required.length) property['required'] = nested.required;
      property['properties'] = nested.properties;
    }
    writeConstraints(field, property);
    const policy = policyToSchema(field);
    if (policy) property[POLICY_KEYWORD] = policy;
    setOwnProperty(properties, trimSpace(field.name), property);
    if (field.required) required.push(trimSpace(field.name));
  }
  return {properties, required};
}

/** Writes builder fields as a JSON Schema object with a `policy` on each property that has one. */
export function fieldsToSchema(fields: SchemaField[]): Record<string, unknown> {
  const {properties, required} = fieldsToProperties(fields);
  const schema: Record<string, unknown> = {type: 'object'};
  if (required.length) schema['required'] = required;
  schema['properties'] = properties;
  return schema;
}

/** Returns a list of names sorted and without repeats, or the value as it is when it is not one. */
function sortedNames(value: unknown): unknown {
  if (!Array.isArray(value) || !value.every((item) => typeof item === 'string')) return value;
  return [...new Set(value)].sort();
}

/** Returns a policy without the settings written as their defaults, with its parties sorted. */
function canonicalPolicy(value: unknown): Record<string, unknown> {
  if (!isRecord(value)) return {'': value};
  const out: Record<string, unknown> = {};
  for (const [key, setting] of Object.entries(value)) {
    if ((key === 'suppliedBy' && setting === 'client') || (key === 'modification' && setting === 'none')) continue;
    out[key] = key === 'modifiableBy' ? sortedNames(setting) : setting;
  }
  return out;
}

/** Returns a schema in the form the server compares two schemas in, as its SchemaEquivalent does. */
function canonicalSchema(value: unknown, root: boolean): unknown {
  if (!isRecord(value)) return value;
  const out: Record<string, unknown> = {};
  for (const [key, member] of Object.entries(value)) {
    if (key === 'properties' && isRecord(member)) {
      if (Object.keys(member).length === 0) continue;
      out[key] = Object.fromEntries(
        Object.entries(member).map(([name, property]) => [name, canonicalSchema(property, false)]),
      );
    } else if (key === 'items') {
      out[key] = canonicalSchema(member, false);
    } else if (key === 'required') {
      const names = sortedNames(member);
      const kept = root && Array.isArray(names) ? names.filter((name) => name !== TYPE_FIELD) : names;
      if (!Array.isArray(kept) || kept.length > 0) out[key] = kept;
    } else if (key === POLICY_KEYWORD) {
      const policy = canonicalPolicy(member);
      if (Object.keys(policy).length > 0) out[key] = policy;
    } else {
      out[key] = member;
    }
  }
  return out;
}

/** Returns a value with the keys of each object in it sorted, so its JSON text does not depend on their order. */
function sortKeys(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(sortKeys);
  if (!isRecord(value)) return value;
  return Object.fromEntries(
    Object.keys(value)
      .sort()
      .map((key) => [key, sortKeys(value[key])]),
  );
}

/**
 * Returns the JSON text of what a schema defines, which is the same for two schemas the server treats
 * as one: the order of properties, required names and parties, a repeated name, `type` in the root's
 * required names, a policy setting written as its default, and empty properties do not count.
 */
export function schemaDefinitionText(schema: unknown): string {
  return JSON.stringify(sortKeys(canonicalSchema(schema, true)));
}

/** A problem with a builder field, keyed for translation. */
export interface FieldIssue {
  /** The key of the field the issue belongs to. */
  fieldKey: string;
  /** The translation key, under `authorizationDetailTypes:`. */
  messageKey: string;
  /** The English message. */
  defaultMessage: string;
}

const NUMBER = /^-?\d+(\.\d+)?([eE][+-]?\d+)?$/;
const DATE = /^(\d{4})-(\d{2})-(\d{2})$/;
// As the server parses RFC 3339: an upper case T and Z, a fraction after a dot or a comma, and no leap
// second.
const DATE_TIME =
  /^(\d{4})-(\d{2})-(\d{2})T([01]\d|2[0-3]):[0-5]\d:[0-5]\d([.,](\d+))?(Z|[+-]([01]\d|2[0-3]):[0-5]\d)$/;

/** Reports whether a value is a JSON number the server holds as a finite double. */
function isFiniteNumber(value: string): boolean {
  return NUMBER.test(value) && Number.isFinite(Number(value));
}

/**
 * Splits a JSON number into its sign, its significant digits and the power of ten of the last of them, so
 * two numbers compare exactly, as the server compares them, whatever their digits.
 */
function decimalParts(value: string): {negative: boolean; digits: string; exponent: number} {
  const match = /^(-?)(\d+)(?:\.(\d+))?(?:[eE]([+-]?\d+))?$/.exec(value) ?? ['', '', '0', '', '0'];
  const fraction = match[3] ?? '';
  let digits = `${match[2]}${fraction}`.replace(/^0+/, '');
  let exponent = Number(match[4] ?? 0) - fraction.length;
  const trailing = /0+$/.exec(digits)?.[0].length ?? 0;
  digits = digits.slice(0, digits.length - trailing);
  exponent += trailing;
  return {negative: match[1] === '-' && digits !== '', digits, exponent};
}

/** Compares two JSON numbers exactly: negative when the first is lower, positive when it is higher. */
function compareDecimals(a: string, b: string): number {
  const x = decimalParts(a);
  const y = decimalParts(b);
  if (x.negative !== y.negative) return x.negative ? -1 : 1;
  const sign = x.negative ? -1 : 1;
  if (!x.digits || !y.digits) return sign * (x.digits.length - y.digits.length);
  const magnitude = x.digits.length + x.exponent - (y.digits.length + y.exponent);
  if (magnitude !== 0) return sign * magnitude;
  const length = Math.max(x.digits.length, y.digits.length);
  const xs = x.digits.padEnd(length, '0');
  const ys = y.digits.padEnd(length, '0');
  if (xs === ys) return 0;
  return sign * (xs < ys ? -1 : 1);
}

/**
 * Reports whether a number as typed is kept exactly once saved. The page holds numbers as doubles, so a
 * number with more digits than a double holds would be saved as a nearby one.
 */
function isExactNumber(value: string): boolean {
  return compareDecimals(value, String(Number(value))) === 0;
}

/** Reports whether a year, month and day name a day that exists, from year 0000 on, as the server reads it. */
function isCalendarDate(year: string, month: string, day: string): boolean {
  const date = new Date(0);
  date.setUTCFullYear(Number(year), Number(month) - 1, Number(day));
  return (
    date.getUTCFullYear() === Number(year) &&
    date.getUTCMonth() === Number(month) - 1 &&
    date.getUTCDate() === Number(day)
  );
}

/**
 * Reports whether a limit is written the way its value type compares, as the server reads it: a JSON
 * number, a full-date (YYYY-MM-DD), or an RFC 3339 date-time with its offset.
 */
export function isValidLimit(value: string, valueType: ValueType): boolean {
  switch (valueType) {
    case 'number':
      return isFiniteNumber(value);
    case 'date': {
      const match = DATE.exec(value);
      return match !== null && isCalendarDate(match[1], match[2], match[3]);
    }
    default: {
      const match = DATE_TIME.exec(value);
      return match !== null && isCalendarDate(match[1], match[2], match[3]);
    }
  }
}

/**
 * Compares two valid limits the way the server does: numbers exactly, dates as days, and dates and times
 * as instants to the nanosecond. Returns a negative number when the first is earlier or lower.
 */
function compareLimits(a: string, b: string, valueType: ValueType): number {
  if (valueType === 'number') return compareDecimals(a, b);
  if (valueType === 'date') return a === b ? 0 : a < b ? -1 : 1;
  // The browser reads an instant to the millisecond, so a tie is decided by the digits after it.
  const instant = (value: string): {ms: number; rest: string} => {
    const fraction = DATE_TIME.exec(value)?.[6] ?? '';
    return {ms: Date.parse(value.replace(',', '.')), rest: fraction.padEnd(9, '0').slice(3, 9)};
  };
  const x = instant(a);
  const y = instant(b);
  if (x.ms !== y.ms) return x.ms - y.ms;
  return x.rest === y.rest ? 0 : x.rest < y.rest ? -1 : 1;
}

/**
 * The largest count a schema keyword such as maxLength or minItems can hold, as the server reads it: a
 * 32-bit integer.
 */
export const MAX_SCHEMA_COUNT = 2147483647;

/** Reports whether a count, such as a maximum length, is a whole number. */
function isWholeNumber(value: string): boolean {
  return /^\d+$/.test(value);
}

/** Returns the index just past the character class a pattern opens at `start`, or -1 when it is not closed. */
function patternClassEnd(pattern: string, start: number): number {
  let i = start + 1;
  if (pattern[i] === '^') i += 1;
  // Read as RE2 reads a class: a closing bracket first in it is a literal one, and a hyphen makes a range
  // only after a single character, so a hyphen first in the class or after a POSIX class is a literal one.
  let first = true;
  while (i < pattern.length && (pattern[i] !== ']' || first)) {
    first = false;
    const posixEnd = pattern.startsWith('[:', i) ? pattern.indexOf(':]', i + 2) : -1;
    if (posixEnd >= 0) {
      i = posixEnd + 2;
      continue;
    }
    i += pattern[i] === '\\' ? 2 : 1;
    if (pattern[i] === '-' && i + 1 < pattern.length && pattern[i + 1] !== ']') {
      // The end of a range is one character, even a bracket, never the start of a POSIX class.
      i += pattern[i + 1] === '\\' ? 3 : 2;
    }
  }
  return i < pattern.length ? i + 1 : -1;
}

/**
 * Returns why the server cannot compile a pattern, or null when nothing here rules it out. The server reads
 * a pattern as Go's RE2, whose syntax a browser does not read, so only what RE2 refuses whatever else the
 * pattern holds is checked: a lookaround or a backreference, and a group or character class left open, a
 * group closed without being opened, or a backslash at the end. The server checks the rest when the type
 * is saved.
 */
export function patternProblem(pattern: string): 'unsupported' | 'incomplete' | null {
  let depth = 0;
  let i = 0;
  while (i < pattern.length) {
    const char = pattern[i];
    if (char === '\\') {
      const next = pattern[i + 1];
      if (next === undefined) return 'incomplete';
      if (next === 'Q') {
        // Quoted text runs to \E, or to the end.
        const end = pattern.indexOf('\\E', i + 2);
        i = end < 0 ? pattern.length : end + 2;
        continue;
      }
      // A digit from 1 to 7 starts an octal code only when another octal digit follows; alone, or 8 and 9,
      // it is a backreference, as is \k.
      if (next === '8' || next === '9' || next === 'k' || (/[1-7]/.test(next) && !/[0-7]/.test(pattern[i + 2] ?? ''))) {
        return 'unsupported';
      }
      i += 2;
    } else if (char === '[') {
      const end = patternClassEnd(pattern, i);
      if (end < 0) return 'incomplete';
      i = end;
    } else {
      if (char === '(') {
        if (['(?=', '(?!', '(?<=', '(?<!'].some((look) => pattern.startsWith(look, i))) return 'unsupported';
        depth += 1;
      } else if (char === ')') {
        depth -= 1;
        if (depth < 0) return 'incomplete';
      }
      i += 1;
    }
  }
  return depth === 0 ? null : 'incomplete';
}

function collectIssues(
  fields: SchemaField[],
  atRoot: boolean,
  inSupplied: boolean,
  typeIdentifier: string,
  issues: FieldIssue[],
): void {
  const seen = new Set<string>();
  for (const field of fields) {
    const name = trimSpace(field.name);
    const issue = (messageKey: string, defaultMessage: string): void => {
      issues.push({fieldKey: field.key, messageKey, defaultMessage});
    };
    // A repeated name is reported before surrounding space, since the schema written from the two fields
    // trims both names to the same key and keeps only one of them.
    if (!name) {
      issue('fields.errors.nameRequired', 'Enter a field name.');
    } else if (seen.has(name)) {
      issue('fields.errors.nameDuplicate', 'Another field at this level has the same name.');
    } else if (name !== field.name) {
      issue('fields.errors.nameSpaces', 'A field name cannot start or end with a space.');
    } else if (name.includes('.')) {
      issue('fields.errors.nameDot', 'A field name cannot contain a dot.');
    }
    seen.add(name);

    const {policy} = field;
    if (atRoot && name === TYPE_FIELD) {
      if (field.kind !== 'string') {
        issue('fields.errors.typeKind', 'The type identifies the request, so it must be plain text.');
      } else if (policy.suppliedBy === 'processor') {
        issue(
          'fields.errors.typeSupplied',
          'The type identifies the request, so the external processor cannot set it.',
        );
      }
    }
    if (inSupplied && effectiveModification(field) !== 'none') {
      // As on the server, a value inside an object the processor supplies cannot change after the request.
      issue(
        'fields.errors.modifiableInSupplied',
        'Your external processor fills in the object this field belongs to, so nobody can change it after the request.',
      );
    } else if (effectiveModification(field) !== 'none' && modifyingParties(field).length === 0) {
      issue('fields.errors.modifiableByRequired', 'Choose who can change this field.');
    }
    if (field.kind === 'dateTime' && modifyingParties(field).includes('user')) {
      issue(
        'fields.errors.dateTimeUser',
        'The consent screen cannot change a date and time, so only the client or the external processor can change it.',
      );
    }
    const valueType = valueTypeFor(field);
    if (valueType) {
      const minimum = policy.minimum === undefined ? '' : String(policy.minimum).trim();
      const maximum = policy.maximum === undefined ? '' : String(policy.maximum).trim();
      const limits = [minimum, maximum].filter(Boolean);
      // As on the server, an inclusive limit is compared only with an inclusive one, and an exclusive limit
      // only with an exclusive one.
      const comparable = Boolean(field.exclusiveMinimum) === Boolean(field.exclusiveMaximum);
      if (limits.some((limit) => !isValidLimit(limit, valueType))) {
        issue('fields.errors.limitInvalid', 'Enter the minimum and maximum as values of this type.');
      } else if (valueType === 'number' && limits.some((limit) => !isExactNumber(limit))) {
        issue('fields.errors.limitPrecision', 'Enter the minimum and maximum with at most 15 significant digits.');
      } else if (limits.length === 2 && comparable && compareLimits(minimum, maximum, valueType) > 0) {
        issue('fields.errors.limitOrder', 'The minimum cannot be greater than the maximum.');
      }
      if (field.kind === 'number' && ((field.exclusiveMinimum && !minimum) || (field.exclusiveMaximum && !maximum))) {
        issue('fields.errors.exclusiveLimitRequired', 'Enter the limit that the value must be above or below.');
      }
    }
    const maxLength = field.maxLength?.trim();
    if (field.kind === 'string' && maxLength && !isWholeNumber(maxLength)) {
      issue('fields.errors.maxLengthInvalid', 'Enter the maximum length as a whole number.');
    } else if (field.kind === 'string' && maxLength && Number(maxLength) > MAX_SCHEMA_COUNT) {
      issue('fields.errors.maxLengthTooLarge', 'The maximum length can be at most 2147483647.');
    }
    const minItems = field.minItems?.trim();
    if (field.kind === 'array' && minItems && !isWholeNumber(minItems)) {
      issue('fields.errors.minItemsInvalid', 'Enter the minimum number of items as a whole number.');
    } else if (field.kind === 'array' && minItems && Number(minItems) > MAX_SCHEMA_COUNT) {
      issue('fields.errors.minItemsTooLarge', 'The minimum number of items can be at most 2147483647.');
    }
    const minLength = field.minLength?.trim();
    if (field.kind === 'string' && minLength && !isWholeNumber(minLength)) {
      issue('fields.errors.minLengthInvalid', 'Enter the minimum length as a whole number.');
    } else if (field.kind === 'string' && minLength && Number(minLength) > MAX_SCHEMA_COUNT) {
      issue('fields.errors.minLengthTooLarge', 'The minimum length can be at most 2147483647.');
    } else if (
      field.kind === 'string' &&
      minLength &&
      maxLength &&
      isWholeNumber(maxLength) &&
      Number(minLength) > Number(maxLength)
    ) {
      issue('fields.errors.lengthOrder', 'The minimum length cannot be greater than the maximum length.');
    }
    const problem = field.kind === 'string' && field.pattern ? patternProblem(field.pattern) : null;
    if (problem === 'unsupported') {
      issue(
        'fields.errors.patternUnsupported',
        'The pattern uses a lookaround or a backreference, which is not supported.',
      );
    } else if (problem === 'incomplete') {
      issue(
        'fields.errors.patternIncomplete',
        'The pattern is incomplete. Check that each group and character class is closed.',
      );
    }
    const maxItems = field.maxItems?.trim();
    if (field.kind === 'array' && maxItems && !isWholeNumber(maxItems)) {
      issue('fields.errors.maxItemsInvalid', 'Enter the maximum number of items as a whole number.');
    } else if (field.kind === 'array' && maxItems && Number(maxItems) > MAX_SCHEMA_COUNT) {
      issue('fields.errors.maxItemsTooLarge', 'The maximum number of items can be at most 2147483647.');
    } else if (
      field.kind === 'array' &&
      maxItems &&
      minItems &&
      isWholeNumber(minItems) &&
      Number(minItems) > Number(maxItems)
    ) {
      issue('fields.errors.itemsOrder', 'The minimum number of items cannot be greater than the maximum.');
    }
    const step = field.step?.trim();
    if (field.kind === 'number' && step && !(isFiniteNumber(step) && Number(step) > 0)) {
      issue('fields.errors.stepInvalid', 'Enter the step as a number greater than 0.');
    } else if (field.kind === 'number' && step && !isExactNumber(step)) {
      issue('fields.errors.stepPrecision', 'Enter the step with at most 15 significant digits.');
    }
    const allowed = supportsAllowedValues(field) ? allowedValuesOf(field) : [];
    const valueKind = allowedValueKind(field);
    const fixed = trimSpace(field.fixedValue ?? '');
    if (supportsFixedValue(field) && fixed) {
      if (field.kind === 'number' && !isFiniteNumber(fixed)) {
        issue('fields.errors.fixedValueInvalid', 'Enter the fixed value as a number.');
      } else if (field.kind === 'number' && !isExactNumber(fixed)) {
        issue('fields.errors.fixedValuePrecision', 'Enter the fixed value with at most 15 significant digits.');
      } else if (field.kind === 'boolean' && fixed !== 'true' && fixed !== 'false') {
        issue('fields.errors.fixedValueBoolean', 'Choose true or false as the fixed value.');
      }
    }
    if (atRoot && name === TYPE_FIELD && typeIdentifier) {
      // As on the server, a fixed value and allowed values on the type must each let it hold the type
      // identifier, since a request must satisfy both. The fixed value is written as typed, untrimmed, so it
      // is compared as written.
      const fixedExcludes = supportsFixedValue(field) && fixed && String(fixedValueOf(field)) !== typeIdentifier;
      if ((allowed.length && !allowed.includes(typeIdentifier)) || fixedExcludes) {
        issue(
          'fields.errors.typeValueExcluded',
          'The fixed value or allowed values of the type must include the type identifier.',
        );
      }
    }
    const written = supportsAllowedValues(field) ? (field.allowedValues ?? []).filter((value) => trimSpace(value)) : [];
    if (written.some((value) => value !== trimSpace(value))) {
      issue('fields.errors.allowedValuesSpaces', 'An allowed value cannot start or end with a space.');
    } else if (valueKind === 'number' && allowed.some((value) => !isFiniteNumber(value))) {
      issue('fields.errors.allowedValuesInvalid', 'Enter each allowed value as a number.');
    } else if (valueKind === 'number' && allowed.some((value) => !isExactNumber(value))) {
      issue('fields.errors.allowedValuesPrecision', 'Enter each allowed value with at most 15 significant digits.');
    } else if (
      new Set<string | number>(valueKind === 'number' ? allowed.map(Number) : allowed).size !== allowed.length
    ) {
      // Numbers are compared by value, as the server compares them, so 1 and 1.0 are the same.
      issue('fields.errors.allowedValuesDuplicate', 'Each allowed value can appear only once.');
    }
    if (field.kind === 'object') {
      collectIssues(field.children, false, inSupplied || policy.suppliedBy === 'processor', typeIdentifier, issues);
    }
  }
}

/**
 * Checks builder fields against the rules the server applies to a schema and its policies. With the type
 * identifier, it also checks that a fixed value or allowed values on the `type` field let it hold it.
 */
export function validateFields(fields: SchemaField[], typeIdentifier = ''): FieldIssue[] {
  const issues: FieldIssue[] = [];
  collectIssues(fields, true, false, typeIdentifier, issues);
  return issues;
}

/** The number a sample starts from, moved inside the field's limits when it is outside them. */
const SAMPLE_NUMBER = 100;

/**
 * Returns a sample number for a field that meets every limit it sets: its minimum and maximum, either of
 * them exclusive, and its step. A field no number meets gets the nearest limit.
 */
function sampleNumber(field: SchemaField): number {
  const limit = (value?: number | string): number | undefined => {
    const text = value === undefined ? '' : String(value).trim();
    return isFiniteNumber(text) ? Number(text) : undefined;
  };
  const minimum = limit(field.policy.minimum);
  const maximum = limit(field.policy.maximum);
  const stepLimit = limit(field.step);
  const step = stepLimit !== undefined && stepLimit > 0 ? stepLimit : undefined;
  // Rounded, so a multiple such as 3 times 0.1 is 0.3.
  const round = (value: number): number => Number(value.toPrecision(15));
  const fits = (value: number): boolean =>
    (minimum === undefined || (field.exclusiveMinimum ? value > minimum : value >= minimum)) &&
    (maximum === undefined || (field.exclusiveMaximum ? value < maximum : value <= maximum)) &&
    (step === undefined || Math.abs(value / step - Math.round(value / step)) < 1e-9);
  const anchors = [SAMPLE_NUMBER, maximum, minimum].filter((value): value is number => value !== undefined);
  const candidates = step
    ? anchors.flatMap((anchor) => {
        const below = Math.floor(anchor / step);
        return [below, below + 1, below - 1].map((multiple) => round(multiple * step));
      })
    : [
        ...anchors.flatMap((anchor) => [anchor, anchor - 1, anchor + 1]),
        ...(minimum !== undefined && maximum !== undefined ? [round((minimum + maximum) / 2)] : []),
      ];
  const sample = candidates.find(fits);
  if (sample !== undefined) return sample;
  if (maximum !== undefined && SAMPLE_NUMBER > maximum) return maximum;
  if (minimum !== undefined && SAMPLE_NUMBER < minimum) return minimum;
  return SAMPLE_NUMBER;
}

/** The date a sample starts from, moved inside the field's limits when it is outside them. */
const SAMPLE_DATE = '2026-01-31';
const SAMPLE_DATE_TIME = '2026-01-31T09:00:00Z';

/** Returns a sample date, or date and time, for a field, within its minimum and maximum. */
function sampleDate(field: SchemaField, valueType: 'date' | 'dateTime'): string {
  const limit = (value?: number | string): string | undefined => {
    const text = value === undefined ? '' : String(value).trim();
    return isValidLimit(text, valueType) ? text : undefined;
  };
  const sample = valueType === 'date' ? SAMPLE_DATE : SAMPLE_DATE_TIME;
  const minimum = limit(field.policy.minimum);
  const maximum = limit(field.policy.maximum);
  if (maximum !== undefined && compareLimits(sample, maximum, valueType) > 0) return maximum;
  if (minimum !== undefined && compareLimits(sample, minimum, valueType) < 0) return minimum;
  return sample;
}

/** The sample items of a list of text, which also caps how many items a sample list holds. */
const SAMPLE_ITEMS = ['first', 'second', 'third', 'fourth', 'fifth', 'sixth', 'seventh', 'eighth', 'ninth', 'tenth'];

/** Returns a sample text for a field: a currency code for a currency, or else its name in brackets. */
function sampleText(field: SchemaField): string {
  if (/currency$/i.test(field.name.trim())) return 'EUR';
  return `‹${field.title.trim() || field.name.trim()}›`;
}

/**
 * Returns a sample of the value a field holds, as a request would carry it: a number within the
 * field's limits, a date, a list of the first allowed values, or the fields of an object.
 */
export function sampleData(field: SchemaField): unknown {
  // A field with a fixed value, or with allowed values on the field itself, holds one of them.
  if (supportsFixedValue(field) && trimSpace(field.fixedValue ?? '') !== '') return fixedValueOf(field);
  const allowed = ALLOWED_VALUE_KINDS.includes(field.kind) ? allowedValuesOf(field) : [];
  if (allowed.length) return field.kind === 'number' ? Number(allowed[0]) : allowed[0];
  switch (field.kind) {
    case 'number':
      return sampleNumber(field);
    case 'date':
      return sampleDate(field, 'date');
    case 'dateTime':
      return sampleDate(field, 'dateTime');
    case 'boolean':
      return true;
    case 'array': {
      // A list the user may add to starts with one allowed value, so the others are offered to add.
      const allowed = allowedValuesOf(field);
      const wanted = allowed.length ? Math.min(effectiveModification(field) === 'addItems' ? 1 : 2, allowed.length) : 2;
      // A list holds at least its fewest items, up to a few, repeating allowed values when it must.
      const fewest = Number(field.minItems?.trim() ?? 0);
      const count = Math.min(Math.max(wanted, Number.isInteger(fewest) ? fewest : 0), SAMPLE_ITEMS.length);
      const values = Array.from({length: count}, (_, index) => {
        if (allowed.length) return allowed[index % allowed.length];
        return field.itemKind === 'string' ? SAMPLE_ITEMS[index] : String(index + 1);
      });
      return field.itemKind === 'number' ? values.map(Number) : values;
    }
    case 'object': {
      const out: Record<string, unknown> = {};
      for (const child of field.children) {
        const name = trimSpace(child.name);
        if (name) setOwnProperty(out, name, sampleData(child));
      }
      return out;
    }
    default:
      return sampleText(field);
  }
}

/**
 * Returns the sample value a consent preview shows for a field, written as the consent screen writes it:
 * an object as its JSON text, as the server writes an object a summary names.
 */
export function sampleValue(field: SchemaField): string {
  const value = sampleData(field);
  if (field.kind === 'object') return JSON.stringify(value);
  return Array.isArray(value) ? value.map(String).join(', ') : String(value);
}

function collectSamples(fields: SchemaField[], prefix: string, samples: Map<string, string>): void {
  for (const field of fields) {
    const path = prefix ? `${prefix}.${trimSpace(field.name)}` : trimSpace(field.name);
    samples.set(path, sampleValue(field));
    if (field.kind === 'object') collectSamples(field.children, path, samples);
  }
}

/** Reports whether any field relies on the processor, to set its value or to change it. */
export function usesProcessor(fields: SchemaField[]): boolean {
  return fields.some(
    (field) =>
      field.policy.suppliedBy === 'processor' ||
      modifyingParties(field).includes('processor') ||
      (field.kind === 'object' && usesProcessor(field.children)),
  );
}

/** Returns the paths of the fields that hold a value, for inserting into a consent summary. */
export function valuePaths(fields: SchemaField[], prefix = ''): string[] {
  return fields.flatMap((field) => {
    const name = trimSpace(field.name);
    if (!name) return [];
    const path = prefix ? `${prefix}.${name}` : name;
    return field.kind === 'object' ? valuePaths(field.children, path) : [path];
  });
}

const PLACEHOLDER = /\{\{\s*([^{}\s]+)\s*\}\}/g;
/** A placeholder, alone. */
const ONE_PLACEHOLDER = /^\{\{\s*[^{}\s]+\s*\}\}$/;

/** The consent summary rendered with sample values, and the problems the server would refuse it for. */
export interface ConsentPreview {
  text: string;
  unknownPaths: string[];
  /** The text in double braces that is not a placeholder, such as a name with a space in it. */
  malformed: string[];
  /** Whether the template holds a control or bidirectional formatting character. */
  unsafeText: boolean;
  /** Whether the template is longer than the server stores. */
  tooLong: boolean;
}

/** Reports whether the server would refuse a consent summary for any of the problems a preview names. */
export function isConsentPreviewInvalid(preview: ConsentPreview | null | undefined): boolean {
  return Boolean(
    preview &&
      (preview.unknownPaths.length > 0 || preview.malformed.length > 0 || preview.unsafeText || preview.tooLong),
  );
}

/** The longest consent template the server stores, in bytes of UTF-8. */
export const MAX_TEMPLATE_BYTES = 1000;

/**
 * Renders a consent template the way the server does, with each `{{path}}` replaced by a sample
 * value for its field, and `{{type}}` by the type identifier, which every detail carries.
 */
export function previewConsentTemplate(template: string, fields: SchemaField[], typeValue = ''): ConsentPreview {
  const samples = new Map<string, string>();
  collectSamples(fields, '', samples);
  const unknownPaths: string[] = [];
  const text = template.replace(PLACEHOLDER, (_match, path: string) => {
    if (path === TYPE_FIELD) return typeValue.trim() || path;
    const sample = samples.get(path);
    if (sample !== undefined) return sample;
    if (!unknownPaths.includes(path)) unknownPaths.push(path);
    return '';
  });
  return {
    text,
    unknownPaths,
    malformed: malformedPlaceholders(template),
    unsafeText: hasUnsafeText(template),
    tooLong: isTemplateTooLong(template),
  };
}

/**
 * Returns the text in double braces of a template that is not a placeholder, such as a name with a
 * space in it, which the server refuses.
 */
export function malformedPlaceholders(template: string): string[] {
  const found: string[] = [];
  for (const match of template.matchAll(/\{\{[^{}]*\}\}/g)) {
    if (!ONE_PLACEHOLDER.test(match[0]) && !found.includes(match[0])) found.push(match[0]);
  }
  return found;
}

/**
 * Returns the placeholder paths of a template that name a field a request can leave out, itself or
 * through an object it belongs to. The server leaves out a summary whose placeholder has no value.
 */
export function optionalPaths(template: string, fields: SchemaField[]): string[] {
  const optional = new Set<string>();
  const walk = (list: SchemaField[], prefix: string, inherited: boolean): void => {
    for (const field of list) {
      const path = prefix ? `${prefix}.${trimSpace(field.name)}` : trimSpace(field.name);
      const leftOut = inherited || !field.required;
      if (leftOut) optional.add(path);
      if (field.kind === 'object') walk(field.children, path, leftOut);
    }
  };
  walk(fields, '', false);
  return templatePaths(template).filter((path) => path !== TYPE_FIELD && optional.has(path));
}

/**
 * Reports whether a text holds a character the server refuses in a consent summary: a control character
 * other than a tab or a line break, or a bidirectional formatting character.
 */
export function hasUnsafeText(text: string): boolean {
  // eslint-disable-next-line no-control-regex
  return /[\x00-\x08\x0b\x0c\x0e-\x1f\x7f-\x9f\u061c\u200e\u200f\u202a-\u202e\u2066-\u2069]/.test(text);
}

/** Reports whether a template is longer than the server stores, as it is saved: trimmed. */
function isTemplateTooLong(template: string): boolean {
  return new TextEncoder().encode(template.trim()).length > MAX_TEMPLATE_BYTES;
}

/**
 * Returns the paths a schema declares, which a consent summary can name: each property, an object's
 * too, and the type member.
 */
export function schemaPaths(schema: unknown): string[] {
  const paths = [TYPE_FIELD];
  const walk = (node: unknown, prefix: string): void => {
    if (!isRecord(node) || !isRecord(node['properties'])) return;
    for (const [name, property] of Object.entries(node['properties'])) {
      const path = prefix ? `${prefix}.${name}` : name;
      if (!paths.includes(path)) paths.push(path);
      walk(property, path);
    }
  };
  walk(schema, '');
  return paths;
}

/** The largest schema the server stores, in bytes of its JSON as sent. */
export const MAX_SCHEMA_BYTES = 64 * 1024;

/** Reports whether a schema is larger than the server stores. */
export function isSchemaTooLarge(schema: unknown): boolean {
  return new TextEncoder().encode(JSON.stringify(schema)).length > MAX_SCHEMA_BYTES;
}

/** A problem the server would refuse in a schema written as JSON, keyed for translation. */
export interface SchemaIssue {
  /** The dotted path of the property it is about, or empty for the schema itself. */
  path: string;
  /** The translation key, under `authorizationDetailTypes:`. */
  messageKey: string;
  /** The English message. */
  defaultMessage: string;
  /** The values the message names. */
  values?: Record<string, string>;
}

/** The JSON Schema types the server allows; integer and null are refused. */
const JSON_SCHEMA_TYPES = ['array', 'boolean', 'number', 'object', 'string'];

/** The keywords every schema may use, and those each type adds, as the server allows them. */
const COMMON_KEYWORDS = ['type', 'title', 'description', 'enum', 'const'];
const TYPE_KEYWORDS: Record<string, string[]> = {
  object: ['properties', 'required'],
  string: ['minLength', 'maxLength', 'pattern', 'format'],
  number: ['minimum', 'maximum', 'exclusiveMinimum', 'exclusiveMaximum', 'multipleOf'],
  boolean: [],
  array: ['items', 'minItems', 'maxItems', 'uniqueItems'],
};

/** The keywords whose value the server reads as a count. */
const COUNT_KEYWORDS = ['minLength', 'maxLength', 'minItems', 'maxItems'];

/** Reports whether a value holds a number no double holds, as JSON.parse reads one beyond range. */
function hasHugeNumber(value: unknown): boolean {
  if (typeof value === 'number') return !Number.isFinite(value);
  if (Array.isArray(value)) return value.some(hasHugeNumber);
  return isRecord(value) && Object.values(value).some(hasHugeNumber);
}

/** Reports whether two values of an enum are the same, numbers compared by value. */
function sameValue(a: unknown, b: unknown): boolean {
  return JSON.stringify(sortKeys(a)) === JSON.stringify(sortKeys(b));
}

function checkSchemaNode(
  node: Record<string, unknown>,
  path: string,
  place: {root: boolean; property: boolean; inItems: boolean},
  issues: SchemaIssue[],
): void {
  const issue = (messageKey: string, defaultMessage: string, values?: Record<string, string>): void => {
    issues.push({path, messageKey: `schema.errors.${messageKey}`, defaultMessage, ...(values && {values})});
  };
  const type = node['type'];
  if (type === 'integer') {
    issue('integerType', 'Use number instead of integer, with a multipleOf of 1 for whole numbers.');
    return;
  }
  if (type === 'null') {
    issue('nullType', 'null is not supported. Set type to string, number, boolean, object, or array.');
    return;
  }
  if (type !== undefined && (typeof type !== 'string' || !JSON_SCHEMA_TYPES.includes(type))) {
    issue('type', 'Set type to one JSON Schema type, such as string or object.');
    return;
  }
  if (type === 'array' && node['items'] === undefined) issue('items', 'Give the list an items schema.');
  const allowed = [
    ...COMMON_KEYWORDS,
    ...(place.root ? ['$schema'] : []),
    ...(place.property ? [POLICY_KEYWORD] : []),
    ...(typeof type === 'string' ? TYPE_KEYWORDS[type] : []),
  ];
  for (const keyword of Object.keys(node).sort()) {
    if (!allowed.includes(keyword)) issue('keyword', '{{keyword}} is not supported here.', {keyword});
  }
  for (const [keyword, value] of Object.entries(node)) {
    if (keyword !== 'properties' && keyword !== 'items' && hasHugeNumber(value)) {
      issue('number', '{{keyword}} holds a number that is out of range.', {keyword});
    }
    if (COUNT_KEYWORDS.includes(keyword) && allowed.includes(keyword)) {
      if (typeof value !== 'number' || !Number.isInteger(value) || value < 0 || value > MAX_SCHEMA_COUNT) {
        issue('count', '{{keyword}} must be a whole number up to 2147483647.', {keyword});
      }
    }
  }
  if (node['title'] === '') issue('title', 'Remove the empty title or give it text.');
  if (type === undefined && !place.root && node['const'] === undefined && node['enum'] === undefined) {
    issue('unconstrained', 'Give it a type, const, or enum.');
  }
  const values = node['enum'];
  if (
    values !== undefined &&
    (!Array.isArray(values) ||
      values.length === 0 ||
      values.some((value) => typeof value === 'string' && value !== trimSpace(value)) ||
      values.some((value, index) => values.slice(index + 1).some((other) => sameValue(value, other))))
  ) {
    issue('enum', 'List each allowed value once, with no space around text.');
  }
  const format = node['format'];
  if (format !== undefined && format !== 'date' && format !== 'date-time') {
    issue('format', 'Use date or date-time as the format.');
  }
  const properties = isRecord(node['properties']) ? node['properties'] : {};
  const required = node['required'];
  if (required !== undefined) {
    if (!Array.isArray(required) || !required.every((name) => typeof name === 'string')) {
      issue('required', 'List the required properties by name.');
    } else {
      for (const name of new Set(required)) {
        if (!(name in properties) && !(place.root && name === TYPE_FIELD)) {
          issue('requiredUndeclared', '{{name}} is required but not declared.', {name});
        }
      }
    }
  }
  if (node['properties'] !== undefined && !isRecord(node['properties'])) {
    issue('properties', 'List the properties as an object.');
  }
  for (const name of Object.keys(properties).sort()) {
    const child = path ? `${path}.${name}` : name;
    const member = properties[name];
    const childIssue = (messageKey: string, defaultMessage: string): void => {
      issues.push({path: child, messageKey, defaultMessage});
    };
    if (!trimSpace(name)) {
      childIssue('fields.errors.nameRequired', 'Enter a field name.');
    } else if (name !== trimSpace(name)) {
      childIssue('fields.errors.nameSpaces', 'A field name cannot start or end with a space.');
    } else if (name.includes('.')) {
      childIssue('fields.errors.nameDot', 'A field name cannot contain a dot.');
    }
    if (!isRecord(member)) {
      childIssue('schema.errors.schemaObject', 'Write it as a schema object.');
    } else {
      // A policy is read only from a property, so none can sit under a list's items.
      checkSchemaNode(member, child, {root: false, property: !place.inItems, inItems: place.inItems}, issues);
    }
  }
  const items = node['items'];
  if (items !== undefined) {
    if (isRecord(items)) checkSchemaNode(items, path, {root: false, property: false, inItems: true}, issues);
    else issue('schemaObject', 'Write it as a schema object.');
  }
}

/**
 * Checks a schema written as JSON against the keywords and values the server allows, so a schema the
 * composer cannot show is not refused only once it is saved. The server checks the field policies too.
 */
export function schemaKeywordIssues(schema: Record<string, unknown>): SchemaIssue[] {
  const issues: SchemaIssue[] = [];
  if (schema['type'] !== 'object') {
    issues.push({path: '', messageKey: 'schema.errors.object', defaultMessage: 'The schema must describe an object.'});
  }
  const dialect = schema['$schema'];
  if (dialect !== undefined && dialect !== 'https://json-schema.org/draft/2020-12/schema') {
    issues.push({
      path: '',
      messageKey: 'schema.errors.dialect',
      defaultMessage: 'Use JSON Schema draft 2020-12, or leave out $schema.',
    });
  }
  checkSchemaNode(schema, '', {root: true, property: false, inItems: false}, issues);
  const typeMember = isRecord(schema['properties']) ? schema['properties'][TYPE_FIELD] : undefined;
  if (
    isRecord(typeMember) &&
    ((typeMember['type'] !== undefined && typeMember['type'] !== 'string') ||
      typeMember['format'] !== undefined ||
      [...(Array.isArray(typeMember['enum']) ? (typeMember['enum'] as unknown[]) : []), typeMember['const']].some(
        (value) => value !== undefined && typeof value !== 'string',
      ))
  ) {
    issues.push({
      path: TYPE_FIELD,
      messageKey: 'fields.errors.typeKind',
      defaultMessage: 'The type identifies the request, so it must be plain text.',
    });
  }
  return issues;
}

/**
 * Checks a consent template against the paths a schema declares, for a schema the builder cannot show.
 * The text is the template as written, since there are no fields to sample.
 */
export function checkConsentTemplate(template: string, paths: string[]): ConsentPreview {
  return {
    text: template,
    unknownPaths: templatePaths(template).filter((path) => !paths.includes(path)),
    malformed: malformedPlaceholders(template),
    unsafeText: hasUnsafeText(template),
    tooLong: isTemplateTooLong(template),
  };
}

/** Returns the paths the placeholders of a consent template name, each once, in order. */
export function templatePaths(template: string): string[] {
  const paths: string[] = [];
  for (const match of template.matchAll(PLACEHOLDER)) {
    if (!paths.includes(match[1])) paths.push(match[1]);
  }
  return paths;
}

/** Whether a path is the given path or a path inside it. */
function isWithin(path: string, base: string): boolean {
  return path === base || path.startsWith(`${base}.`);
}

/** Returns the placeholder paths of a template that name the field at the path or a field inside it. */
export function pathsReferencing(template: string, path: string): string[] {
  if (!path) return [];
  return templatePaths(template).filter((candidate) => isWithin(candidate, path));
}

/**
 * Rewrites the placeholders of a template that name the field at one path, or a field inside it, to
 * name the same field at another path, as when the field is renamed.
 */
export function renameTemplatePaths(template: string, from: string, to: string): string {
  if (!from || !to) return template;
  return template.replace(PLACEHOLDER, (match, path: string) =>
    isWithin(path, from) ? `{{${to}${path.slice(from.length)}}}` : match,
  );
}

/** Reports whether a value is an acceptable type value: printable ASCII without spaces, at most 512 characters. */
export function isValidTypeValue(value: string): boolean {
  return value.length > 0 && value.length <= 512 && /^[\x21-\x7e]+$/.test(value);
}

/**
 * Reports whether an endpoint is HTTPS, or HTTP on a loopback host for local development, with no
 * credentials in it and a port, if any, from 1 to 65535.
 */
export function isValidProcessorEndpoint(endpoint: string): boolean {
  return processorEndpointProblem(endpoint) === null;
}

/**
 * Why the server refuses a processor endpoint: it is not a URL it can read, it is not HTTPS or HTTP on
 * a loopback host, it has a user name or password in it, or its port is not from 1 to 65535.
 */
export type ProcessorEndpointProblem = 'invalid' | 'scheme' | 'credentials' | 'port';

/** Returns why the server refuses a processor endpoint, or null when it accepts it. */
export function processorEndpointProblem(endpoint: string): ProcessorEndpointProblem | null {
  // The server parses the endpoint with Go's URL parser, which refuses a control character, a
  // malformed percent-escape outside the query, and a host character that would need escaping.
  // eslint-disable-next-line no-control-regex
  if (/[\x00-\x1f\x7f]/.test(endpoint)) return 'invalid';
  const scheme = /^([a-z][a-z0-9+.-]*):/i.exec(endpoint)?.[1].toLowerCase();
  if (!scheme) return 'invalid';
  if (scheme !== 'https' && scheme !== 'http') return 'scheme';
  // The server needs an authority, so a URL such as https:host is refused, and reads the host as
  // written, so a form the browser rewrites, such as 127.1, is not taken for the address it names.
  const parts = /^[a-z]+:\/\/([^/?#]*)([^?#]*)(?:\?[^#]*)?(?:#(.*))?$/i.exec(endpoint);
  if (!parts) return 'invalid';
  const [, authority, path, fragment = ''] = parts;
  // Credentials in the URL would be stored and shown with the endpoint, so the server refuses them.
  if (authority.includes('@')) return 'credentials';
  const hostPort = /^(\[[^\]]*\]|[^:]*)(?::(.*))?$/.exec(authority);
  const host = hostPort?.[1] ?? '';
  const port = hostPort?.[2] ?? '';
  if (port !== '' && (!/^\d+$/.test(port) || Number(port) < 1 || Number(port) > 65535)) return 'port';
  // The browser's URL parser lets some of the host characters the server refuses through, such as a space.
  if (!/^(\[[^\]]+\]|[A-Za-z0-9\-._~!$&'()*+,;=\u0080-￿]+)$/.test(host)) return 'invalid';
  if (/%(?![0-9a-f]{2})/i.test(path) || /%(?![0-9a-f]{2})/i.test(fragment)) return 'invalid';
  let url: URL;
  try {
    url = new URL(endpoint);
  } catch {
    return 'invalid';
  }
  if (url.protocol === 'https:') return null;
  return isLoopbackHost(host, url.hostname) ? null : 'scheme';
}

/** A dotted quad in 127.0.0.0/8, each part a decimal number without leading zeros. */
const LOOPBACK_IPV4 = /^127(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/;
/** ::1, or an IPv4-mapped address in 127.0.0.0/8, as the URL parser writes an IPv6 host. */
const LOOPBACK_IPV6 = /^\[(::1|::ffff:7f[0-9a-f]{2}:[0-9a-f]{1,4})\]$/;

/**
 * Reports whether a host, as written, names the loopback interface: localhost, a dotted quad in
 * 127.0.0.0/8, or an IPv6 loopback address in any spelling, which the URL parser normalizes.
 */
function isLoopbackHost(written: string, hostname: string): boolean {
  if (written.toLowerCase() === 'localhost') return true;
  if (written.startsWith('[')) return LOOPBACK_IPV6.test(hostname);
  return LOOPBACK_IPV4.test(written);
}
