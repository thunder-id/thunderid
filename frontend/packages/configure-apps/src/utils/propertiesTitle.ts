// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {APP_PAGES, type AppPageDefinition, AppConfig} from '@thunderid/system-apps';
import type {AppBuilderSelection} from '../components/AppOutline';

/**
 * The heading of the properties panel for what is selected: a page, or the app's appearance.
 */
const propertiesTitle = (
  t: (key: string, defaultValue: string, options?: Record<string, unknown>) => string,
  config: AppConfig,
  selection: AppBuilderSelection,
): string => {
  const definition: AppPageDefinition | undefined = APP_PAGES[config.type].find(
    (candidate: AppPageDefinition) => candidate.key === selection,
  );

  return definition
    ? t('builder.properties.pageTitle', '{{page}} page', {page: t(`builder.pages.${definition.key}`, definition.key)})
    : t(`builder.appearance.sections.${selection}`, selection);
};

export default propertiesTitle;
