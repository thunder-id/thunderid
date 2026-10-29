// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getConsentDetailLayout, type ConsentDetailRow} from '@thunderid/react';
import type {SxProps, Theme} from '@wso2/oxygen-ui';

// A single style object, which can be merged with others in an sx array.
type SxObject = Exclude<SxProps<Theme>, readonly unknown[] | ((theme: Theme) => unknown)>;

export interface AuthorizationDetailRowLayout {
  // Whether the value is stacked below its label rather than beside it.
  stacked: boolean;
  row: SxObject;
  label: SxObject;
  value: SxObject;
}

// How one field of an authorization detail is laid out. A short field sits beside its label and drops
// below it when the two do not fit; a long one is stacked, the label above the value, so neither squeezes
// the other. Exported so an editor of a type lays a field out exactly as the consent screen does.
export default function getAuthorizationDetailRowLayout(row: ConsentDetailRow): AuthorizationDetailRowLayout {
  const stacked = getConsentDetailLayout(row) === 'stacked';
  return {
    stacked,
    row: {
      display: 'flex',
      flexDirection: stacked ? 'column' : 'row',
      flexWrap: stacked ? 'nowrap' : 'wrap',
      justifyContent: 'space-between',
      alignItems: stacked ? 'stretch' : 'center',
      columnGap: 2,
      rowGap: 0.5,
      py: 0.75,
      px: 1,
    },
    label: {flex: stacked ? 'none' : '1 1 auto', minWidth: 0, lineHeight: 1.45, overflowWrap: 'anywhere'},
    value: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: stacked ? 'flex-start' : 'flex-end',
      gap: 1,
      minWidth: 0,
      maxWidth: '100%',
      ml: stacked ? 0 : 'auto',
    },
  };
}
