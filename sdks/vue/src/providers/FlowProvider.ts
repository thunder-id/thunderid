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

import {
  defineComponent,
  h,
  provide,
  readonly,
  shallowReadonly,
  ref,
  type Component,
  type PropType,
  type Ref,
  type SetupContext,
  type VNode,
} from 'vue';
import {FLOW_KEY} from '../keys';
import type {FlowContextValue, FlowMessage, FlowStep} from '../models/contexts';

/**
 * FlowProvider manages authentication flow UI state and makes it available
 * via `useFlow()`.
 *
 * It tracks the current step, title, subtitle, messages, loading state,
 * and back-navigation callback for embedded authentication flows.
 *
 * @internal — This provider is mounted automatically by `<ThunderIDProvider>`.
 */
interface FlowProviderProps {
  initialStep: FlowStep | null;
  initialSubtitle: string | undefined;
  initialTitle: string;
  onFlowChange: ((step: FlowStep) => void) | undefined;
}

const FlowProvider: Component = defineComponent({
  name: 'FlowProvider',
  props: {
    /** Initial step to start with. */
    initialStep: {default: null, type: Object as PropType<FlowStep>},
    /** Initial subtitle. */
    initialSubtitle: {default: undefined, type: String},
    /** Initial title. */
    initialTitle: {default: '', type: String},
    /** Callback when the flow step changes. */
    onFlowChange: {default: undefined, type: Function as PropType<(step: FlowStep) => void>},
  },
  setup(props: FlowProviderProps, {slots}: SetupContext): () => VNode {
    const currentStep: Ref<FlowStep> = ref(props.initialStep ?? null);
    const title: Ref<string> = ref(props.initialTitle ?? '');
    const subtitle: Ref<string | undefined> = ref(props.initialSubtitle);
    const messages: Ref<FlowMessage[]> = ref([]);
    const error: Ref<string | null> = ref(null);
    const isLoading: Ref<boolean> = ref(false);
    const showBackButton: Ref<boolean> = ref(false);
    const onGoBack: Ref<(() => void) | undefined> = ref(undefined);

    const setCurrentStep = (step: FlowStep): void => {
      currentStep.value = step;
      if (step) {
        title.value = step.title;
        subtitle.value = step.subtitle;
        showBackButton.value = step.canGoBack ?? false;
      }
      props.onFlowChange?.(step);
    };

    const setTitle = (newTitle: string): void => {
      title.value = newTitle;
    };

    const setSubtitle = (newSubtitle?: string): void => {
      subtitle.value = newSubtitle;
    };

    const setError = (newError: string | null): void => {
      error.value = newError;
    };

    const setIsLoading = (loading: boolean): void => {
      isLoading.value = loading;
    };

    const setShowBackButton = (show: boolean): void => {
      showBackButton.value = show;
    };

    const setOnGoBack = (callback?: () => void): void => {
      onGoBack.value = callback;
    };

    const addMessage = (message: FlowMessage): void => {
      const messageWithId: FlowMessage = {
        ...message,
        id: message.id ?? `msg-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
      };
      messages.value = [...messages.value, messageWithId];
    };

    const removeMessage = (messageId: string): void => {
      messages.value = messages.value.filter((msg: FlowMessage): boolean => msg.id !== messageId);
    };

    const clearMessages = (): void => {
      messages.value = [];
    };

    const reset = (): void => {
      currentStep.value = props.initialStep ?? null;
      title.value = props.initialTitle ?? '';
      subtitle.value = props.initialSubtitle;
      messages.value = [];
      error.value = null;
      isLoading.value = false;
      showBackButton.value = false;
      onGoBack.value = undefined;
    };

    const navigateToFlow = (
      flowType: NonNullable<FlowStep>['type'],
      options?: {metadata?: Record<string, any>; subtitle?: string; title?: string},
    ): void => {
      const stepId = `${flowType}-${Date.now()}`;
      const step: NonNullable<FlowStep> = {
        canGoBack: flowType !== 'signin',
        id: stepId,
        metadata: options?.metadata,
        subtitle: options?.subtitle,
        title: options?.title ?? '',
        type: flowType,
      };
      setCurrentStep(step);
      clearMessages();
      error.value = null;
    };

    const context: FlowContextValue = {
      addMessage,
      clearMessages,
      currentStep: readonly(currentStep),
      error: readonly(error),
      isLoading: readonly(isLoading),
      messages: shallowReadonly(messages),
      navigateToFlow,
      onGoBack: readonly(onGoBack),
      removeMessage,
      reset,
      setCurrentStep,
      setError,
      setIsLoading,
      setOnGoBack,
      setShowBackButton,
      setSubtitle,
      setTitle,
      showBackButton: readonly(showBackButton),
      subtitle: readonly(subtitle),
      title: readonly(title),
    };

    provide(FLOW_KEY, context);

    return () => h('div', {style: 'display:contents'}, slots['default']?.());
  },
});

export default FlowProvider;
