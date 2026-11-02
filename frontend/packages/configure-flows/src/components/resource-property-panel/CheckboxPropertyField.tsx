// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Checkbox, FormControlLabel, FormHelperText} from '@wso2/oxygen-ui';
import startCase from 'lodash-es/startCase';
import {type ReactElement, type SyntheticEvent} from 'react';
import useResourceFieldError from '../../hooks/useResourceFieldError';
import type {Resource} from '../../models/resources';

/**
 * Props interface of {@link CheckboxPropertyField}
 */
export interface CheckboxPropertyFieldPropsInterface {
  /**
   * The resource associated with the property.
   */
  resource: Resource;
  /**
   * The key of the property.
   */
  propertyKey: string;
  /**
   * The value of the property.
   */
  propertyValue: boolean;
  /**
   * The event handler for the property change.
   * @param propertyKey - The key of the property.
   * @param newValue - The new value of the property.
   * @param resource - The resource associated with the property.
   */
  onChange: (propertyKey: string, newValue: unknown, resource: Resource) => void;
}

/**
 * Checkbox property field component for rendering checkbox input fields.
 *
 * @param props - Props injected to the component.
 * @returns The CheckboxPropertyField component.
 */
function CheckboxPropertyField({
  resource,
  propertyKey,
  propertyValue,
  onChange,
  ...rest
}: CheckboxPropertyFieldPropsInterface): ReactElement {
  /**
   * Get the error message for the checkbox property field.
   */
  const errorMessage: string = useResourceFieldError(resource?.id, propertyKey);

  return (
    <Box>
      <FormControlLabel
        control={<Checkbox checked={propertyValue} color={errorMessage ? 'error' : 'primary'} />}
        label={startCase(propertyKey)}
        onChange={(_event: SyntheticEvent, checked: boolean) => onChange(propertyKey, checked, resource)}
        {...rest}
      />
      {errorMessage && <FormHelperText error>{errorMessage}</FormHelperText>}
    </Box>
  );
}

export default CheckboxPropertyField;
