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

import {Alert, FormLabel, MenuItem, Select, Stack} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import useFlowConfig from '@/features/flows/hooks/useFlowConfig';
import type {StepData} from '@/features/flows/models/steps';
import {isSessionNode} from '@/features/login-flow/utils/ssoGraphTransforms';

/**
 * Properties panel for the SSO check executor: a picker for the session
 * checkpoint (`checkpointRef`) the check skips to when a live session exists.
 */
function SsoCheckProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const {flowNodes} = useFlowConfig();

  const checkpointRef = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.properties?.checkpointRef as string) ?? '';
  }, [resource]);

  const sessionNodes = useMemo(() => (flowNodes ?? []).filter(isSessionNode), [flowNodes]);

  const isDangling = checkpointRef !== '' && !sessionNodes.some((node) => node.id === checkpointRef);

  return (
    <Stack gap={2}>
      {isDangling && (
        <Alert severity="warning">
          {t(
            'flows:sso.properties.checkpointDangling',
            'The referenced session step no longer exists. Select a valid session step.',
          )}
        </Alert>
      )}
      <div>
        <FormLabel htmlFor="sso-checkpoint-select">
          {t('flows:sso.properties.checkpointLabel', 'Session checkpoint')}
        </FormLabel>
        <Select
          id="sso-checkpoint-select"
          value={sessionNodes.some((node) => node.id === checkpointRef) ? checkpointRef : ''}
          onChange={(e) => onChange('data.properties.checkpointRef', e.target.value, resource)}
          fullWidth
        >
          {sessionNodes.map((node) => {
            const label = ((node.data as StepData | undefined)?.properties?.displayName as string) || node.id;
            return (
              <MenuItem key={node.id} value={node.id}>
                {label}
              </MenuItem>
            );
          })}
        </Select>
      </div>
    </Stack>
  );
}

export default SsoCheckProperties;
