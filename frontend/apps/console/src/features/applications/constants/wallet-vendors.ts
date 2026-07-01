/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/** A known OID4VCI wallet vendor and the fixed client id it presents. */
export interface WalletVendor {
  id: string;
  label: string;
  clientId: string;
  redirectUri?: string;
}

/** The "Custom" option lets the admin enter an arbitrary client id. */
export const CUSTOM_WALLET_VENDOR = 'custom';

/**
 * Known wallets with their fixed, vendor-assigned client ids. Selecting one
 * pre-fills the client id (and a default redirect URI where known); "Custom"
 * lets the admin type the client id for any other OID4VCI wallet.
 */
export const WALLET_VENDORS: WalletVendor[] = [
  {id: CUSTOM_WALLET_VENDOR, label: 'Custom', clientId: ''},
  {
    id: 'heidi',
    label: 'Heidi',
    clientId: 'c3ce7a6c-2bbb-4abe-909c-41bc9463d3c5',
    redirectUri: 'ch.ubique.funke://issuance',
  },
  {
    id: 'lissi',
    label: 'Lissi',
    clientId: '9c481dc3-2ad0-4fe0-881d-c32ad02fe0fc',
    redirectUri: 'https://oob.lissi.io/vci-cb',
  },
];
