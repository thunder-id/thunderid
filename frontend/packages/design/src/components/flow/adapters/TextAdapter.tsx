// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {cn} from '@thunderid/utils';
import {Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useOptionalDesign from '../../../contexts/Design/useOptionalDesign';
import type {FlowComponent} from '../../../models/flow';
import mapEmbeddedFlowTextColor from '../../../utils/mapEmbeddedFlowTextColor';
import {mapEmbeddedFlowTextVariant} from '../../../utils/mapEmbeddedFlowTextVariant';

interface TextAdapterProps {
  component: FlowComponent;
  resolve: (template: string | undefined) => string | undefined;
}

export default function TextAdapter({component, resolve}: TextAdapterProps): JSX.Element {
  const {t} = useTranslation();
  // The console mounts no DesignProvider, so the adapter reads the design without requiring one.
  const isDesignEnabled = useOptionalDesign()?.isDesignEnabled ?? false;
  const typographyVariant = mapEmbeddedFlowTextVariant(component.variant);

  const textAlign = component.align ?? (isDesignEnabled ? 'center' : 'left');
  const color = mapEmbeddedFlowTextColor(component.color);

  return (
    <Typography
      id={component.id}
      className={[cn('Flow--text', `Text--${typographyVariant}`), component.classes].filter(Boolean).join(' ')}
      variant={typographyVariant}
      color={color}
      sx={{mb: 1, textAlign}}
    >
      {t(resolve(component.label)!)}
    </Typography>
  );
}
