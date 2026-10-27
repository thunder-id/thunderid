// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {House, ShieldCheck, UserRound} from '@wso2/oxygen-ui-icons-react';
import type {ReactNode} from 'react';
import type {AccountDestination} from '../configs/RouteConfig';

/**
 * The icon each destination shows in the navigation and on the home page tiles.
 */
const DESTINATION_ICONS: Record<AccountDestination, ReactNode> = {
  home: <House size={20} strokeWidth={1.75} />,
  profile: <UserRound size={20} strokeWidth={1.75} />,
  security: <ShieldCheck size={20} strokeWidth={1.75} />,
};

export default DESTINATION_ICONS;
