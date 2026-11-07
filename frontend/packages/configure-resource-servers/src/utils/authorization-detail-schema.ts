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
  POLICY_KEYWORD,
  'items',
  'properties',
  'required',
  'minimum',
  'maximum',
  'maxLength',
  'minItems',
];
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
  const names = new Set(fields.map((field) => field.name.trim()));
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
  // An empty list allows no value, which the builder cannot show.
  if (!Array.isArray(values) || values.length === 0 || !values.every((value) => typeof value === itemKind)) {
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
    if (title !== undefined && typeof title !== 'string') return null;
    const policy = parsePolicy(property[POLICY_KEYWORD]);
    if (!policy) return null;
    const kind = parseKind(property);
    if (!kind) return null;
    // An object only groups fields; a change rule on it is one the builder cannot show.
    if (kind.kind === 'object' && (policy.modification ?? 'none') !== 'none') return null;
    // A number is limited by the schema's own minimum and maximum, and other ordered values by their policy.
    const ownLimits = property['minimum'] !== undefined || property['maximum'] !== undefined;
    if (hasSchemaLimits(kind.kind)) {
      if (policy.minimum !== undefined || policy.maximum !== undefined) return null;
      for (const limit of ['minimum', 'maximum'] as const) {
        if (property[limit] === undefined) continue;
        if (typeof property[limit] !== 'number') return null;
        policy[limit] = String(property[limit]);
      }
    } else if (ownLimits) {
      return null;
    }
    const maxLength = property['maxLength'];
    if (
      maxLength !== undefined &&
      (kind.kind !== 'string' || !Number.isInteger(maxLength) || (maxLength as number) < 0)
    ) {
      return null;
    }
    const minItems = property['minItems'];
    if (minItems !== undefined && (kind.kind !== 'array' || !Number.isInteger(minItems) || (minItems as number) < 0)) {
      return null;
    }
    const children = kind.kind === 'object' ? parseProperties(property['properties'], property['required']) : [];
    if (!children) return null;
    fields.push({
      key: newFieldKey(),
      name,
      title: title ?? '',
      kind: kind.kind,
      itemKind: kind.itemKind,
      ...(kind.allowedValues?.length && {allowedValues: kind.allowedValues}),
      required: requiredNames.includes(name),
      policy,
      ...(maxLength !== undefined && {maxLength: String(maxLength as number)}),
      ...(minItems !== undefined && {minItems: String(minItems as number)}),
      children,
    });
  }
  return fields;
}

/**
 * Reads a schema into builder fields. Returns null when the schema uses anything the builder cannot
 * show, such as combinators, patterns, or a list of allowed values on anything but a list's items.
 */
export function schemaToFields(schema: unknown): SchemaField[] | null {
  if (!isRecord(schema) || schema['type'] !== 'object' || !onlyKeys(schema, ['type', 'properties', 'required'])) {
    return null;
  }
  return parseProperties(schema['properties'], schema['required']);
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

/**
 * Writes the standard keywords that constrain a field's value: a number's range, a text's length, and
 * the fewest items a list holds.
 */
function writeConstraints(field: SchemaField, property: Record<string, unknown>): void {
  if (hasSchemaLimits(field.kind)) {
    const minimum = toLimit(field.policy.minimum, true);
    const maximum = toLimit(field.policy.maximum, true);
    if (minimum !== undefined) property['minimum'] = minimum;
    if (maximum !== undefined) property['maximum'] = maximum;
  }
  const maxLength = field.maxLength?.trim();
  if (field.kind === 'string' && maxLength) {
    const parsed = Number(maxLength);
    property['maxLength'] = Number.isFinite(parsed) ? parsed : maxLength;
  }
  const minItems = field.minItems?.trim();
  if (field.kind === 'array' && minItems) {
    const parsed = Number(minItems);
    property['minItems'] = Number.isFinite(parsed) ? parsed : minItems;
  }
}

/** Returns a list field's allowed values, trimmed and without blanks. */
export function allowedValuesOf(field: SchemaField): string[] {
  return (field.allowedValues ?? []).map((value) => value.trim()).filter(Boolean);
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
    if (field.title.trim()) property['title'] = field.title;
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
    setOwnProperty(properties, field.name.trim(), property);
    if (field.required) required.push(field.name.trim());
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
// As the server parses RFC 3339: an upper case T and Z, and no leap second.
const DATE_TIME = /^(\d{4})-(\d{2})-(\d{2})T([01]\d|2[0-3]):[0-5]\d:[0-5]\d(\.\d+)?(Z|[+-]([01]\d|2[0-3]):[0-5]\d)$/;

/** Reports whether a value is a JSON number the server holds as a finite double. */
function isFiniteNumber(value: string): boolean {
  return NUMBER.test(value) && Number.isFinite(Number(value));
}

/** Reports whether a year, month and day name a day that exists. */
function isCalendarDate(year: string, month: string, day: string): boolean {
  const date = new Date(Date.UTC(Number(year), Number(month) - 1, Number(day)));
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

/** Returns a valid limit as a number that orders the way its value type does. */
function limitValue(value: string, valueType: ValueType): number {
  return valueType === 'number' ? Number(value) : Date.parse(value);
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

function collectIssues(fields: SchemaField[], atRoot: boolean, inSupplied: boolean, issues: FieldIssue[]): void {
  const seen = new Set<string>();
  for (const field of fields) {
    const name = field.name.trim();
    const issue = (messageKey: string, defaultMessage: string): void => {
      issues.push({fieldKey: field.key, messageKey, defaultMessage});
    };
    if (!name) {
      issue('fields.errors.nameRequired', 'Enter a field name.');
    } else if (name !== field.name) {
      issue('fields.errors.nameSpaces', 'A field name cannot start or end with a space.');
    } else if (name.includes('.')) {
      issue('fields.errors.nameDot', 'A field name cannot contain a dot.');
    } else if (/\s/.test(name)) {
      // A summary placeholder cannot name a field with a space in it.
      issue('fields.errors.nameInnerSpace', 'A field name cannot contain a space.');
    } else if (seen.has(name)) {
      issue('fields.errors.nameDuplicate', 'Another field at this level has the same name.');
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
      const limits = [policy.minimum, policy.maximum]
        .map((limit) => (limit === undefined ? '' : String(limit).trim()))
        .filter(Boolean);
      if (limits.some((limit) => !isValidLimit(limit, valueType))) {
        issue('fields.errors.limitInvalid', 'Enter the minimum and maximum as values of this type.');
      } else if (limits.length === 2 && limitValue(limits[0], valueType) > limitValue(limits[1], valueType)) {
        issue('fields.errors.limitOrder', 'The minimum cannot be greater than the maximum.');
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
    const allowed = field.kind === 'array' ? allowedValuesOf(field) : [];
    const written = field.kind === 'array' ? (field.allowedValues ?? []).filter((value) => value.trim()) : [];
    if (written.some((value) => value !== value.trim())) {
      issue('fields.errors.allowedValuesSpaces', 'An allowed value cannot start or end with a space.');
    } else if (field.itemKind === 'number' && allowed.some((value) => !isFiniteNumber(value))) {
      issue('fields.errors.allowedValuesInvalid', 'Enter each allowed value as a number.');
    } else if (
      new Set<string | number>(field.itemKind === 'number' ? allowed.map(Number) : allowed).size !== allowed.length
    ) {
      // Numbers are compared by value, as the server compares them, so 1 and 1.0 are the same.
      issue('fields.errors.allowedValuesDuplicate', 'Each allowed value can appear only once.');
    }
    if (field.kind === 'object') {
      collectIssues(field.children, false, inSupplied || policy.suppliedBy === 'processor', issues);
    }
  }
}

/** Checks builder fields against the rules the server applies to a schema and its policies. */
export function validateFields(fields: SchemaField[]): FieldIssue[] {
  const issues: FieldIssue[] = [];
  collectIssues(fields, true, false, issues);
  return issues;
}

/** The number a sample starts from, moved inside the field's limits when it is outside them. */
const SAMPLE_NUMBER = 100;

/** Returns a sample number for a field, within its minimum and maximum. */
function sampleNumber(field: SchemaField): number {
  const limit = (value?: number | string): number | undefined => {
    const text = value === undefined ? '' : String(value).trim();
    return isFiniteNumber(text) ? Number(text) : undefined;
  };
  const minimum = limit(field.policy.minimum);
  const maximum = limit(field.policy.maximum);
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
  if (maximum !== undefined && limitValue(sample, valueType) > limitValue(maximum, valueType)) return maximum;
  if (minimum !== undefined && limitValue(sample, valueType) < limitValue(minimum, valueType)) return minimum;
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
        const name = child.name.trim();
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
    const path = prefix ? `${prefix}.${field.name.trim()}` : field.name.trim();
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
    const name = field.name.trim();
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
      const path = prefix ? `${prefix}.${field.name.trim()}` : field.name.trim();
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
      values.some((value) => typeof value === 'string' && value !== value.trim()) ||
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
    if (!name.trim()) {
      childIssue('fields.errors.nameRequired', 'Enter a field name.');
    } else if (name !== name.trim()) {
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
