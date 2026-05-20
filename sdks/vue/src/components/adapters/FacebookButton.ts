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
 * Facebook Sign-In Button Component.
 * Handles authentication with Facebook identity provider.
 */
const FacebookButton: Component = defineComponent({
  name: 'FacebookButton',
  props: {
    isLoading: {default: false, type: Boolean},
  },
  emits: ['click'],
  setup(props: {isLoading: boolean}, {slots, emit, attrs}: SetupContext): () => VNode {
    const {t} = useI18n();

    const facebookIcon = (): VNode =>
      h('svg', {height: '18', viewBox: '0 0 512 512', width: '18', xmlns: 'http://www.w3.org/2000/svg'}, [
        h('path', {
          d: 'M448,0H64C28.704,0,0,28.704,0,64v384c0,35.296,28.704,64,64,64h384c35.296,0,64-28.704,64-64V64C512,28.704,483.296,0,448,0z',
          fill: '#1976D2',
        }),
        h('path', {
          d: 'M432,256h-80v-64c0-17.664,14.336-16,32-16h32V96h-64l0,0c-53.024,0-96,42.976-96,96v64h-64v80h64v176h96V336h48L432,256z',
          fill: '#FAFAFA',
        }),
      ]);

    return (): VNode =>
      h(
        Button,
        {
          ...attrs,
          color: 'primary' as const,
          disabled: props.isLoading,
          fullWidth: true,
          type: 'button' as const,
          variant: 'solid' as const,
          ...(slots['default'] ? {} : {startIcon: facebookIcon()}),
          onClick: (e: MouseEvent) => emit('click', e),
        },
        () =>
          slots['default']?.({isLoading: props.isLoading}) ??
          (t('elements.buttons.facebook.text') || 'Sign in with Facebook'),
      );
  },
});

export default FacebookButton;
