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
