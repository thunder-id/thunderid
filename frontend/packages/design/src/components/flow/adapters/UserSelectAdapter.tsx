// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useFetchUsers} from '@thunderid/react';
import type {JSX} from 'react';
import ResourceSelectAdapter, {type ResourceSelectAdapterProps} from './ResourceSelectAdapter';

/** Picks a user. Submits the user's id, labelled by display name, then username, then email. */
export default function UserSelectAdapter(props: ResourceSelectAdapterProps): JSX.Element | null {
  const fetchUsers = useFetchUsers();

  return <ResourceSelectAdapter {...props} fetchPage={fetchUsers} />;
}
