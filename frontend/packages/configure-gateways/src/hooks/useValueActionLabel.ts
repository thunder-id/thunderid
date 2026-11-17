// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTranslation} from 'react-i18next';
import type {ValueRow} from '../components/ValuesTable';
import type {ValueKind} from '../models/deploy';
import {isHeld} from '../utils/valueRows';

/** What setting a row's value is called: the word its button shows, and the same said with its name. */
export interface ValueActionLabel {
  /** The short text a button shows, such as "Edit". */
  text: string;
  /** The full label, the text with the value's name, as the button's accessible name and the dialog's title. */
  label: string;
}

/**
 * What setting a row's value does, as its button reads and its dialog is titled: a variable held is
 * edited, a secret held is replaced, and a value not held is set. The short text and the full label are
 * chosen together, so the two cannot drift apart.
 */
export default function useValueActionLabel(row: ValueRow, kind: ValueKind): ValueActionLabel {
  const {t} = useTranslation('deploy');
  const secret = kind === 'secret';
  if (!isHeld(row, secret)) {
    return {text: t('values.set', 'Set'), label: t('values.setLabel', 'Set {{name}}', {name: row.name})};
  }
  return secret
    ? {text: t('values.replace', 'Replace'), label: t('values.replaceLabel', 'Replace {{name}}', {name: row.name})}
    : {text: t('values.edit', 'Edit'), label: t('values.editLabel', 'Edit {{name}}', {name: row.name})};
}
