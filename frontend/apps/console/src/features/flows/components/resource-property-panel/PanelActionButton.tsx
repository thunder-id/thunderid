/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {Button, type ButtonProps} from '@wso2/oxygen-ui';
import type {ReactElement} from 'react';

/**
 * Props interface of {@link PanelActionButton}
 */
export interface PanelActionButtonProps extends Omit<ButtonProps, 'sx' | 'variant' | 'size' | 'fullWidth'> {
  /**
   * Accent palette used for the start icon and the hover border.
   */
  accent?: 'primary' | 'error';
}

/**
 * Action button for the resource properties panel, styled after the flow
 * preview's option rows: a full-width bordered row with a left-aligned label,
 * an accent-colored start icon, and an accent border on hover.
 *
 * @param props - Props injected to the component.
 * @returns The PanelActionButton component.
 */
function PanelActionButton({accent = 'primary', children, ...rest}: PanelActionButtonProps): ReactElement {
  return (
    <Button
      fullWidth
      size="small"
      sx={{
        textTransform: 'none',
        justifyContent: 'flex-start',
        px: 1.5,
        py: 0.75,
        borderRadius: 1.5,
        border: '1px solid',
        borderColor: 'divider',
        color: 'text.primary',
        fontWeight: 500,
        '& .MuiButton-startIcon': {color: `${accent}.main`},
        '&:hover': {
          borderColor: `${accent}.main`,
          bgcolor: 'action.hover',
        },
      }}
      {...rest}
    >
      {children}
    </Button>
  );
}

export default PanelActionButton;
