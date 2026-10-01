// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {PuzzleIcon} from '@wso2/oxygen-ui-icons-react';
import {type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import type {Element as FlowElement} from '../../../../models/elements';

export interface CustomAdapterPropsInterface {
  resource: FlowElement;
}

function CustomAdapter({resource}: CustomAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();

  return (
    <Box
      display="flex"
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      sx={{
        width: '100%',
        minHeight: 64,
        backgroundColor: 'rgba(0, 0, 0, 0.04)',
        borderRadius: 1,
        border: '1px dashed rgba(0, 0, 0, 0.2)',
        px: 1,
        py: 1.5,
        gap: 0.5,
      }}
    >
      <PuzzleIcon size={20} />
      <Typography variant="h5">{t('flows:core.placeholders.customComponent', 'Custom')}</Typography>
      <Typography variant="subtitle2" color="textSecondary" sx={{fontFamily: 'monospace', fontSize: '0.7rem'}}>
        {t('flows:core.placeholders.customComponent.identifier', 'Identifier: {{id}}', {id: resource.id})}
      </Typography>
    </Box>
  );
}

export default CustomAdapter;
