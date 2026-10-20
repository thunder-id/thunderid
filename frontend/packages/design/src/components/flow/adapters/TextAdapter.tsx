// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {cn} from '@thunderid/utils';
import {Typography} from '@wso2/oxygen-ui';
import {useContext, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import DesignContext from '../../../contexts/Design/DesignContext';
import type {FlowComponent} from '../../../models/flow';
import mapEmbeddedFlowTextColor from '../../../utils/mapEmbeddedFlowTextColor';
import {mapEmbeddedFlowTextVariant} from '../../../utils/mapEmbeddedFlowTextVariant';

interface TextAdapterProps {
  component: FlowComponent;
  resolve: (template: string | undefined) => string | undefined;
}

export default function TextAdapter({component, resolve}: TextAdapterProps): JSX.Element {
  const {t} = useTranslation();
  // Read the context directly so the adapter also works on pages that mount no DesignProvider.
  const isDesignEnabled = useContext(DesignContext)?.isDesignEnabled ?? false;
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
