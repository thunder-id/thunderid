// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {Stack} from '@wso2/oxygen-ui';
import {InfoIcon} from '@wso2/oxygen-ui-icons-react';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props interface of {@link Hint}
 */
export interface HintPropsInterface {
  /**
   * Hint text to be displayed.
   */
  hint: string;
}

/**
 * Hint component to display additional information for input fields.
 *
 * @param props - Props injected to the component.
 * @returns The Hint component.
 */
export function Hint({hint}: HintPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();

  return (
    <Stack direction="row" gap={0.5} alignItems="center" justifyContent="flex-start">
      <InfoIcon size={12} />
      <span>{resolve(hint, {t}) ?? hint}</span>
    </Stack>
  );
}

export default Hint;
