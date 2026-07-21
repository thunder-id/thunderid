/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {Button, useColorScheme, type ButtonProps, type SxProps, type Theme} from '@wso2/oxygen-ui';
import {Position} from '@xyflow/react';
import {useCallback, useMemo, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import NodeHandle from './NodeHandle';
import TemplatePlaceholder, {containsTemplateLiteral} from './TemplatePlaceholder';
import VisualFlowConstants from '@/features/flows/constants/VisualFlowConstants';
import {ButtonVariants, type Element as FlowElement} from '@/features/flows/models/elements';
import resolveStaticResourcePath from '@/features/flows/utils/resolveStaticResourcePath';

/**
 * Full-color brand icons (e.g. Google) must keep their colors; every other icon
 * in the set is monochrome-dark and needs inversion to stay visible in dark mode.
 */
const FULL_COLOR_ICON_PATTERN = /google\.svg$|recaptcha\.png$/;

/**
 * Configuration interface for Button element.
 */
interface ButtonConfig {
  styles?: SxProps<Theme>;
  image?: string;
}

/**
 * Button element type.
 */
export type ButtonElement = FlowElement<ButtonConfig> & {
  variant?: string;
  label?: string;
  image?: string;
  startIcon?: string;
  endIcon?: string;
};

/**
 * Props interface of {@link ButtonAdapter}
 */
export interface ButtonAdapterPropsInterface {
  /**
   * The button element properties.
   */
  resource: FlowElement;
  /**
   * The index of the element in its parent container.
   * Used to trigger handle position updates when elements are reordered.
   * @defaultValue undefined
   */
  elementIndex?: number;
}

/**
 * Adapter for the Button component.
 *
 * @param props - Props injected to the component.
 * @returns The ButtonAdapter component.
 */
function ButtonAdapter({resource, elementIndex = undefined}: ButtonAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();

  const buttonConfig = resource.config as ButtonConfig | undefined;

  const {config, image} = useMemo(() => {
    let buttonProps: ButtonProps = {};
    const buttonImage = '';

    if (resource.variant === ButtonVariants.Primary) {
      buttonProps = {
        color: 'primary',
        fullWidth: true,
        variant: 'contained',
      };
    } else if (resource.variant === ButtonVariants.Secondary) {
      buttonProps = {
        color: 'secondary',
        fullWidth: true,
        variant: 'contained',
      };
    } else if (resource.variant === ButtonVariants.Text) {
      buttonProps = {
        fullWidth: true,
        variant: 'text',
      };
    } else if (resource.variant === ButtonVariants.Outlined) {
      buttonProps = {
        fullWidth: true,
        variant: 'outlined',
      };
    }

    return {config: buttonProps, image: buttonImage};
  }, [resource.variant]);

  // Cast resource to ButtonElement to access label and image properties
  const buttonElement = resource as ButtonElement;

  const {mode, systemMode} = useColorScheme();
  const effectiveMode = mode === 'system' ? systemMode : mode;

  const renderButtonIcon = useCallback(
    (src: string): ReactElement => (
      <img
        src={resolveStaticResourcePath(src)}
        height={20}
        alt=""
        style={{
          filter:
            effectiveMode === 'dark' && !FULL_COLOR_ICON_PATTERN.test(src) ? 'brightness(0.9) invert(1)' : undefined,
        }}
      />
    ),
    [effectiveMode],
  );

  const startIcon = useMemo(() => {
    // Check resource.startIcon first (new format), then resource.image for backwards compatibility,
    // then config.image, then variant default
    if (buttonElement?.startIcon && typeof buttonElement.startIcon === 'string') {
      return renderButtonIcon(buttonElement.startIcon);
    }
    if (buttonElement?.image && typeof buttonElement.image === 'string') {
      return renderButtonIcon(buttonElement.image);
    }
    if (buttonConfig?.image) {
      return renderButtonIcon(buttonConfig.image);
    }
    if (image) {
      return renderButtonIcon(image);
    }
    return undefined;
  }, [buttonElement?.startIcon, buttonElement?.image, buttonConfig?.image, image, renderButtonIcon]);

  const endIcon = useMemo(() => {
    if (buttonElement?.endIcon && typeof buttonElement.endIcon === 'string') {
      return renderButtonIcon(buttonElement.endIcon);
    }
    return undefined;
  }, [buttonElement?.endIcon, renderButtonIcon]);

  const rawLabel = buttonElement?.label ?? '';
  const labelNode: ReactNode = containsTemplateLiteral(rawLabel) ? (
    <TemplatePlaceholder value={rawLabel} t={t} />
  ) : (
    (resolve(rawLabel, {t}) ?? rawLabel)
  );

  return (
    <div className="adapter button-adapter">
      <Button sx={buttonConfig?.styles} startIcon={startIcon} endIcon={endIcon} {...config}>
        {labelNode}
      </Button>
      <NodeHandle
        id={`${resource?.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`}
        type="source"
        position={Position.Right}
        positionKey={elementIndex}
      />
    </div>
  );
}

export default ButtonAdapter;
