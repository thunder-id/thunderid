// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {FormControl, FormLabel, MenuItem, Select} from '@wso2/oxygen-ui';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import type {Element as FlowElement} from '../../../../../models/elements';
import {Hint} from '../../hint';

/**
 * Select element type with properties at top level.
 */
export type SelectElement = FlowElement & {
  hint?: string;
  label?: string;
  placeholder?: string;
  required?: boolean;
  options?: unknown[];
};

/**
 * Props interface of {@link SelectAdapter}
 */
export interface SelectAdapterPropsInterface {
  /**
   * The select element properties.
   */
  resource: FlowElement;
}

/**
 * Adapter for SELECT input elements in the flow builder.
 * Renders a dropdown preview. Options are populated dynamically
 * at runtime via ForwardedData from upstream executors.
 *
 * @param props - Props injected to the component.
 * @returns The SelectAdapter component.
 */
function SelectAdapter({resource}: SelectAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();

  const selectElement = resource as SelectElement;
  const label = resolve(selectElement?.label, {t}) ?? selectElement?.label ?? '';
  const placeholder = resolve(selectElement?.placeholder, {t}) ?? selectElement?.placeholder ?? '';

  return (
    <FormControl fullWidth>
      <FormLabel required={selectElement?.required}>{label}</FormLabel>
      <Select displayEmpty size="small" fullWidth value="">
        <MenuItem value="" disabled>
          {placeholder}
        </MenuItem>
      </Select>
      {selectElement?.hint && <Hint hint={selectElement.hint} />}
    </FormControl>
  );
}

export default SelectAdapter;
