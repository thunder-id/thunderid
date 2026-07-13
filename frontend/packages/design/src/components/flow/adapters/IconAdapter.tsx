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
import {Box} from '@wso2/oxygen-ui';
import * as OxygenIcons from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import type React from 'react';
import type {FlowComponent} from '../../../models/flow';

interface IconAdapterProps {
  component: FlowComponent;
}

export default function IconAdapter({component}: IconAdapterProps): JSX.Element | null {
  const iconName = component.name ?? 'ArrowLeftRight';
  const icons = OxygenIcons as unknown as Record<
    string,
    React.ComponentType<{fontSize?: number | string; sx?: Record<string, unknown>}>
  >;
  if (!Object.keys(icons).includes(iconName)) return null;
  const IconComponent = icons[iconName];
  if (!IconComponent) return null;

  return (
    <Box
      id={component.id}
      className={[cn('Flow--icon'), component.classes].filter(Boolean).join(' ')}
      sx={{display: 'flex', alignItems: 'center'}}
    >
      <IconComponent fontSize={component.size ?? 24} sx={{color: component.color ?? 'currentColor'}} />
    </Box>
  );
}
