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

import {BrandingPreference, Theme, transformBrandingPreferenceToTheme} from '@thunderid/browser';
import {
  computed,
  defineComponent,
  h,
  provide,
  readonly,
  shallowReadonly,
  ref,
  watch,
  type Component,
  type PropType,
  type Ref,
  type SetupContext,
  type VNode,
} from 'vue';
import {BRANDING_KEY} from '../keys';
import type {BrandingContextValue} from '../models/contexts';

interface BrandingProviderProps {
  brandingPreference: BrandingPreference | null;
  enabled: boolean;
  error: Error | null;
  forceTheme: 'light' | 'dark' | undefined;
  isLoading: boolean;
  refetch: (() => Promise<void>) | undefined;
}

/**
 * BrandingProvider manages branding preference state and makes branding data
 * available to child components via `useBranding()`.
 *
 * It receives branding preferences from a parent component (typically
 * `<ThunderIDProvider>`) and transforms them into `Theme` objects.
 *
 * @internal — This provider is mounted automatically by `<ThunderIDProvider>`.
 */
const BrandingProvider: Component = defineComponent({
  name: 'BrandingProvider',
  props: {
    /** Whether branding is enabled. When false the context provides null. */
    brandingPreference: {
      default: null,
      type: Object as PropType<BrandingPreference | null>,
    },
    /** Loading state from the parent. */
    enabled: {
      default: true,
      type: Boolean,
    },
    error: {
      default: null,
      type: Object as PropType<Error | null>,
    },
    /** Force a specific theme mode, overriding the one declared in branding. */
    forceTheme: {
      default: undefined,
      type: String as PropType<'light' | 'dark'>,
    },
    /** Re-fetch callback from the parent (bypasses dedup). */
    isLoading: {
      default: false,
      type: Boolean,
    },
    refetch: {
      default: undefined,
      type: Function as PropType<() => Promise<void>>,
    },
  },
  setup(props: BrandingProviderProps, {slots}: SetupContext): () => VNode {
    const theme: Ref<Theme | null> = ref(null);
    const activeTheme: Ref<'light' | 'dark' | null> = ref(null);

    // Process branding preference whenever it changes
    const processBranding = (): void => {
      if (!props.enabled || !props.brandingPreference) {
        theme.value = null;
        activeTheme.value = null;
        return;
      }

      const activeThemeFromBranding: string | undefined = (props.brandingPreference as any)?.preference?.theme
        ?.activeTheme;
      if (activeThemeFromBranding) {
        const mode: string = activeThemeFromBranding.toLowerCase();
        activeTheme.value = mode === 'light' || mode === 'dark' ? mode : null;
      } else {
        activeTheme.value = null;
      }

      const transformedTheme: Theme | null = transformBrandingPreferenceToTheme(
        props.brandingPreference,
        props.forceTheme,
      );
      theme.value = transformedTheme;
    };

    watch(() => [props.brandingPreference, props.forceTheme, props.enabled], processBranding, {immediate: true});

    const fetchBranding = async (): Promise<void> => {
      if (props.refetch) {
        await props.refetch();
      }
    };

    const context: BrandingContextValue = {
      activeTheme: readonly(activeTheme),
      brandingPreference: readonly(computed(() => props.brandingPreference)) as Readonly<
        Ref<BrandingPreference | null>
      >,
      error: readonly(computed(() => props.error)) as Readonly<Ref<Error | null>>,
      fetchBranding,
      isLoading: readonly(computed(() => props.isLoading)) as Readonly<Ref<boolean>>,
      refetch: props.refetch ?? fetchBranding,
      theme: shallowReadonly(theme),
    };

    provide(BRANDING_KEY, context);

    return (): VNode => h('div', {style: 'display:contents'}, slots['default']?.());
  },
});

export default BrandingProvider;
