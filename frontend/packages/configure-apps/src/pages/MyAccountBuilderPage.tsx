// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';
import AppBuilderPage from './AppBuilderPage';

/**
 * Builds MyAccount, the Account App where the people who sign in to your apps manage their own
 * profile and credentials.
 *
 * @public
 */
export default function MyAccountBuilderPage(): JSX.Element {
  return <AppBuilderPage type="ACCOUNT" />;
}
