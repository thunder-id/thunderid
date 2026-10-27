// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Bot, ShieldCheck, UserRound, UsersRound} from '@wso2/oxygen-ui-icons-react';
import type {ReactNode} from 'react';
import type {ManagedResource} from '../configs/RouteConfig';

/**
 * The icon each managed resource shows in the navigation.
 */
const RESOURCE_ICONS: Record<ManagedResource, ReactNode> = {
  agents: <Bot size={18} strokeWidth={1.75} />,
  groups: <UsersRound size={18} strokeWidth={1.75} />,
  roles: <ShieldCheck size={18} strokeWidth={1.75} />,
  users: <UserRound size={18} strokeWidth={1.75} />,
};

export default RESOURCE_ICONS;
