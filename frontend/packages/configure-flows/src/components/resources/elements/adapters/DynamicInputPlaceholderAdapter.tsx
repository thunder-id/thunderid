// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {Layers} from '@wso2/oxygen-ui-icons-react';
import {type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import type {Element as FlowElement} from '../../../../models/elements';

export interface DynamicInputPlaceholderAdapterPropsInterface {
  resource: FlowElement;
}

function DynamicInputPlaceholderAdapter({resource}: DynamicInputPlaceholderAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();

  const placeholder =
    (resource as FlowElement & {placeholder?: string}).placeholder ??
    t('flows:core.placeholders.dynamicInputPlaceholder.title');
  const hint =
    (resource as FlowElement & {hint?: string}).hint ?? t('flows:core.placeholders.dynamicInputPlaceholder.hint');

  return (
    <Box
      className="adapter dynamic-input-placeholder-adapter"
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 0.75,
        minHeight: 72,
        px: 2,
        py: 1.5,
        borderRadius: 1,
        border: '1px dashed',
        borderColor: 'primary.light',
        backgroundColor: 'action.hover',
        textAlign: 'center',
      }}
    >
      <Layers size={20} color="primary" />
      <Typography variant="h5" color="primary">
        {placeholder}
      </Typography>
      <Typography variant="subtitle2" color="text.secondary" sx={{fontSize: '0.7rem'}}>
        {hint}
      </Typography>
    </Box>
  );
}

export default DynamicInputPlaceholderAdapter;
