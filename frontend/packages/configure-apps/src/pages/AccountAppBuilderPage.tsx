// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';
import AppBuilderPage from './AppBuilderPage';

/**
 * Builds the Account App, where the people who sign in to your apps manage their own profile and
 * credentials.
 *
 * @public
 */
export default function AccountAppBuilderPage(): JSX.Element {
  return <AppBuilderPage type="ACCOUNT" />;
}
