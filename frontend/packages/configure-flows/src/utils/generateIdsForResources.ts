// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import generateResourceId from './generateResourceId';
import type {JsonValue} from '../models/json';

const replaceIds = (obj: JsonValue, matcher: string): JsonValue => {
  if (Array.isArray(obj)) {
    return obj.map((item) => replaceIds(item, matcher));
  }
  if (typeof obj === 'object' && obj !== null) {
    const objAsRecord = obj;

    return Object.fromEntries(
      Object.entries(objAsRecord).map(([key, value]) => {
        if (key === 'id' && value === `{{${matcher}}}`) {
          const typeValue = objAsRecord['type'];
          const type = typeof typeValue === 'string' ? typeValue.toLowerCase() : 'component';

          return [key, generateResourceId(type)];
        }

        return [key, replaceIds(value, matcher)];
      }),
    );
  }

  return obj;
};

const generateIdsForResources = <T = unknown>(resources: T, matcher = 'ID'): T =>
  replaceIds(resources as JsonValue, matcher) as T;

export default generateIdsForResources;
