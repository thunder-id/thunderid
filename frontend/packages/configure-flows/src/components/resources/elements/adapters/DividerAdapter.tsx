// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {Divider, type DividerProps} from '@wso2/oxygen-ui';
import {useMemo, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import {DividerVariants, type Element as FlowElement} from '../../../../models/elements';

/**
 * Divider element type.
 */
export type DividerElement = FlowElement & {
  variant?: string;
  label?: string;
};

/**
 * Props interface of {@link DividerAdapter}
 */
export interface DividerAdapterPropsInterface {
  /**
   * The divider element properties.
   */
  resource: FlowElement;
}

/**
 * Adapter for the Divider component.
 *
 * @param props - Props injected to the component.
 * @returns The DividerAdapter component.
 */
function DividerAdapter({resource}: DividerAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();

  const dividerElement = resource as DividerElement;
  const variantStr = resource?.variant as string | undefined;

  const config: DividerProps = useMemo(() => {
    if (variantStr === DividerVariants.Horizontal || variantStr === DividerVariants.Vertical) {
      return {
        orientation: variantStr.toLowerCase() as 'horizontal' | 'vertical',
      };
    }
    if (variantStr) {
      return {
        variant: variantStr.toLowerCase() as DividerProps['variant'],
      };
    }
    return {};
  }, [variantStr]);

  return (
    <Divider {...config}>
      {dividerElement?.label && (resolve(dividerElement.label, {t}) ?? dividerElement.label)}
    </Divider>
  );
}

export default DividerAdapter;
