// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {useTranslation} from 'react-i18next';
import {humanize} from './groupByResource';
import type {ValueKind, ValueReference} from '../models/deploy';

type Translate = ReturnType<typeof useTranslation>['t'];

/** One value a resource uses, as it refers to it. */
export interface ResourceValue {
  kind: ValueKind;
  name: string;
  /** The field the value stands in, such as `clientSecret`. */
  field?: string;
  /** Whether the value is a list, entered one item per line. */
  list: boolean;
}

/** A resource and the values it uses, in the order it refers to them. */
export interface ResourceValues {
  key: string;
  resourceType: string;
  resourceId?: string;
  resourceName?: string;
  values: ResourceValue[];
}

/** What a field is called for a person: `clientSecret` reads "Client secret". */
export function fieldLabel(t: Translate, field: string | undefined): string {
  return field
    ? t(`deploy:values.fields.${field}`, {defaultValue: humanize(field)})
    : t('deploy:values.value', 'Value');
}

/**
 * Groups the places values are referred to by the resource that refers to them. A value a resource refers
 * to more than once is listed once for it.
 */
export function valuesByResource(references: ValueReference[]): ResourceValues[] {
  const resources = new Map<string, ResourceValues>();
  references.forEach((reference: ValueReference) => {
    const key = `${reference.resourceType}/${reference.resourceId ?? reference.resourceName ?? ''}`;
    const resource = resources.get(key) ?? {
      key,
      resourceType: reference.resourceType,
      resourceId: reference.resourceId,
      resourceName: reference.resourceName,
      values: [],
    };
    const same = resource.values.find(
      (value: ResourceValue) => value.kind === reference.kind && value.name === reference.name,
    );
    if (same) same.list = same.list || reference.list === true;
    else
      resource.values.push({
        kind: reference.kind,
        name: reference.name,
        field: reference.field,
        list: reference.list === true,
      });
    resources.set(key, resource);
  });
  return [...resources.values()];
}
