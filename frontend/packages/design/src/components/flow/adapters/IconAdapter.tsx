// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {cn} from '@thunderid/utils';
import {Box} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import useOxygenIcons from '../../../hooks/useOxygenIcons';
import type {FlowComponent} from '../../../models/flow';
import OxygenIcon from '../../OxygenIcon';

interface IconAdapterProps {
  component: FlowComponent;
}

export default function IconAdapter({component}: IconAdapterProps): JSX.Element | null {
  const icon = useOxygenIcons()?.get(component.name ?? 'ArrowLeftRight');
  if (!icon) return null;

  return (
    <Box
      id={component.id}
      className={[cn('Flow--icon'), component.classes].filter(Boolean).join(' ')}
      sx={{display: 'flex', alignItems: 'center'}}
    >
      <OxygenIcon icon={icon} size={component.size ?? 24} color={component.color ?? 'currentColor'} />
    </Box>
  );
}
