// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes, useAdministrationOperation} from '@thunderid/contexts';
import {type JSX} from 'react';
import UserAddFormPage from './UserAddFormPage';
import UserAddFlowPage from './UserAddPage';

/**
 * Chooses how a user is added, by how the `users` `create` operation runs.
 *
 * Native creation is a plain write, so it gets a form that posts to the users API. Otherwise the
 * onboarding flow runs, where adding a user is a journey an operator can shape.
 */
export default function UserAddRoute(): JSX.Element {
  const mode = useAdministrationOperation('users', 'create');

  return mode === AdministrationModes.NATIVE ? <UserAddFormPage /> : <UserAddFlowPage />;
}
