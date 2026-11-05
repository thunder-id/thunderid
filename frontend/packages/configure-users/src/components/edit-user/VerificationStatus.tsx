// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Chip, Tooltip} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface VerificationStatusProps {
  /**
   * When the stored value was verified; absent while it is unverified.
   */
  verifiedAt?: string;
}

/**
 * Compact verification status beside a stored contact attribute's label.
 */
export default function VerificationStatus({verifiedAt = undefined}: VerificationStatusProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Tooltip
      describeChild
      title={
        verifiedAt
          ? t('users:verification.verifiedAt', 'Verified on {{date}}', {date: new Date(verifiedAt).toLocaleString()})
          : ''
      }
    >
      <Chip
        label={
          verifiedAt ? t('users:verification.verified', 'Verified') : t('users:verification.unverified', 'Unverified')
        }
        size="small"
        variant="outlined"
        color={verifiedAt ? 'success' : 'default'}
        tabIndex={verifiedAt ? 0 : undefined}
        data-testid="verification-status"
        sx={{ml: 1}}
      />
    </Tooltip>
  );
}
