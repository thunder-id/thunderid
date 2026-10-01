// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {Box, Button, type ButtonProps, type SxProps, type Theme} from '@wso2/oxygen-ui';
import {Position} from '@xyflow/react';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import NodeHandle from './NodeHandle';
import VisualFlowConstants from '../../../../constants/VisualFlowConstants';
import type {Element as FlowElement} from '../../../../models/elements';

/**
 * Configuration interface for ResendButton element.
 */
interface ResendButtonConfig {
  styles?: SxProps<Theme>;
}

/**
 * ResendButton element type.
 */
export type ResendButtonElement = FlowElement<ResendButtonConfig> & {
  label?: string;
};

/**
 * Props interface of {@link ResendButtonAdapter}
 */
export interface ResendButtonAdapterPropsInterface {
  /**
   * The step id the resource resides on.
   */
  stepId: string;
  /**
   * The resend button element properties.
   */
  resource: FlowElement;
}

/**
 * Adapter for the ResendButton component.
 *
 * @param props - Props injected to the component.
 * @returns The ResendButtonAdapter component.
 */
function ResendButtonAdapter({resource}: ResendButtonAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();

  const config: ButtonProps = {
    color: 'secondary',
    fullWidth: true,
    variant: 'contained',
  };

  const resendConfig = resource.config as ResendButtonConfig | undefined;
  const resendElement = resource as ResendButtonElement;

  return (
    <Box data-testid="resend-button-adapter" sx={{position: 'relative', width: '100%'}}>
      <Button sx={resendConfig?.styles} {...config}>
        {resolve(resendElement?.label, {t}) ?? resendElement?.label ?? ''}
      </Button>
      <NodeHandle
        id={`${resource?.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`}
        type="source"
        position={Position.Right}
      />
    </Box>
  );
}

export default ResendButtonAdapter;
