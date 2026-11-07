// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useIdentityProviders} from '@thunderid/configure-connections';
import type {ConnectionInstance} from '@thunderid/react';
import {useMemo} from 'react';

/**
 * The connections a connection variable can take, in the form the variable components read them. Only identity
 * providers qualify, which is also the one category a variable can name.
 *
 * @returns The connections.
 */
export default function useFlowVariableConnections(): Pick<
  ConnectionInstance,
  'categories' | 'id' | 'name' | 'type'
>[] {
  const {data: identityProviders} = useIdentityProviders();

  return useMemo(
    () =>
      (identityProviders ?? []).map((provider: {id: string; name: string; type: string}) => ({
        categories: ['identity-provider' as const],
        id: provider.id,
        name: provider.name,
        type: provider.type.toLowerCase() as ConnectionInstance['type'],
      })),
    [identityProviders],
  );
}
