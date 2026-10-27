// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';
import AppBuilderPage from './AppBuilderPage';

/**
 * Builds the Admin App, where the admins of an organization unit manage its users, groups, roles
 * and agents.
 *
 * @public
 */
export default function AdminAppBuilderPage(): JSX.Element {
  return <AppBuilderPage type="ADMIN" />;
}
