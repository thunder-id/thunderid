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
import {Divider} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {FlowComponent} from '../../../models/flow';

interface DividerAdapterProps {
  component: FlowComponent;
  resolve: (template: string | undefined) => string | undefined;
}

export default function DividerAdapter({component, resolve}: DividerAdapterProps): JSX.Element {
  const {t} = useTranslation();
  const label = resolve(component.label);

  return (
    <Divider
      id={component.id}
      className={[cn('Flow--divider', 'Divider--root'), component.classes].filter(Boolean).join(' ')}
      orientation={component.variant === 'VERTICAL' ? 'vertical' : 'horizontal'}
      sx={{my: 2}}
    >
      {label ? t(label) : undefined}
    </Divider>
  );
}
