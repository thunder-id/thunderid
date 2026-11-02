// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// GatePreview
export {default as GatePreview} from './GatePreview/GatePreview';
export {default as PreviewThemeProvider} from './GatePreview/PreviewThemeProvider';

// NotificationTemplatePreview (shared email/SMS template preview)
export {
  default as NotificationTemplatePreview,
  type NotificationTemplatePreviewProps,
  type NotificationChannel,
} from './NotificationTemplatePreview/NotificationTemplatePreview';

// Components
export {default as LayoutPresetThumbnail, type LayoutPresetVariant} from './components/layouts/LayoutPresetThumbnail';
export {default as LayoutThumbnail} from './components/layouts/LayoutThumbnail';
export {default as ThemeThumbnail} from './components/themes/ThemeThumbnail';

// Contexts
export {default as LayoutBuilderProvider} from './contexts/LayoutBuilder/LayoutBuilderProvider';
export {default as useLayoutBuilder} from './contexts/LayoutBuilder/useLayoutBuilder';
export {default as LayoutBuilderContext} from './contexts/LayoutBuilder/LayoutBuilderContext';

export {default as ThemeBuilderProvider} from './contexts/ThemeBuilder/ThemeBuilderProvider';
export {default as useThemeBuilder} from './contexts/ThemeBuilder/useThemeBuilder';
export {default as ThemeBuilderContext} from './contexts/ThemeBuilder/ThemeBuilderContext';

// Constants
export {VIEWPORT_WIDTHS, VIEWPORT_HEIGHTS} from './components/viewportConstants';
export {default as ColorSchemeOptions} from './constants/ColorSchemeOptions';
export {default as DesignUIConstants} from './constants/design-ui-constants';

// Models
export * from './models/theme-builder';

// Pages
export {default as DesignPage} from './pages/DesignPage';
export {default as LayoutBuilderPage} from './pages/LayoutBuilderPage';
export {default as ThemeBuilderPage} from './pages/ThemeBuilderPage';
export {default as ThemeCreatePage} from './pages/ThemeCreatePage';

// Routes
export type {DesignRoutePaths} from './hooks/useDesignRoutes';
export {defaultDesignRoutePaths, default as useDesignRoutes} from './hooks/useDesignRoutes';
