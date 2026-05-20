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

import {
  FlowMetadataResponse,
  FlowMetaType,
  getFlowMetaV2,
  I18nBundle,
  TranslationBundleConstants,
} from '@thunderid/browser';
import {
  defineComponent,
  h,
  inject,
  onMounted,
  provide,
  readonly,
  shallowReadonly,
  ref,
  watch,
  type Component,
  type Ref,
  type SetupContext,
  type VNode,
} from 'vue';
import {THUNDERID_KEY, FLOW_META_KEY, I18N_KEY} from '../keys';
import type {ThunderIDContext, FlowMetaContextValue, I18nContextValue} from '../models/contexts';

/**
 * FlowMetaProvider fetches flow metadata from the `GET /flow/meta` endpoint
 * (v2 API) and makes it available via `useFlowMeta()`.
 *
 * It also integrates with `I18nProvider` so that server-side translations
 * from the metadata are automatically injected into the i18n system.
 *
 * @internal — This provider is mounted automatically by `<ThunderIDProvider>`.
 */
interface FlowMetaProviderProps {
  enabled: boolean;
}

const FlowMetaProvider: Component = defineComponent({
  name: 'FlowMetaProvider',
  props: {
    /**
     * When false the provider skips fetching and provides null meta.
     * @default true
     */
    enabled: {default: true, type: Boolean},
  },
  setup(props: FlowMetaProviderProps, {slots}: SetupContext): () => VNode {
    const thunderIDContext: ThunderIDContext | undefined = inject(THUNDERID_KEY);
    const i18nContext: I18nContextValue | null = inject(I18N_KEY, null);

    const meta: Ref<FlowMetadataResponse | null> = ref(null);
    const isLoading: Ref<boolean> = ref(false);
    const error: Ref<Error | null> = ref(null);
    const pendingLanguage: Ref<string | null> = ref(null);

    const baseUrl: string | undefined = thunderIDContext?.baseUrl;
    const applicationId: string | undefined = thunderIDContext?.applicationId;

    const fetchFlowMeta = async (): Promise<void> => {
      if (!props.enabled) {
        meta.value = null;
        return;
      }

      isLoading.value = true;
      error.value = null;

      try {
        const result: FlowMetadataResponse = await getFlowMetaV2({
          baseUrl,
          id: applicationId,
          type: FlowMetaType.App,
        });
        meta.value = result;
      } catch (err: unknown) {
        error.value = err instanceof Error ? err : new Error(String(err));
      } finally {
        isLoading.value = false;
      }
    };

    const switchLanguage = async (language: string): Promise<void> => {
      if (!props.enabled) return;

      isLoading.value = true;
      error.value = null;

      try {
        const result: FlowMetadataResponse = await getFlowMetaV2({
          baseUrl,
          id: applicationId,
          language,
          type: FlowMetaType.App,
        });

        // Inject translations before switching language so the i18n state is updated
        if (result.i18n?.translations && i18nContext?.injectBundles) {
          const flatTranslations: Record<string, string> = {};
          Object.entries(result.i18n.translations).forEach(([namespace, keys]: [string, Record<string, string>]) => {
            Object.entries(keys).forEach(([key, value]: [string, string]) => {
              flatTranslations[`${namespace}.${key}`] = value;
            });
          });
          const bundle: I18nBundle = {translations: flatTranslations} as unknown as I18nBundle;
          i18nContext.injectBundles({[language]: bundle});
        }

        // Defer setLanguage so that injectBundles' state is committed first
        pendingLanguage.value = language;
        meta.value = result;
      } catch (err: unknown) {
        error.value = err instanceof Error ? err : new Error(String(err));
      } finally {
        isLoading.value = false;
      }
    };

    // After injectBundles + pendingLanguage are committed, call setLanguage
    watch(pendingLanguage, (lang: string | null) => {
      if (lang && i18nContext?.setLanguage) {
        i18nContext.setLanguage(lang);
        pendingLanguage.value = null;
      }
    });

    // When meta loads with i18n translations, inject them into the i18n system
    watch(
      () => meta.value?.i18n?.translations,
      (translations: Record<string, Record<string, string>> | undefined) => {
        if (!translations || !i18nContext?.injectBundles) return;

        const metaLanguage: string = (meta.value?.i18n as any)?.language || TranslationBundleConstants.FALLBACK_LOCALE;

        const flatTranslations: Record<string, string> = {};
        Object.entries(translations).forEach(([namespace, keys]: [string, Record<string, string>]) => {
          Object.entries(keys).forEach(([key, value]: [string, string]) => {
            flatTranslations[`${namespace}.${key}`] = value;
          });
        });

        const bundle: I18nBundle = {translations: flatTranslations} as unknown as I18nBundle;
        const bundlesToInject: Record<string, I18nBundle> = {[metaLanguage]: bundle};

        const currentLang: string = i18nContext.currentLanguage.value;
        const fallbackLang: string = i18nContext.fallbackLanguage;

        if (currentLang && currentLang !== metaLanguage) {
          bundlesToInject[currentLang] = bundle;
        }
        if (fallbackLang && fallbackLang !== metaLanguage) {
          bundlesToInject[fallbackLang] = bundle;
        }

        i18nContext.injectBundles(bundlesToInject);
      },
    );

    onMounted(() => {
      fetchFlowMeta();
    });

    const context: FlowMetaContextValue = {
      error: readonly(error),
      fetchFlowMeta,
      isLoading: readonly(isLoading),
      meta: shallowReadonly(meta),
      switchLanguage,
    };

    provide(FLOW_META_KEY, context);

    return () => h('div', {style: 'display:contents'}, slots['default']?.());
  },
});

export default FlowMetaProvider;
