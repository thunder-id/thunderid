// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCallback, useContext} from 'react';
import {useTranslation} from 'react-i18next';
import ResourceNamesContext from '../contexts/ResourceNames';
import {humanize} from '../utils/groupByResource';

/** What a change, entry or resource is named by: its type, its id, and its own name when it has one. */
export interface Named {
  key: string;
  resourceType: string;
  id?: string;
  name?: string;
}

/**
 * What a row calls a resource: its own name, else the name the environments' maps give it, else its kind
 * and the start of its id, never a bare id.
 */
export default function useChangeName(): (named: Named) => string {
  const {t} = useTranslation('deploy');
  const names = useContext(ResourceNamesContext);
  return useCallback(
    (named: Named): string => {
      if (named.name) return named.name;
      const known = names.get(named.key) ?? (named.id ? names.get(`${named.resourceType}/${named.id}`) : undefined);
      if (known && known !== named.id) return known;
      const type = t(`changes.types.${named.resourceType}`, humanize(named.resourceType));
      const id = named.id ?? named.key.split('/').pop() ?? named.key;
      return t('changes.unnamed', '{{type}} {{id}}', {type, id: id.slice(0, 8)});
    },
    [names, t],
  );
}
