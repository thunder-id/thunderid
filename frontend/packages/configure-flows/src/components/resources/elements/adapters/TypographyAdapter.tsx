// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {Typography, type TypographyProps} from '@wso2/oxygen-ui';
import {useMemo, type CSSProperties, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import TemplatePlaceholder, {containsTemplateLiteral} from './TemplatePlaceholder';
import {VARIANT_TO_MUI_MAP, COLOR_ENUM_TO_MUI_MAP} from '../../../../constants/typographyVariantMaps';
import {TypographyVariants, type Element} from '../../../../models/elements';

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
  color?: string;
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
  const colorEnum = typographyElement?.color;
  const color = colorEnum ? COLOR_ENUM_TO_MUI_MAP[colorEnum] : undefined;

  const rawLabel = typographyElement?.label ?? '';
  const labelNode: ReactNode = containsTemplateLiteral(rawLabel) ? (
    <TemplatePlaceholder value={rawLabel} t={t} />
  ) : (
    (resolve(rawLabel, {t}) ?? rawLabel)
  );

  return (
    <Typography variant={muiVariant} align={align} color={color} style={typographyConfig?.styles} {...config}>
      {labelNode}
    </Typography>
  );
}

export default TypographyAdapter;
