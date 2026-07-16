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

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {Typography, type TypographyProps} from '@wso2/oxygen-ui';
import {useMemo, type CSSProperties, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import TemplatePlaceholder, {containsTemplateLiteral} from './TemplatePlaceholder';
import {VARIANT_TO_MUI_MAP} from '@/features/flows/constants/typographyVariantMaps';
import {TypographyVariants, type Element} from '@/features/flows/models/elements';

/**
 * Configuration interface for Typography element.
 */
interface TypographyConfig {
  styles?: CSSProperties;
}

/**
 * Typography element with specific variant type.
 */
export interface TypographyElement extends Element<TypographyConfig> {
  variant: (typeof TypographyVariants)[keyof typeof TypographyVariants];
  label?: string;
  align?: 'inherit' | 'left' | 'center' | 'right' | 'justify';
}

/**
 * Props interface of {@link TypographyAdapter}
 */
export interface TypographyAdapterPropsInterface {
  /**
   * The step id the resource resides on.
   */
  stepId: string;
  /**
   * The typography element properties.
   */
  resource: Element;
}

/**
 * Adapter for the Typography component.
 *
 * @param props - Props injected to the component.
 * @returns The TypographyAdapter component.
 */
function TypographyAdapter({resource}: TypographyAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();

  const typographyConfig = resource.config as TypographyConfig | undefined;
  const typographyElement = resource as TypographyElement;
  const variantStr = resource?.variant as string | undefined;

  const config: TypographyProps = useMemo(() => ({}), []);

  const muiVariant = variantStr ? VARIANT_TO_MUI_MAP[variantStr] : undefined;
  const align = typographyElement?.align;

  const rawLabel = typographyElement?.label ?? '';
  const labelNode: ReactNode = containsTemplateLiteral(rawLabel) ? (
    <TemplatePlaceholder value={rawLabel} t={t} />
  ) : (
    (resolve(rawLabel, {t}) ?? rawLabel)
  );

  return (
    <Typography variant={muiVariant} align={align} style={typographyConfig?.styles} {...config}>
      {labelNode}
    </Typography>
  );
}

export default TypographyAdapter;
