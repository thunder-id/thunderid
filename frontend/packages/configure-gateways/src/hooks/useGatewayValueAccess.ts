// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCapabilitiesContext} from '@thunderid/contexts';
import type {ValueKind} from '../models/deploy';

export interface GatewayValueAccess {
  /** Whether the admin may perform `action` on a gateway's values of `kind`. */
  canValue: (kind: ValueKind, action?: string) => boolean;
  /** Whether the admin may view both the variables and the secrets. */
  canViewValues: boolean;
  /** Whether the admin may set both the variables and the secrets. */
  canSetValues: boolean;
}

/** What the admin may do with a gateway's variables and secrets. Each kind is also its resource's name. */
export default function useGatewayValueAccess(): GatewayValueAccess {
  const {can} = useCapabilitiesContext();
  const canValue = (kind: ValueKind, action?: string): boolean => can(kind, action);
  const canBoth = (action?: string): boolean => canValue('variable', action) && canValue('secret', action);

  return {canValue, canViewValues: canBoth(), canSetValues: canBoth('update')};
}
