// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Tooltip, Typography, type TypographyProps} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {shortHash} from '../utils/format';

export interface VersionNameProps {
  /** The version's number, "v14"; without one the version is named by its short hash. */
  number?: number;
  /** The hash of what the version holds, shown on hover. */
  hash?: string;
  variant?: TypographyProps['variant'];
}

/** A version as people name it, "v14", with the hash of what it holds a hover away. */
export default function VersionName({
  number = undefined,
  hash = undefined,
  variant = 'body2',
}: VersionNameProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const name = number !== undefined ? `v${number}` : shortHash(hash);
  const label = (
    <Typography component="span" variant={variant} fontWeight={600} sx={{whiteSpace: 'nowrap'}}>
      {name}
    </Typography>
  );
  if (!hash) return label;
  return (
    <Tooltip
      title={
        <Typography variant="caption" component="span" sx={{fontFamily: 'monospace', wordBreak: 'break-all'}}>
          {t('version.hash', 'Hash {{hash}}', {hash})}
        </Typography>
      }
    >
      {label}
    </Tooltip>
  );
}
