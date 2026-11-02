// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {FormControl, FormControlLabel, FormHelperText, FormLabel, Radio, RadioGroup} from '@wso2/oxygen-ui';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import type {FieldOption} from '../../../../models/base';
import type {Element as FlowElement} from '../../../../models/elements';
import {Hint} from '../hint';

/**
 * Choice element type with properties at top level.
 */
export type ChoiceElement = FlowElement & {
  defaultValue?: string;
  options?: FieldOption[];
  label?: string;
  hint?: string;
};

/**
 * Props interface of {@link ChoiceAdapter}
 */
export interface ChoiceAdapterPropsInterface {
  /**
   * The choice element properties.
   */
  resource: FlowElement;
}

/**
 * Adapter for the Choice component that renders a radio group.
 *
 * @param props - Props injected to the component.
 * @returns The ChoiceAdapter component.
 */
function ChoiceAdapter({resource}: ChoiceAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();
  const choiceElement = resource as ChoiceElement;

  return (
    <FormControl sx={{my: 2}}>
      <FormLabel id={choiceElement?.id}>{resolve(choiceElement?.label, {t}) ?? choiceElement?.label ?? ''}</FormLabel>
      <RadioGroup defaultValue={choiceElement?.defaultValue}>
        {choiceElement?.options?.map((option: FieldOption) => (
          <FormControlLabel
            key={option?.key}
            value={option?.value}
            control={<Radio />}
            label={resolve(option?.label, {t}) ?? option?.label ?? ''}
          />
        ))}
      </RadioGroup>
      {choiceElement?.hint && (
        <FormHelperText>
          <Hint hint={choiceElement?.hint} />
        </FormHelperText>
      )}
    </FormControl>
  );
}

export default ChoiceAdapter;
