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

import {withVendorCSSClassPrefix} from '@thunderid/browser';
import {type Component, type VNode, defineComponent, h} from 'vue';
import Button from '../primitives/Button';

/**
 * BaseSignInButton — styled sign-in button with customization support.
 *
 * By default, renders a styled Button primitive with contents from the slot or fallback text.
 * Set `unstyled={true}` to render a plain <button> for full customization control.
 *
 * @example
 * <!-- Default styled button with custom text -->
 * <BaseSignInButton>Custom Text</BaseSignInButton>
 *
 * @example
 * <!-- Unstyled button for full customization -->
 * <BaseSignInButton unstyled class="my-custom-styles">Custom Content</BaseSignInButton>
 */
const BaseSignInButton: Component = defineComponent({
  name: 'BaseSignInButton',
  props: {
    disabled: {
      default: false,
      type: Boolean,
    },
    isLoading: {
      default: false,
      type: Boolean,
    },
    /**
     * When true, renders a plain <button> with no default styling.
     * When false (default), renders a styled Button component.
     */
    unstyled: {
      default: false,
      type: Boolean,
    },
  },
  emits: ['click'],
  setup(props: any, {slots, emit, attrs}: any): any {
    const handleClick = (e: MouseEvent): void => {
      if (!props.disabled && !props.isLoading) {
        emit('click', e);
      }
    };

    return (): any => {
      // Unstyled mode: plain button for full customization
      if (props.unstyled) {
        return h(
          'button',
          {
            class: [withVendorCSSClassPrefix('sign-in-button-wrapper'), (attrs.class as string) || '']
              .filter(Boolean)
              .join(' '),
            disabled: props.disabled || props.isLoading,
            onClick: handleClick,
            style: attrs.style,
            type: 'button' as const,
          },
          slots.default ? slots.default({isLoading: props.isLoading}) : 'Sign In',
        );
      }

      // Styled mode (default): always render the styled Button with slot/fallback content
      return h(
        Button,
        {
          class: [withVendorCSSClassPrefix('sign-in-button'), (attrs.class as string) || ''].filter(Boolean).join(' '),
          disabled: props.disabled || props.isLoading,
          loading: props.isLoading,
          onClick: handleClick,
          style: attrs.style,
          type: 'button' as const,
        },
        slots.default ? (): VNode | VNode[] => slots.default({isLoading: props.isLoading}) : (): string => 'Sign In',
      );
    };
  },
});

export default BaseSignInButton;
