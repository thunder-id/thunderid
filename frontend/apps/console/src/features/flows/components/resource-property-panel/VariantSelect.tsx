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

import {FormControl, FormHelperText, FormLabel, MenuItem, Select} from '@wso2/oxygen-ui';
import {memo, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import useResourceFieldError from '../../hooks/useResourceFieldError';
import type {Element} from '../../models/elements';
import type {Resource} from '../../models/resources';

/**
 * Props interface of {@link VariantSelect}
 */
export interface VariantSelectProps {
  resource: Resource;
  selectedVariant: Element | undefined;
  onVariantChange?: (variant: string) => void;
}

/**
 * Reusable variant selector dropdown for resource property panels.
 * Renders a FormLabel + Select with the available variants for a resource.
 *
 * @param props - Props injected to the component.
 * @returns The VariantSelect component, or null if the resource has no variants.
 */
function VariantSelect({
  resource,
  selectedVariant,
  onVariantChange = undefined,
}: VariantSelectProps): ReactElement | null {
  const {t} = useTranslation();
  const errorMessage: string = useResourceFieldError(resource?.id, 'variant');

  if (!resource.variants || resource.variants.length === 0) {
    return null;
  }

  return (
    <div>
      <FormControl fullWidth error={!!errorMessage}>
        <FormLabel htmlFor="variant-select">{t('flows:core.elements.text.variant.label', 'Variant')}</FormLabel>
        <Select
          id="variant-select"
          value={selectedVariant?.variant ?? ''}
          error={!!errorMessage}
          onChange={(e) => {
            const newVariant = resource.variants?.find((variant: Element) => variant.variant === e.target.value);
            onVariantChange?.((newVariant?.variant as string) ?? '');
          }}
          fullWidth
        >
          {resource.variants.map((variant: Element) => (
            <MenuItem key={variant.variant as string} value={variant.variant as string}>
              {variant.variant as string}
            </MenuItem>
          ))}
        </Select>
        {errorMessage && <FormHelperText>{errorMessage}</FormHelperText>}
      </FormControl>
    </div>
  );
}

export default memo(VariantSelect);
