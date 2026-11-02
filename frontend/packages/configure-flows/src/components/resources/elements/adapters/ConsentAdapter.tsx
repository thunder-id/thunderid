// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Typography, Box, Divider, Switch} from '@wso2/oxygen-ui';
import {type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import type {Resource} from '../../../../models/resources';

/**
 * Props interface for ConsentAdapter
 */
export interface ConsentAdapterPropsInterface {
  resource?: Resource;
}

/** Widths of the mock attribute label bars, sketching rows of varying length. */
const PLACEHOLDER_ROW_WIDTHS = ['55%', '40%', '65%'];

/**
 * A placeholder for the Consent element. The consented attributes are resolved
 * per application at runtime, so this sketches the attribute list the end user
 * will see: label bars with toggles, in the same row layout as the gate.
 *
 * @returns The ConsentAdapter placeholder component.
 */
function ConsentAdapter(): ReactElement {
  const {t} = useTranslation();

  return (
    <Box data-testid="consent-placeholder" sx={{width: '100%', py: 1}}>
      <Box sx={{border: '1px dashed', borderColor: 'divider', borderRadius: 1.5, px: 1.5, py: 0.5}}>
        {PLACEHOLDER_ROW_WIDTHS.map((width, index) => (
          <Box key={width}>
            <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'space-between', py: 0.75}}>
              <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5, flex: 1}}>
                <Box sx={{width: 6, height: 6, borderRadius: '50%', bgcolor: 'text.disabled', flexShrink: 0}} />
                <Box sx={{width, height: 8, borderRadius: 1, bgcolor: 'action.disabledBackground'}} />
              </Box>
              <Switch size="small" checked disabled />
            </Box>
            {index < PLACEHOLDER_ROW_WIDTHS.length - 1 && <Divider sx={{opacity: 0.5}} />}
          </Box>
        ))}
      </Box>
      <Typography
        variant="caption"
        color="textSecondary"
        sx={{fontStyle: 'italic', display: 'block', mt: 0.75, textAlign: 'center'}}
      >
        {t('flows:core.elements.consent.placeholder', 'Consent attributes will appear here at runtime')}
      </Typography>
    </Box>
  );
}

export default ConsentAdapter;
