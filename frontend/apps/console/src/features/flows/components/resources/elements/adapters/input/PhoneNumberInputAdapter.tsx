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
import {FormHelperText, TextField} from '@wso2/oxygen-ui';
import {type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Hint} from '../../hint';
import TemplatePlaceholder, {containsTemplateLiteral} from '../TemplatePlaceholder';
import type {Element as FlowElement} from '@/features/flows/models/elements';

/**
 * Phone Number Input element type with properties at top level.
 */
export type PhoneNumberInputElement = FlowElement & {
  label?: string;
  placeholder?: string;
  required?: boolean;
  hint?: string;
};

/**
 * Props interface of {@link PhoneNumberInputAdapter}
 */
export interface PhoneNumberInputAdapterPropsInterface {
  /**
   * The phone number input element properties.
   */
  resource: FlowElement;
}

/**
 * Adapter for the Phone Number input component.
 *
 * @param props - Props injected to the component.
 * @returns The PhoneNumberInputAdapter component.
 */
function PhoneNumberInputAdapter({resource}: PhoneNumberInputAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();

  const phoneElement = resource as PhoneNumberInputElement;

  const rawLabel = phoneElement?.label ?? '';
  const labelNode: ReactNode = containsTemplateLiteral(rawLabel) ? (
    <TemplatePlaceholder value={rawLabel} t={t} />
  ) : (
    (resolve(rawLabel, {t}) ?? rawLabel)
  );

  return (
    <>
      <TextField
        id={phoneElement?.id}
        className={phoneElement?.classes}
        label={labelNode}
        placeholder={resolve(phoneElement?.placeholder, {t}) ?? phoneElement?.placeholder ?? ''}
        InputLabelProps={{
          required: phoneElement?.required,
        }}
        type="number"
      />
      {phoneElement?.hint && (
        <FormHelperText>
          <Hint hint={phoneElement?.hint} />
        </FormHelperText>
      )}
    </>
  );
}

export default PhoneNumberInputAdapter;
