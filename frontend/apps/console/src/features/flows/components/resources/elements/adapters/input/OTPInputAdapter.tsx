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
import {Box, FormHelperText, InputLabel, OutlinedInput} from '@wso2/oxygen-ui';
import {type CSSProperties, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Hint} from '../../hint';
import TemplatePlaceholder, {containsTemplateLiteral} from '../TemplatePlaceholder';
import type {Element as FlowElement} from '@/features/flows/models/elements';

/**
 * OTP Input element type with properties at top level.
 */
export type OTPInputElement = FlowElement & {
  label?: string;
  required?: boolean;
  inputType?: string;
  styles?: CSSProperties;
  placeholder?: string;
  hint?: string;
};

/**
 * Props interface of {@link OTPInputAdapter}
 */
export interface OTPInputAdapterPropsInterface {
  /**
   * The OTP input element properties.
   */
  resource: FlowElement;
}

/**
 * Adapter for the OTP inputs.
 *
 * @param props - Props injected to the component.
 * @returns The OTPInputAdapter component.
 */
function OTPInputAdapter({resource}: OTPInputAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();

  const otpElement = resource as OTPInputElement;

  const rawLabel = otpElement?.label ?? '';
  const labelNode: ReactNode = containsTemplateLiteral(rawLabel) ? (
    <TemplatePlaceholder value={rawLabel} t={t} />
  ) : (
    (resolve(rawLabel, {t}) ?? rawLabel)
  );

  return (
    <div id={otpElement?.id} className={otpElement?.classes}>
      <InputLabel htmlFor="otp-input-adapter" required={otpElement?.required} disableAnimation>
        {labelNode}
      </InputLabel>
      <Box display="flex" flexDirection="row" gap={1}>
        {Array.from({length: 6}, (_, index) => (
          <OutlinedInput
            key={index}
            size="small"
            id="otp-input-adapter"
            type={otpElement?.inputType}
            style={otpElement?.styles}
            placeholder={resolve(otpElement?.placeholder, {t}) ?? otpElement?.placeholder ?? ''}
          />
        ))}
      </Box>
      {otpElement?.hint && (
        <FormHelperText>
          <Hint hint={otpElement?.hint} />
        </FormHelperText>
      )}
    </div>
  );
}

export default OTPInputAdapter;
