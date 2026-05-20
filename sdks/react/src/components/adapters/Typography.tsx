/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {FC} from 'react';
import useTheme from '../../contexts/Theme/useTheme';
import {AdapterProps} from '../../models/adapters';
import Typography from '../primitives/Typography/Typography';

/**
 * Typography component for sign-up forms (titles, descriptions, etc.).
 */
const TypographyComponent: FC<AdapterProps> = ({component}: AdapterProps) => {
  const {theme} = useTheme();
  const config: Record<string, unknown> = component.config || {};
  const text: string = (config['text'] as string) || (config['content'] as string) || '';
  const variant: string = component.variant?.toLowerCase() || 'body1';

  // Map component variants to Typography variants
  let typographyVariant: any = 'body1';

  switch (variant) {
    case 'h1':
      typographyVariant = 'h1';
      break;
    case 'h2':
      typographyVariant = 'h2';
      break;
    case 'h3':
      typographyVariant = 'h3';
      break;
    case 'h4':
      typographyVariant = 'h4';
      break;
    case 'h5':
      typographyVariant = 'h5';
      break;
    case 'h6':
      typographyVariant = 'h6';
      break;
    case 'subtitle1':
      typographyVariant = 'subtitle1';
      break;
    case 'subtitle2':
      typographyVariant = 'subtitle2';
      break;
    case 'body2':
      typographyVariant = 'body2';
      break;
    case 'caption':
      typographyVariant = 'caption';
      break;
    default:
      typographyVariant = 'body1';
  }

  return (
    <Typography
      key={component.id}
      variant={typographyVariant}
      style={{marginBottom: `calc(${theme.vars.spacing.unit} * 2)`}}
    >
      {text}
    </Typography>
  );
};

export default TypographyComponent;
