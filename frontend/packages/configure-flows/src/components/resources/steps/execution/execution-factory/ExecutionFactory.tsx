// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import ResourceDisplayImage from '../../../../ResourceDisplayImage';
import type {ExecutionMinimalPropsInterface} from '../ExecutionMinimal';

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
    <Box>
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
