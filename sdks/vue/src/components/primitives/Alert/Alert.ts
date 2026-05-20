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
import {type Component, type SetupContext, type VNode, defineComponent, h, type PropType} from 'vue';

type AlertProps = Readonly<{
  dismissible: boolean;
  severity: 'success' | 'error' | 'warning' | 'info';
}>;

const Alert: Component = defineComponent({
  name: 'Alert',
  props: {
    dismissible: {default: false, type: Boolean},
    severity: {
      default: 'info',
      type: String as PropType<'success' | 'error' | 'warning' | 'info'>,
    },
  },
  emits: ['dismiss'],
  setup(props: AlertProps, {slots, emit, attrs}: SetupContext): () => VNode {
    return (): VNode =>
      h(
        'div',
        {
          class: [
            withVendorCSSClassPrefix('alert'),
            withVendorCSSClassPrefix(`alert--${props.severity}`),
            (attrs.class as string) || '',
          ]
            .filter(Boolean)
            .join(' '),
          role: 'alert',
          style: attrs.style,
        },
        [
          h('div', {class: withVendorCSSClassPrefix('alert__content')}, slots['default']?.()),
          props.dismissible
            ? h(
                'button',
                {
                  'aria-label': 'Dismiss',
                  class: withVendorCSSClassPrefix('alert__dismiss'),
                  onClick: () => emit('dismiss'),
                  type: 'button',
                },
                '\u00d7',
              )
            : null,
        ],
      );
  },
});

export default Alert;
