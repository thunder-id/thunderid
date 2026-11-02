// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {FormHelperText, TextField} from '@wso2/oxygen-ui';
import {type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {Element as FlowElement} from '../../../../../models/elements';
import {Hint} from '../../hint';
import TemplatePlaceholder, {containsTemplateLiteral} from '../TemplatePlaceholder';

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
