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

import {EmbeddedFlowType} from '@thunderid/browser';
import {type Component, type PropType, type SetupContext, type VNode, defineComponent, h} from 'vue';
import BaseInviteUser from './BaseInviteUser';
import type {BaseInviteUserRenderProps, InviteUserFlowResponse} from './BaseInviteUser';
import useThunderID from '../../../composables/useThunderID';

export type InviteUserRenderProps = BaseInviteUserRenderProps;

/**
 * InviteUser — admin invite component using authenticated ThunderID SDK context.
 */
const InviteUser: Component = defineComponent({
  name: 'InviteUser',
  props: {
    className: {default: '', type: String},
    onError: {default: undefined, type: Function as PropType<(error: Error) => void>},
    onFlowChange: {
      default: undefined,
      type: Function as PropType<(response: InviteUserFlowResponse) => void>,
    },
    onInviteLinkGenerated: {
      default: undefined,
      type: Function as PropType<(inviteLink: string, flowId: string) => void>,
    },
    showSubtitle: {default: true, type: Boolean},
    showTitle: {default: true, type: Boolean},
    size: {default: 'medium', type: String as PropType<'small' | 'medium' | 'large'>},
    variant: {default: 'outlined', type: String as PropType<'outlined' | 'elevated'>},
  },
  setup(props: any, {slots}: SetupContext): () => VNode | null {
    const {http, baseUrl, isInitialized} = useThunderID();

    const handleInitialize = async (payload: Record<string, any>): Promise<InviteUserFlowResponse> => {
      const response: any = await http.request({
        data: {...payload, flowType: EmbeddedFlowType.UserOnboarding, verbose: true},
        headers: {Accept: 'application/json', 'Content-Type': 'application/json'},
        method: 'POST',
        url: `${baseUrl}/flow/execute`,
      } as any);
      return response.data as InviteUserFlowResponse;
    };

    const handleSubmit = async (payload: Record<string, any>): Promise<InviteUserFlowResponse> => {
      const response: any = await http.request({
        data: {...payload, verbose: true},
        headers: {Accept: 'application/json', 'Content-Type': 'application/json'},
        method: 'POST',
        url: `${baseUrl}/flow/execute`,
      } as any);
      return response.data as InviteUserFlowResponse;
    };

    return (): VNode | null =>
      h(
        BaseInviteUser,
        {
          className: props.className,
          isInitialized: isInitialized?.value ?? false,
          onError: props.onError,
          onFlowChange: props.onFlowChange,
          onInitialize: handleInitialize,
          onInviteLinkGenerated: props.onInviteLinkGenerated,
          onSubmit: handleSubmit,
          showSubtitle: props.showSubtitle,
          showTitle: props.showTitle,
          size: props.size,
          variant: props.variant,
        },
        slots['default'] ? {default: (renderProps: any) => slots['default'](renderProps)} : undefined,
      );
  },
});

export default InviteUser;
