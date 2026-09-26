// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {HeidiIcon, LissiIcon} from '@thunderid/components';
import type {ComponentType} from 'react';

/** A known OID4VCI wallet vendor and the fixed client id it presents. */
export interface WalletVendor {
  id: string;
  label: string;
  clientId: string;
  redirectUri?: string;
  /** The vendor's brand mark, shown on its picker card. Absent for "Custom". */
  logo?: ComponentType<{height?: number}>;
  /** The card background the logo was designed against ("Custom" uses the theme default). */
  cardBackground?: string;
  /** The color the logo mark renders in against `cardBackground`. */
  logoColor?: string;
}

/** The "Custom" option lets the admin enter an arbitrary client id. */
export const CUSTOM_WALLET_VENDOR = 'custom';

/**
 * Known wallets with their fixed, vendor-assigned client ids. Selecting one
 * pre-fills the client id (and a default redirect URI where known); "Custom"
 * lets the admin type the client id for any other OID4VCI wallet.
 */
export const WALLET_VENDORS: WalletVendor[] = [
  {
    id: 'heidi',
    label: 'Heidi',
    clientId: 'c3ce7a6c-2bbb-4abe-909c-41bc9463d3c5',
    redirectUri: 'ch.ubique.funke://issuance',
    logo: HeidiIcon,
    cardBackground: '#FFFFFF',
    logoColor: '#0A0A0A',
  },
  {
    id: 'lissi',
    label: 'Lissi',
    clientId: '9c481dc3-2ad0-4fe0-881d-c32ad02fe0fc',
    redirectUri: 'https://oob.lissi.io/vci-cb',
    logo: LissiIcon,
    cardBackground: '#0A0A0A',
    logoColor: '#FFFFFF',
  },
  {id: CUSTOM_WALLET_VENDOR, label: 'Custom', clientId: ''},
];
