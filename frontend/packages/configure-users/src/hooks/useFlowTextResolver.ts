// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {resolveFlowTemplateLiterals, useThunderID} from '@thunderid/react';
import {useCallback} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Resolves `{{ t(...) }}` and `{{ meta(...) }}` literals carried by flow-rendered components.
 *
 * Flow labels are authored server side, so the Console cannot ship default values for them.
 * They are resolved against the Console's own i18next instance, which already loads every
 * namespace from `GET /i18n/languages/{language}/translations/resolve` on start up. That keeps
 * these labels working regardless of whether the SDK's `GET /flow/meta` bootstrap call ran.
 *
 * Parsing stays with the SDK resolver so both literal forms, and any added later, behave the
 * same here as they do in the sign-in gate. Only the translation lookup differs: the SDK emits
 * dot-separated keys (`onboarding.forms.x`), while i18next is configured with
 * `keySeparator: false` and expects the namespace to be colon separated (`onboarding:forms.x`).
 */
export default function useFlowTextResolver(): (text?: string) => string | undefined {
  const {meta} = useThunderID();
  const {t} = useTranslation();

  return useCallback(
    (text?: string): string | undefined =>
      text
        ? resolveFlowTemplateLiterals(text, {
            meta,
            t: (key: string): string => t(key.replace('.', ':')),
          })
        : undefined,
    [meta, t],
  );
}
