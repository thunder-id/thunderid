// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {BuilderAppConfig} from '../models/builder';

/**
 * Warns that the Gate cannot be styled when the application the app signs in with is gone.
 */
export default function GateNote({config}: {config: BuilderAppConfig}): JSX.Element | null {
  const {t} = useTranslation('apps');

  return config.applicationId ? null : (
    <Alert severity="warning">
      {t(
        'builder.appearance.gate.noApplication',
        'The application this app signs in with was not found, so its sign-in screens cannot be styled here.',
      )}
    </Alert>
  );
}
