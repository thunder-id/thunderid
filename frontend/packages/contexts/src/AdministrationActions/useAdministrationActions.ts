// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import AdministrationActionsContext, {type AdministrationActions} from './AdministrationActionsContext';

/**
 * Reads the administrative actions this console performs.
 *
 * An absent entry is not an error: it means the operation is a plain management call here.
 */
export default function useAdministrationActions(): AdministrationActions {
  return useContext(AdministrationActionsContext);
}
