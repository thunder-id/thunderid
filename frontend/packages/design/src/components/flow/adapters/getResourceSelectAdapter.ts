// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {EmbeddedFlowComponentType} from '@thunderid/react';
import type {ComponentType} from 'react';
import AuthFlowSelectAdapter from './AuthFlowSelectAdapter';
import type {ResourceSelectAdapterProps} from './ResourceSelectAdapter';
import UserSelectAdapter from './UserSelectAdapter';
import UserTypeSelectAdapter from './UserTypeSelectAdapter';

/**
 * The adapter for a flow component type that picks a management resource, or `undefined` for any other
 * type. These pickers load with the signed-in user's token, so they belong on signed-in pages.
 */
export default function getResourceSelectAdapter(type: string): ComponentType<ResourceSelectAdapterProps> | undefined {
  switch (type as EmbeddedFlowComponentType) {
    case EmbeddedFlowComponentType.UserSelect:
      return UserSelectAdapter;
    case EmbeddedFlowComponentType.AuthFlowSelect:
      return AuthFlowSelectAdapter;
    case EmbeddedFlowComponentType.UserTypeSelect:
      return UserTypeSelectAdapter;
    default:
      return undefined;
  }
}
