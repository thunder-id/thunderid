// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';
import IntegrationGuides, {
  type IntegrationGuidesProps,
} from '@/features/applications/components/edit-application/integration-guides/IntegrationGuides';

/**
 * Renders the application Overview tab, which tells a developer how to integrate against this
 * application.
 *
 * The tab prints the endpoints a client calls at runtime, all of them built from the server this
 * console talks to. That is correct on a Data Plane, which serves them. It is not correct on a
 * Control Plane, which serves none of them, so this module is passed to the page by the Data Plane
 * console's route tree and by nothing else.
 */
export default function renderIntegrationGuides(props: IntegrationGuidesProps): JSX.Element {
  return <IntegrationGuides {...props} />;
}
