// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {Box, Typography} from '@wso2/oxygen-ui';
import {type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import type {Element as FlowElement} from '../../../../models/elements';

/**
 * Key-value list element type with properties at top level.
 */
export type KeyValueListElement = FlowElement & {
  label?: string;
  source?: string;
};

/**
 * Props interface of {@link KeyValueListAdapter}
 */
export interface KeyValueListAdapterPropsInterface {
  /**
   * The key-value list element properties.
   */
  resource?: FlowElement;
}

/**
 * A read-only canvas rendering of the KEY_VALUE_LIST element. Its rows are resolved at runtime
 * from the additional data key named in `source`, so how many there are and what they are called
 * is unknown while authoring, and the canvas shows the label and the bound key instead.
 *
 * @param props - Props injected to the component.
 * @returns The KeyValueListAdapter component.
 */
function KeyValueListAdapter({resource = undefined}: KeyValueListAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();

  const element = resource as KeyValueListElement | undefined;
  const source: string | undefined = element?.source;

  return (
    <Box sx={{display: 'flex', flexDirection: 'column', gap: 0.5, width: '100%'}}>
      <Typography variant="body2">{resolve(element?.label, {t}) ?? element?.label ?? ''}</Typography>
      <Typography variant="caption" color="textSecondary" sx={{fontFamily: 'monospace'}}>
        {source ? `{{${source}}}` : 'No source bound'}
      </Typography>
    </Box>
  );
}

export default KeyValueListAdapter;
