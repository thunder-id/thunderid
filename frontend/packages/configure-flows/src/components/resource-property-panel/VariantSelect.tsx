// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
