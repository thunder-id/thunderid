// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useFetchUserTypes} from '@thunderid/react';
import type {JSX} from 'react';
import ResourceSelectAdapter, {type ResourceSelectAdapterProps} from './ResourceSelectAdapter';

/** Picks a user type. Submits and shows the user type's handle. */
export default function UserTypeSelectAdapter(props: ResourceSelectAdapterProps): JSX.Element | null {
  const fetchUserTypes = useFetchUserTypes();

  return <ResourceSelectAdapter {...props} fetchPage={fetchUserTypes} />;
}
