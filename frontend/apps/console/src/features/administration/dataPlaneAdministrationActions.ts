// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {deleteUserViaFlow} from '@thunderid/configure-users';
import type {AdministrationActions} from '@thunderid/contexts';
import {
  deleteApplicationViaFlow,
  regenerateClientSecretViaFlow,
} from '../applications/utils/applicationAdministrationFlow';

/**
 * The administrative operations as a Data Plane carries them out.
 *
 * Each runs the administration flow configured for it, which is what revokes tokens, ends sessions
 * and detaches grants before a record is removed; every one of them reaches `/flow/execute`.
 *
 * This module is imported by the Data Plane console's route tree and by nothing else. That is the
 * whole mechanism: the Control Plane console has its own route tree, never imports this, and so
 * links none of the flow machinery reached from here. There is no setting that decides it.
 */
const dataPlaneAdministrationActions: AdministrationActions = {
  deleteApplication: deleteApplicationViaFlow,
  deleteUser: deleteUserViaFlow,
  regenerateClientSecret: regenerateClientSecretViaFlow,
};

export default dataPlaneAdministrationActions;
