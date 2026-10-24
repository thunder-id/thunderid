// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationAccessOffers} from '../models/organization-access';
import type {SharingPolicy} from '../models/sharing-policy';

/**
 * Reports which deployment-wide choices to offer for a resource. The console can keep them but not
 * create them, so each is offered only while the saved policy already holds it.
 *
 * @param policy - The owner's policy as last read
 * @returns The deployment-wide choices the screen may show
 */
export default function getOrganizationAccessOffers(policy: SharingPolicy | undefined): OrganizationAccessOffers {
  return {everyUnit: Boolean(policy?.allUnits), everyTree: policy?.otherTreeRoots?.mode === 'all'};
}
