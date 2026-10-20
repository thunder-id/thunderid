// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {cn} from '@thunderid/utils';
import {Button} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {FlowComponent} from '../../../models/flow';

export interface SubmitButtonAdapterProps {
  component: FlowComponent;
  /** Disables the button, while the flow is loading or the step cannot be submitted yet. */
  disabled: boolean;
  resolve: (template: string | undefined) => string | undefined;
  /** When provided, the button fires its own action via onClick instead of form submit */
  onClick?: () => void;
}

export default function SubmitButtonAdapter({
  component,
  disabled,
  resolve,
  onClick = undefined,
}: SubmitButtonAdapterProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Button
      id={component.id}
      type={onClick ? 'button' : 'submit'}
      fullWidth
      className={cn(
        'Flow--submitButton',
        'Button--root',
        component.variant === 'PRIMARY' ? 'Button--primary' : 'Button--outlined',
      )}
      variant={component.variant === 'PRIMARY' ? 'contained' : 'outlined'}
      disabled={disabled}
      onClick={onClick}
      sx={{mt: 2}}
    >
      {t(resolve(component.label)!)}
    </Button>
  );
}
