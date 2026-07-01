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

import {Checkbox, FormControl, FormControlLabel, FormLabel, TextField} from '@wso2/oxygen-ui';
import startCase from 'lodash-es/startCase';
import type {ChangeEvent, ReactElement, SyntheticEvent} from 'react';
import PresentationDefinitionSelect from './PresentationDefinitionSelect';
import RichTextWithTranslation from './rich-text/RichTextWithTranslation';
import {ElementTypes} from '../../models/elements';
import type {Resource} from '../../models/resources';

/**
 * Props interface of {@link CommonStepPropertyFactory}
 */
export interface CommonStepPropertyFactoryPropsInterface {
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
  propertyValue: unknown;
  /**
   * The event handler for the property change.
   * @param propertyKey - The key of the property.=
   * @param newValue - The new value of the property.
   * @param resource - The resource associated with the property.
   */
  onChange: (propertyKey: string, newValue: string | boolean | number, resource: Resource, debounce?: boolean) => void;
  /**
   * Additional props.
   */
  [key: string]: unknown;
}

/**
 * Factory to generate the common property configurator for the given step.
 *
 * @param props - Props injected to the component.
 * @returns The CommonStepPropertyFactory component.
 */
function CommonStepPropertyFactory({
  resource,
  propertyKey,
  propertyValue,
  onChange,
  ...rest
}: CommonStepPropertyFactoryPropsInterface): ReactElement | null {
  const displayKey = propertyKey.replace(/^data\.properties\./, '');
  const displayLabel = startCase(displayKey);

  if (propertyKey === 'text') {
    if (resource.type === ElementTypes.RichText) {
      return (
        <RichTextWithTranslation
          onChange={(html: string) => onChange(propertyKey, html, resource, true)}
          resource={resource}
          {...rest}
        />
      );
    }
  }

  if (typeof propertyValue === 'boolean') {
    return (
      <FormControlLabel
        control={<Checkbox checked={propertyValue} />}
        label={displayLabel}
        onChange={(_event: SyntheticEvent, checked: boolean) => onChange(propertyKey, checked, resource)}
        {...rest}
      />
    );
  }

  if (typeof propertyValue === 'number') {
    return (
      <FormControl fullWidth sx={{mb: 3}}>
        <FormLabel htmlFor={propertyKey}>{displayLabel}</FormLabel>
        <TextField
          fullWidth
          id={propertyKey}
          defaultValue={propertyValue}
          onChange={(e: ChangeEvent<HTMLInputElement>) => {
            const val = e.target.value;

            if (val === '') {
              return;
            }

            const num = Number(val);

            if (Number.isNaN(num)) {
              return;
            }

            onChange(propertyKey, num, resource, true);
          }}
          placeholder={`Enter ${displayLabel}`}
          type="number"
          {...rest}
        />
      </FormControl>
    );
  }

  if (displayKey === 'presentation_definition_id') {
    return (
      <PresentationDefinitionSelect
        propertyKey={propertyKey}
        value={typeof propertyValue === 'string' ? propertyValue : ''}
        onChange={(newValue: string) => onChange(propertyKey, newValue, resource, true)}
      />
    );
  }

  if (typeof propertyValue === 'string') {
    return (
      <FormControl fullWidth sx={{mb: 3}}>
        <FormLabel htmlFor={propertyKey}>{displayLabel} </FormLabel>
        <TextField
          fullWidth
          id={propertyKey}
          defaultValue={propertyValue}
          onChange={(e: ChangeEvent<HTMLInputElement>) => onChange(propertyKey, e.target.value, resource, true)}
          placeholder={`Enter ${displayLabel}`}
          {...rest}
        />
      </FormControl>
    );
  }

  return null;
}

export default CommonStepPropertyFactory;
