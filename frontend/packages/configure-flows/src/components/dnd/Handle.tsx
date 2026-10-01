// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Tooltip} from '@wso2/oxygen-ui';
import type {CSSProperties, HTMLAttributes, ReactNode, Ref} from 'react';
import Action from './Action';

export interface HandleProps extends HTMLAttributes<HTMLButtonElement> {
  cursor?: CSSProperties['cursor'];
  label: ReactNode;
  ref?: Ref<HTMLButtonElement>;
}

function Handle({children, label, cursor = 'pointer', ref = null, ...rest}: HandleProps) {
  return (
    <Action ref={ref} cursor={cursor} {...rest}>
      <Tooltip title={label}>
        <span>{children}</span>
      </Tooltip>
    </Action>
  );
}

export default Handle;
