// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormControl, FormHelperText, FormLabel, MenuItem, Select} from '@wso2/oxygen-ui';
import {memo, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import useResourceFieldError from '../../hooks/useResourceFieldError';
import {TypographyColors} from '../../models/elements';
import type {Resource} from '../../models/resources';

/**
 * Available color options for text components
 * Dynamically generated from TypographyColors enum
 */
const COLOR_OPTIONS: {value: string; label: string}[] = Object.values(TypographyColors).map((value) => ({
  value,
  label: value,
}));

/**
 * Props interface of {@link ColorSelect}
 */
export interface ColorSelectProps {
  resource: Resource;
  selectedColor: string | undefined;
  onColorChange?: (color: string) => void;
}

/**
 * Color selector dropdown for TEXT component color property.
 * Renders a FormLabel + Select with available text colors from oxygen-ui.
 *
 * @param props - Props injected to the component.
 * @returns The ColorSelect component.
 */
function ColorSelect({resource, selectedColor, onColorChange = undefined}: ColorSelectProps): ReactElement {
  const {t} = useTranslation();
  const errorMessage: string = useResourceFieldError(resource?.id, 'color');

  return (
    <div>
      <FormControl fullWidth error={!!errorMessage}>
        <FormLabel htmlFor="color-select">{t('flows:core.elements.text.color.label', 'Color')}</FormLabel>
        <Select
          id="color-select"
          value={selectedColor ?? ''}
          error={!!errorMessage}
          onChange={(e) => {
            onColorChange?.(e.target.value);
          }}
          fullWidth
          displayEmpty
        >
          <MenuItem value="">{t('flows:core.elements.text.color.default', 'DEFAULT')}</MenuItem>
          {COLOR_OPTIONS.map((color) => (
            <MenuItem key={color.value} value={color.value}>
              {color.label}
            </MenuItem>
          ))}
        </Select>
        {errorMessage && <FormHelperText>{errorMessage}</FormHelperText>}
      </FormControl>
    </div>
  );
}

export default memo(ColorSelect);
