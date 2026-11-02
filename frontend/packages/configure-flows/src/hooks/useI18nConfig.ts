// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import I18nContext, {type I18nContextProps} from '../context/I18nContext';

/**
 * Hook that provides access to i18n configuration (text, language, locales, branding).
 *
 * Use this hook when a component only needs i18n/branding-related state.
 * For other context domains, see useUIPanelState, useInteractionState, useFlowConfig.
 */
const useI18nConfig = (): I18nContextProps => {
  const context = useContext(I18nContext);

  if (context === undefined) {
    throw new Error('useI18nConfig must be used within a FlowBuilderCoreProvider');
  }

  return context;
};

export default useI18nConfig;
