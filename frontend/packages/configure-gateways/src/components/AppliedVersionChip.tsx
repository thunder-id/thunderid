// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Chip, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetAppliedVersion from '../api/useGetAppliedVersion';

export interface AppliedVersionChipProps {
  gatewayId: string;
}

/**
 * The version a gateway holds, read for that gateway alone since the listing does not carry it.
 */
export default function AppliedVersionChip({gatewayId}: AppliedVersionChipProps): JSX.Element {
  const {t} = useTranslation();
  const {data, isLoading, isError} = useGetAppliedVersion(gatewayId);

  if (isLoading) {
    return (
      <Typography variant="body2" color="text.disabled">
        ...
      </Typography>
    );
  }

  if (isError) {
    return (
      <Typography variant="body2" color="text.disabled">
        {t('gateways:applied.unknown', 'Unknown')}
      </Typography>
    );
  }

  if (data?.appliedVersion === undefined) {
    return <Chip size="small" variant="outlined" label={t('gateways:applied.notApplied', 'Not applied')} />;
  }

  return (
    <Chip
      size="small"
      color="success"
      variant="outlined"
      label={t('gateways:versions.versionNumber', 'Version {{version}}', {version: data.appliedVersion})}
    />
  );
}
