// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {Box, FormHelperText, InputLabel, OutlinedInput} from '@wso2/oxygen-ui';
import {useEdges, useNodes} from '@xyflow/react';
import {type CSSProperties, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Hint} from '../../hint';
import TemplatePlaceholder, {containsTemplateLiteral} from '../TemplatePlaceholder';
import type {Element as FlowElement} from '@/features/flows/models/elements';
import resolveUpstreamOtpLength from '@/features/flows/utils/resolveUpstreamOtpLength';

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
  /**
   * The step id the element resides on, used to locate the Generate OTP node that feeds it.
   */
  stepId: string;
}

/**
 * Adapter for the OTP inputs.
 *
 * @param props - Props injected to the component.
 * @returns The OTPInputAdapter component.
 */
function OTPInputAdapter({resource, stepId}: OTPInputAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();
  const nodes = useNodes();
  const edges = useEdges();

  const otpElement = resource as OTPInputElement;
  const otpLength = resolveUpstreamOtpLength(stepId, nodes, edges);

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
        {Array.from({length: otpLength}, (_, index) => (
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
