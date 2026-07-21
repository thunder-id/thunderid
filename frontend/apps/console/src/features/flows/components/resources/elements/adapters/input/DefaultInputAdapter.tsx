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
import {TextField} from '@wso2/oxygen-ui';
import {type CSSProperties, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Hint} from '../../hint';
import TemplatePlaceholder, {containsTemplateLiteral} from '../TemplatePlaceholder';
import type {Element as FlowElement} from '@/features/flows/models/elements';

/**
 * Input element type with properties at top level.
 */
export type InputElement = FlowElement & {
  defaultValue?: string;
  hint?: string;
  maxLength?: number;
  minLength?: number;
  label?: string;
  multiline?: boolean;
  placeholder?: string;
  required?: boolean;
  inputType?: string;
  styles?: CSSProperties;
};

/**
 * Props interface of {@link DefaultInputAdapter}
 */
export interface DefaultInputAdapterPropsInterface {
  /**
   * The input element properties.
   */
  resource: FlowElement;
}

/**
 * Fallback adapter for the inputs.
 *
 * @param props - Props injected to the component.
 * @returns The DefaultInputAdapter component.
 */
function DefaultInputAdapter({resource}: DefaultInputAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();

  const inputElement = resource as InputElement;

  const rawLabel = inputElement?.label ?? '';
  const labelNode: ReactNode = containsTemplateLiteral(rawLabel) ? (
    <TemplatePlaceholder value={rawLabel} t={t} />
  ) : (
    (resolve(rawLabel, {t}) ?? rawLabel)
  );

  return (
    <TextField
      fullWidth
      id={inputElement?.id}
      className={inputElement?.classes}
      defaultValue={inputElement?.defaultValue}
      helperText={inputElement?.hint && <Hint hint={inputElement?.hint} />}
      inputProps={{
        maxLength: inputElement?.maxLength,
        minLength: inputElement?.minLength,
      }}
      label={labelNode}
      multiline={inputElement?.multiline}
      placeholder={resolve(inputElement?.placeholder, {t}) ?? inputElement?.placeholder ?? ''}
      required={inputElement?.required}
      InputLabelProps={{
        required: inputElement?.required,
      }}
      type={inputElement?.inputType}
      style={inputElement?.styles}
      autoComplete={inputElement?.inputType === 'password' ? 'new-password' : 'off'}
    />
  );
}

export default DefaultInputAdapter;
