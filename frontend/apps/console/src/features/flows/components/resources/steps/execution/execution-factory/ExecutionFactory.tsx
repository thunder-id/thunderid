/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {Box, Typography, useColorScheme} from '@wso2/oxygen-ui';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import GithubExecution from './GithubExecution';
import GoogleExecution from './GoogleExecution';
import type {ExecutionMinimalPropsInterface} from '../ExecutionMinimal';
import {ExecutionTypes} from '@/features/flows/models/steps';
import resolveStaticResourcePath from '@/features/flows/utils/resolveStaticResourcePath';

/**
 * Props interface of {@link CommonStepFactory}
 */
export type ExecutionFactoryPropsInterface = ExecutionMinimalPropsInterface;

/**
 * Factory for creating execution types.
 *
 * @param props - Props injected to the component.
 * @returns The ExecutionFactory component.
 */
function ExecutionFactory({resource}: ExecutionFactoryPropsInterface): ReactElement | null {
  const {t} = useTranslation();
  const {mode, systemMode} = useColorScheme();

  // Determine the effective mode - if mode is 'system', use systemMode
  const effectiveMode = mode === 'system' ? systemMode : mode;

  const action = resource.data?.action;
  const executorName = action?.executor?.name;
  const displayImage = resource.display?.image;
  // display.label contains the action/mode (e.g., "Passkey Challenge", "Send SMS OTP")
  const displayLabel = resource.display?.label;
  // Optional descriptive text shown inside the node body (e.g. what the executor checks).
  const displayDescription = resource.display?.description;

  // Google, GitHub, and SMS OTP executors have special validation logic
  if (executorName === ExecutionTypes.GoogleFederation) {
    return <GoogleExecution resource={resource} />;
  }

  if (executorName === ExecutionTypes.GithubFederation) {
    return <GithubExecution resource={resource} />;
  }

  // For all other executors, render icon and action/mode label
  // The header shows the executor name, the content shows the action/mode
  if (displayImage) {
    return (
      <Box className="flow-builder-execution">
        <Box display="flex" gap={1} alignItems="center">
          <img
            src={resolveStaticResourcePath(displayImage)}
            alt={`${displayLabel ?? 'executor'}-icon`}
            height="20"
            style={{filter: effectiveMode === 'dark' ? 'brightness(0.9) invert(1)' : 'none'}}
          />
          <Typography variant="body1">{displayLabel ?? t('flows:core.executions.names.default')}</Typography>
        </Box>
        {displayDescription && (
          <Typography variant="body2" color="text.secondary">
            {displayDescription}
          </Typography>
        )}
      </Box>
    );
  }

  // Fallback for executors without display image
  return (
    <Box display="flex" flexDirection="column" gap={1}>
      <Typography variant="body1">{displayLabel ?? t('flows:core.executions.names.default')}</Typography>
      {displayDescription && (
        <Typography variant="body2" color="text.secondary">
          {displayDescription}
        </Typography>
      )}
    </Box>
  );
}

export default ExecutionFactory;
