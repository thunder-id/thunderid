// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DateTimePickerField} from '@thunderid/design';
import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {useState, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {InputElement} from './DefaultInputAdapter';
import type {Element as FlowElement} from '../../../../../models/elements';
import TemplatePlaceholder, {containsTemplateLiteral} from '../TemplatePlaceholder';

interface DateTimeInputAdapterProps {
  resource: FlowElement;
}

export default function DateTimeInputAdapter({resource}: DateTimeInputAdapterProps): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();
  const input = resource as InputElement;
  const [value, setValue] = useState<string | null>(input.defaultValue ?? null);
  const [prevDefaultValue, setPrevDefaultValue] = useState(input.defaultValue);

  // Reset the picker when the configured default changes (adjust state during render, not in an effect).
  if (input.defaultValue !== prevDefaultValue) {
    setPrevDefaultValue(input.defaultValue);
    setValue(input.defaultValue ?? null);
  }

  const rawLabel = input.label ?? '';
  const label: ReactNode = containsTemplateLiteral(rawLabel) ? (
    <TemplatePlaceholder value={rawLabel} t={t} />
  ) : (
    (resolve(rawLabel, {t}) ?? rawLabel)
  );

  return (
    <>
      <input type="hidden" name={input.id} value={value ?? ''} />
      <DateTimePickerField
        id={`${input.id}-date-time`}
        value={value}
        label={label}
        required={input.required}
        helperText={input.hint}
        onChange={setValue}
      />
    </>
  );
}
