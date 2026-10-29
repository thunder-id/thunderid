// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AdministrationFlowConfigKeyValue, BasicFlowSummary} from '@thunderid/utils';

/**
 * The administration flows behind the account lifecycle operations.
 *
 * @public
 */
export interface AccountLifecycleFlows {
  /** Each flow a lifecycle purpose resolves to, once, however many purposes name it. */
  flows: BasicFlowSummary[];
  /** The purposes with no usable flow: unset, or naming a flow that does not exist. */
  unconfigured: AdministrationFlowConfigKeyValue[];
}
