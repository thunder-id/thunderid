// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {type JSX} from 'react';
import UserAddFormPage from './UserAddFormPage';
import UserAddFlowPage from './UserAddPage';

/**
 * Chooses how a user is added, by plane.
 *
 * A Control Plane executes no flows. It holds configuration and serves no runtime, so there is no
 * /flow/execute for an embedded onboarding flow to reach and the request 404s. Authoring a user
 * there is a plain write, so it gets a form.
 *
 * A Data Plane runs the flow, where onboarding is a journey an operator can shape: extra steps,
 * verification, whatever the flow says. That is worth keeping where it can actually run.
 */
export default function UserAddRoute(): JSX.Element {
  const {config} = useConfig();

  return config.plane === 'cp' ? <UserAddFormPage /> : <UserAddFlowPage />;
}
