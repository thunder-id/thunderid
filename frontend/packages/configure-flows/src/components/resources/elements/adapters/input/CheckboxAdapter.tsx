// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {Checkbox, FormControlLabel, FormHelperText} from '@wso2/oxygen-ui';
import {type CSSProperties, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import type {Element as FlowElement} from '../../../../../models/elements';
import {Hint} from '../../hint';

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
