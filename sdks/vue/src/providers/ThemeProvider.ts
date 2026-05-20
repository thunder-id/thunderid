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
  Theme,
  ThemeConfig,
  ThemeMode,
  ThemePreferences,
  RecursivePartial,
  BrowserThemeDetection,
  DEFAULT_THEME,
  createTheme,
  detectThemeMode,
  createClassObserver,
  createMediaQueryListener,
} from '@thunderid/browser';
import {
  computed,
  defineComponent,
  h,
  inject,
  onBeforeUnmount,
  onMounted,
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
import {BRANDING_KEY, THEME_KEY} from '../keys';
import type {BrandingContextValue, ThemeContextValue} from '../models/contexts';
import {createVueLogger} from '../utils/logger';

const logger: ReturnType<typeof createVueLogger> = createVueLogger('ThemeProvider');

/**
 * ThemeProvider manages theme state and provides it to child components via `useTheme()`.
 *
 * It supports:
 * - Fixed color schemes (`light` | `dark`)
 * - System preference detection (`system`)
 * - CSS-class-based detection (`class`)
 * - Branding-driven mode (`branding`) — inherits the active theme from `BrandingProvider`
 * - Merging server branding theme with local overrides
 * - CSS variable injection onto `document.documentElement`
 *
 * @example
 * ```vue
 * <ThemeProvider mode="system" :inherit-from-branding="true">
 *   <App />
 * </ThemeProvider>
 * ```
 */
interface ThemeProviderProps {
  detection: BrowserThemeDetection;
  inheritFromBranding: boolean;
  mode: ThemeMode | 'branding';
  theme: RecursivePartial<ThemeConfig> | undefined;
}

const ThemeProvider: Component = defineComponent({
  name: 'ThemeProvider',
  props: {
    /** Theme detection configuration (for 'class' or 'system' mode). */
    detection: {default: () => ({}), type: Object as PropType<BrowserThemeDetection>},
    /** Whether to inherit theme from ThunderID branding preference. */
    inheritFromBranding: {default: true as ThemePreferences['inheritFromBranding'], type: Boolean},
    /**
     * The theme mode:
     * - `'light'` | `'dark'`: Fixed color scheme.
     * - `'system'`: Follows OS preference.
     * - `'class'`: Detects theme from CSS classes on `<html>`.
     * - `'branding'`: Follows the active theme from branding preference.
     */
    mode: {
      default: DEFAULT_THEME as ThemeMode | 'branding',
      type: String as PropType<ThemeMode | 'branding'>,
    },
    /** Optional partial theme overrides applied on top of the resolved theme. */
    theme: {default: undefined, type: Object as PropType<RecursivePartial<ThemeConfig>>},
  },
  setup(props: ThemeProviderProps, {slots}: SetupContext): () => VNode {
    // Try to consume branding context – it is optional (BrandingProvider may not be mounted)
    const brandingContext: BrandingContextValue | null = inject(BRANDING_KEY, null);

    const initColorScheme = (): 'light' | 'dark' => {
      if (props.mode === 'light' || props.mode === 'dark') return props.mode;
      if (props.mode === 'branding') return detectThemeMode('system', props.detection);
      return detectThemeMode(props.mode as ThemeMode, props.detection);
    };

    const colorScheme: Ref<'light' | 'dark'> = ref(initColorScheme());

    // Update color scheme when branding's active theme is available
    watch(
      () => (brandingContext as any)?.activeTheme.value,
      (brandingActiveTheme: string | 'light' | 'dark' | undefined): void => {
        if (!props.inheritFromBranding || !brandingActiveTheme) return;
        if (props.mode === 'branding') {
          colorScheme.value = brandingActiveTheme as 'light' | 'dark';
        } else if (props.mode === 'system' && !(brandingContext as any)?.isLoading.value) {
          colorScheme.value = brandingActiveTheme as 'light' | 'dark';
        }
      },
    );

    // Warn if inheritFromBranding is true but no BrandingProvider is present
    if (props.inheritFromBranding && !brandingContext) {
      logger.warn(
        'ThemeProvider: inheritFromBranding is enabled but BrandingProvider is not available. ' +
          'Make sure to wrap your app with BrandingProvider or ThunderIDProvider.',
      );
    }

    // Merge branding theme with user-provided overrides
    const finalThemeConfig: Ref<RecursivePartial<ThemeConfig> | undefined> = computed<
      RecursivePartial<ThemeConfig> | undefined
    >(() => {
      const themeConfig: RecursivePartial<ThemeConfig> | undefined = props.theme;
      const brandingTheme: RecursivePartial<ThemeConfig> | null | undefined = props.inheritFromBranding
        ? (brandingContext as any)?.theme.value
        : null;

      if (!brandingTheme) return themeConfig;

      const brandingThemeConfig: RecursivePartial<ThemeConfig> = {
        borderRadius: brandingTheme.borderRadius,
        colors: brandingTheme.colors,
        components: brandingTheme.components,
        images: brandingTheme.images,
        shadows: brandingTheme.shadows,
        spacing: brandingTheme.spacing,
      };

      return {
        ...brandingThemeConfig,
        ...themeConfig,
        borderRadius: {...brandingThemeConfig.borderRadius, ...themeConfig?.borderRadius},
        colors: {...brandingThemeConfig.colors, ...themeConfig?.colors},
        components: {...brandingThemeConfig.components, ...themeConfig?.components},
        images: {...brandingThemeConfig.images, ...themeConfig?.images},
        shadows: {...brandingThemeConfig.shadows, ...themeConfig?.shadows},
        spacing: {...brandingThemeConfig.spacing, ...themeConfig?.spacing},
      };
    });

    const resolvedTheme: Ref<Theme> = computed<Theme>(() =>
      createTheme(finalThemeConfig.value, colorScheme.value === 'dark'),
    );

    const direction: Ref<'ltr' | 'rtl'> = computed<'ltr' | 'rtl'>(
      () => ((finalThemeConfig.value as any)?.direction as 'ltr' | 'rtl') || 'ltr',
    );

    const toggleTheme = (): void => {
      colorScheme.value = colorScheme.value === 'light' ? 'dark' : 'light';
    };

    // Apply CSS variables to DOM
    const applyToDom = (theme: Theme): void => {
      if (typeof document === 'undefined') return;
      const root: HTMLElement = document.documentElement;
      // Use the pre-computed cssVariables map from createTheme() which contains
      // correctly-named CSS variables (e.g. --thunder-color-primary-main).
      Object.entries(theme.cssVariables).forEach(([key, value]: [key: string, value: string]): void => {
        root.style.setProperty(key, value);
      });
    };

    watch(resolvedTheme, (theme: Theme): void => applyToDom(theme), {immediate: true});

    // Apply direction to document
    watch(
      direction,
      (dir: 'ltr' | 'rtl'): void => {
        if (typeof document !== 'undefined') {
          document.documentElement.dir = dir;
        }
      },
      {immediate: true},
    );

    // Set up automatic theme detection listeners
    let classObserver: MutationObserver | null = null;
    let mediaQuery: MediaQueryList | null = null;

    const handleThemeChange = (isDark: boolean): void => {
      colorScheme.value = isDark ? 'dark' : 'light';
    };

    onMounted((): void => {
      if (props.mode === 'branding') return;

      if (props.mode === 'class') {
        const targetElement: HTMLElement = (props.detection as any).targetElement || document.documentElement;
        if (targetElement) {
          classObserver = createClassObserver(targetElement, handleThemeChange, props.detection);
        }
      } else if (props.mode === 'system') {
        if (!props.inheritFromBranding || !(brandingContext as any)?.activeTheme.value) {
          mediaQuery = createMediaQueryListener(handleThemeChange);
        }
      }
    });

    onBeforeUnmount((): void => {
      if (classObserver) classObserver.disconnect();
      if (mediaQuery?.removeEventListener) {
        mediaQuery.removeEventListener('change', handleThemeChange as any);
      }
    });

    const context: ThemeContextValue = {
      brandingError: brandingContext?.error ?? readonly(ref(null)),
      colorScheme: readonly(colorScheme),
      direction: readonly(direction) as Readonly<Ref<'ltr' | 'rtl'>>,
      inheritFromBranding: props.inheritFromBranding,
      isBrandingLoading: brandingContext?.isLoading ?? readonly(ref(false)),
      theme: shallowReadonly(resolvedTheme),
      toggleTheme,
    };

    provide(THEME_KEY, context);

    return () => h('div', {style: 'display:contents'}, slots['default']?.());
  },
});

export default ThemeProvider;
