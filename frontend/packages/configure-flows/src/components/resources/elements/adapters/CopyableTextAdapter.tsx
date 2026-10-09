// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {Box, Typography} from '@wso2/oxygen-ui';
import {type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import type {Element as FlowElement} from '../../../../models/elements';

/**
 * Copyable text element type with properties at top level.
 */
export type CopyableTextElement = FlowElement & {
  label?: string;
  source?: string;
};

/**
 * Props interface of {@link CopyableTextAdapter}
 */
export interface CopyableTextAdapterPropsInterface {
  /**
   * The copyable text element properties.
   */
  resource?: FlowElement;
}

/**
 * A read-only canvas rendering of the COPYABLE_TEXT element. The value is resolved at
 * runtime from the additional data key named in `source`, which has no value while
 * authoring, so the canvas shows the label and the bound key instead.
 *
 * @param props - Props injected to the component.
 * @returns The CopyableTextAdapter component.
 */
function CopyableTextAdapter({resource = undefined}: CopyableTextAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();

  const element = resource as CopyableTextElement | undefined;
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

export default CopyableTextAdapter;
