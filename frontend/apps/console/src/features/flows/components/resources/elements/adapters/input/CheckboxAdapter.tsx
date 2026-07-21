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
import {Checkbox, FormControlLabel, FormHelperText} from '@wso2/oxygen-ui';
import {type CSSProperties, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import {Hint} from '../../hint';
import type {Element as FlowElement} from '@/features/flows/models/elements';

/**
 * Checkbox element type with properties at top level.
 */
export type CheckboxElement = FlowElement & {
  defaultValue?: string;
  label?: string;
  required?: boolean;
  styles?: CSSProperties;
  hint?: string;
};

/**
 * Props interface of {@link CheckboxAdapter}
 */
export interface CheckboxAdapterPropsInterface {
  /**
   * The checkbox element properties.
   */
  resource: FlowElement;
}

/**
 * Adapter for the Checkbox component.
 *
 * @param props - Props injected to the component.
 * @returns The CheckboxAdapter component.
 */
function CheckboxAdapter({resource}: CheckboxAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();

  const checkboxElement = resource as CheckboxElement;

  return (
    <div id={checkboxElement?.id}>
      <FormControlLabel
        control={<Checkbox defaultChecked />}
        className={checkboxElement?.classes}
        label={resolve(checkboxElement?.label, {t}) ?? checkboxElement?.label ?? ''}
        required={checkboxElement?.required}
        style={checkboxElement?.styles}
      />
      {checkboxElement?.hint && (
        <FormHelperText>
          <Hint hint={checkboxElement?.hint} />
        </FormHelperText>
      )}
    </div>
  );
}

export default CheckboxAdapter;
