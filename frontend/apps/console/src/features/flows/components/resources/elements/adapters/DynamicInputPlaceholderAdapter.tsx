/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {Box, Typography} from '@wso2/oxygen-ui';
import {Layers} from '@wso2/oxygen-ui-icons-react';
import {type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import type {Element as FlowElement} from '@/features/flows/models/elements';

export interface DynamicInputPlaceholderAdapterPropsInterface {
  resource: FlowElement;
}

function DynamicInputPlaceholderAdapter({resource}: DynamicInputPlaceholderAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();

  const placeholder =
    (resource as FlowElement & {placeholder?: string}).placeholder ??
    t('flows:core.placeholders.dynamicInputPlaceholder.title', 'Dynamic Input');

  const hint =
    (resource as FlowElement & {hint?: string}).hint ??
    t(
      'flows:core.placeholders.dynamicInputPlaceholder.hint',
      'Resolves input fields passed from runtime when the flow executes',
    );

  return (
    <Box
      display="flex"
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      sx={{
        width: '100%',
        minHeight: 72,
        borderRadius: 1,
        border: '1px dashed',
        borderColor: 'primary.light',
        backgroundColor: (theme) =>
          theme.palette.mode === 'dark' ? 'rgba(99, 102, 241, 0.08)' : 'rgba(99, 102, 241, 0.05)',
        px: 1.5,
        py: 1.5,
        gap: 0.5,
      }}
    >
      <Layers size={20} color="primary" />
      <Typography variant="h5" color="primary" align="center">
        {placeholder}
      </Typography>
      <Typography variant="subtitle2" color="textSecondary" align="center" sx={{fontSize: '0.7rem'}}>
        {hint}
      </Typography>
    </Box>
  );
}

export default DynamicInputPlaceholderAdapter;
