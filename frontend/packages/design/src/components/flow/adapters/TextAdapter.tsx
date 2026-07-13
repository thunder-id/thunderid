/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {cn} from '@thunderid/utils';
import {Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useDesign from '../../../contexts/Design/useDesign';
import type {FlowComponent} from '../../../models/flow';
import {mapEmbeddedFlowTextVariant} from '../../../utils/mapEmbeddedFlowTextVariant';

interface TextAdapterProps {
  component: FlowComponent;
  resolve: (template: string | undefined) => string | undefined;
}

export default function TextAdapter({component, resolve}: TextAdapterProps): JSX.Element {
  const {t} = useTranslation();
  const {isDesignEnabled} = useDesign();
  const typographyVariant = mapEmbeddedFlowTextVariant(component.variant);

  const textAlign = component.align ?? (isDesignEnabled ? 'center' : 'left');

  return (
    <Typography
      id={component.id}
      className={[cn('Flow--text', `Text--${typographyVariant}`), component.classes].filter(Boolean).join(' ')}
      variant={typographyVariant}
      sx={{mb: 1, textAlign}}
    >
      {t(resolve(component.label)!)}
    </Typography>
  );
}
