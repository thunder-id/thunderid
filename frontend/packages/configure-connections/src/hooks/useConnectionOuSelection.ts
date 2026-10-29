// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useHasMultipleOUs} from '@thunderid/configure-organization-units';
import {useState} from 'react';

interface UseConnectionOuSelectionResult {
  /** The organization unit the new connection will belong to. Empty until one is chosen or implied. */
  ouId: string;
  setOuId: (ouId: string) => void;
  /** True when the user has to pick, because more than one organization unit exists. */
  needsPicker: boolean;
  isLoading: boolean;
}

/**
 * The organization unit a new connection is created in. It is picked by the user, and implied when the tenant has a
 * single organization unit whose children were looked up successfully.
 */
export default function useConnectionOuSelection(): UseConnectionOuSelectionResult {
  const {hasMultipleOUs, isLoading, isError, ouList} = useHasMultipleOUs();
  const [selectedOuId, setOuId] = useState<string>('');

  // The only organization unit is implied only when every lookup succeeded: a failed look-up of its children
  // would otherwise hide the picker and put the connection under a unit the user never chose.
  const impliedOuId: string = !hasMultipleOUs && !isError && ouList.length === 1 ? ouList[0].id : '';
  const ouId: string = selectedOuId !== '' ? selectedOuId : impliedOuId;

  return {ouId, setOuId, needsPicker: hasMultipleOUs, isLoading};
}
