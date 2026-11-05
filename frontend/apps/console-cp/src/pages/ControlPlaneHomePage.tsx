// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {EnvironmentsOverview} from '@thunderid/configure-gateways';
import {HomePage} from '@thunderid/configure-onboarding';
import type {JSX} from 'react';

/** The Control Plane's home: the greeting, then where the configuration is deployed, and nothing else. */
export default function ControlPlaneHomePage(): JSX.Element {
  return <HomePage lead={<EnvironmentsOverview />} showGettingStarted={false} />;
}
