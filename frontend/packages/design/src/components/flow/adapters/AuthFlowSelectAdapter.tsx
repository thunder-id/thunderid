// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useFetchAuthFlows} from '@thunderid/react';
import type {JSX} from 'react';
import ResourceSelectAdapter, {type ResourceSelectAdapterProps} from './ResourceSelectAdapter';

/** Picks an authentication flow. Submits the flow's id, labelled by its name, then its handle. */
export default function AuthFlowSelectAdapter(props: ResourceSelectAdapterProps): JSX.Element | null {
  const fetchAuthFlows = useFetchAuthFlows();

  return <ResourceSelectAdapter {...props} fetchPage={fetchAuthFlows} />;
}
