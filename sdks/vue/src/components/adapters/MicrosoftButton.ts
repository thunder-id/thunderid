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

import {defineComponent, h, type Component, type SetupContext, type VNode} from 'vue';
import useI18n from '../../composables/useI18n';
import Button from '../primitives/Button';

/**
 * Microsoft Sign-In Button Component.
 * Handles authentication with Microsoft identity provider.
 */
const MicrosoftButton: Component = defineComponent({
  name: 'MicrosoftButton',
  props: {
    isLoading: {default: false, type: Boolean},
  },
  emits: ['click'],
  setup(props: {isLoading: boolean}, {slots, emit, attrs}: SetupContext): () => VNode {
    const {t} = useI18n();

    const microsoftIcon = (): VNode =>
      h('svg', {height: '14', viewBox: '0 0 23 23', width: '14', xmlns: 'http://www.w3.org/2000/svg'}, [
        h('path', {d: 'M0 0h23v23H0z', fill: '#f3f3f3'}),
        h('path', {d: 'M1 1h10v10H1z', fill: '#f35325'}),
        h('path', {d: 'M12 1h10v10H12z', fill: '#81bc06'}),
        h('path', {d: 'M1 12h10v10H1z', fill: '#05a6f0'}),
        h('path', {d: 'M12 12h10v10H12z', fill: '#ffba08'}),
      ]);

    return (): VNode =>
      h(
        Button,
        {
          ...attrs,
          color: 'secondary' as const,
          disabled: props.isLoading,
          fullWidth: true,
          type: 'button' as const,
          variant: 'solid' as const,
          ...(slots['default'] ? {} : {startIcon: microsoftIcon()}),
          onClick: (e: MouseEvent) => emit('click', e),
        },
        () =>
          slots['default']?.({isLoading: props.isLoading}) ??
          (t('elements.buttons.microsoft.text') || 'Sign in with Microsoft'),
      );
  },
});

export default MicrosoftButton;
