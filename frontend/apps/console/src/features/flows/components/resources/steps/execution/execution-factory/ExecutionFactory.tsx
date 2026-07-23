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

import {Box, Typography} from '@wso2/oxygen-ui';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import type {ExecutionMinimalPropsInterface} from '../ExecutionMinimal';
import ResourceDisplayImage from '@/features/flows/components/ResourceDisplayImage';

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

  // Display metadata is resolved from the same executor definitions that back the
  // resource panel listing, so the node always mirrors the panel entry.
  const displayImage = resource.display?.image;
  // display.label contains the action/mode (e.g., "Passkey Challenge", "Send SMS OTP")
  const displayLabel = resource.display?.label;

  return (
    <Box className="flow-builder-execution">
      <Box display="flex" gap={1} alignItems="center">
        <ResourceDisplayImage
          image={displayImage}
          label={displayLabel}
          preserveColor={resource.display?.preserveImageColor}
        />
        <Typography variant="body1">{displayLabel ?? t('flows:core.executions.names.default', 'Executor')}</Typography>
      </Box>
    </Box>
  );
}

export default ExecutionFactory;
