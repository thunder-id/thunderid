// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMemo} from 'react';
import {useTranslation} from 'react-i18next';
import type {
  FieldKind,
  ItemKind,
  Modification,
  ModifyingParty,
  SchemaField,
} from '../../../models/authorization-detail-type';
import {effectiveModification, modifyingParties} from '../../../utils/authorization-detail-schema';
import type {FlatField, PaletteItem} from '../../../utils/schema-composer';

/** A party that may change a field, as the composer offers it: a short name and what it can do. */
export interface PartyLabel {
  label: string;
  hint: string;
}

/** The labels the composer's panels share. */
export interface ComposerLabels {
  kind: Record<FieldKind, string>;
  palette: Record<PaletteItem, string>;
  itemKind: Record<ItemKind, string>;
  party: Record<ModifyingParty, PartyLabel>;
  /** How a field may change after the request, as an answer to whether it can change. */
  modification: (field: SchemaField, modification: Modification) => string;
  /** The heading of a field: its display name, its name, or a placeholder for a new field. */
  heading: (field: {name: string; title: string}) => string;
  /** Who supplies a field, as a short sentence. */
  supplier: (flat: Pick<FlatField, 'field' | 'processorSupplied'>) => string;
  /** How a field may change after the request and who may change it, as a short sentence. */
  change: (flat: Pick<FlatField, 'field' | 'processorSupplied'>) => string;
  /** Who supplies a field and who may change it, in plain words. */
  policySummary: (flat: Pick<FlatField, 'field' | 'processorSupplied'>) => string;
  /**
   * The accessible description of a field on the card, read after its heading: its kind, whether it
   * is required, its rule, and whether it needs attention.
   */
  rowDescription: (flat: Pick<FlatField, 'field' | 'processorSupplied'>, hasIssues: boolean) => string;
}

/** The parties in the order a reader thinks of them: the user first, the client last. */
export const READING_ORDER: ModifyingParty[] = ['user', 'processor', 'client'];

/** Whether a field holds a date, whose lower values are earlier ones. */
const isDate = (field: SchemaField): boolean => field.kind === 'date' || field.kind === 'dateTime';

/**
 * Returns the translated labels of field kinds, changes, and parties, made once for each language, since
 * every row of the card reads them.
 */
export default function useComposerLabels(): ComposerLabels {
  const {t, i18n} = useTranslation();
  return useMemo(() => composerLabels(t, i18n.language), [t, i18n.language]);
}

function composerLabels(t: ReturnType<typeof useTranslation>['t'], language: string): ComposerLabels {
  const kind: Record<FieldKind, string> = {
    string: t('authorizationDetailTypes:fields.kind.string', 'Text'),
    number: t('authorizationDetailTypes:fields.kind.number', 'Number'),
    date: t('authorizationDetailTypes:fields.kind.date', 'Date'),
    dateTime: t('authorizationDetailTypes:fields.kind.dateTime', 'Date and time'),
    boolean: t('authorizationDetailTypes:fields.kind.boolean', 'Boolean'),
    array: t('authorizationDetailTypes:fields.kind.array', 'List'),
    object: t('authorizationDetailTypes:fields.kind.object', 'Object'),
  };

  const party: Record<ModifyingParty, PartyLabel> = {
    user: {
      label: t('authorizationDetailTypes:composer.who.user', 'The user, on the consent screen'),
      hint: t('authorizationDetailTypes:composer.who.userHint', 'Can adjust it before approving.'),
    },
    processor: {
      label: t('authorizationDetailTypes:composer.who.processor', 'Your external processor'),
      hint: t('authorizationDetailTypes:composer.who.processorHint', 'Can adjust it before the user is asked.'),
    },
    client: {
      label: t('authorizationDetailTypes:composer.who.client', 'The client'),
      hint: t(
        'authorizationDetailTypes:composer.who.clientHint',
        'Can later ask for a token with less than the user approved.',
      ),
    },
  };

  // The parties as a sentence names them.
  const partyNames: Record<ModifyingParty, string> = {
    user: t('authorizationDetailTypes:composer.party.user', 'the user'),
    processor: t('authorizationDetailTypes:composer.party.processor', 'your external processor'),
    client: t('authorizationDetailTypes:composer.party.client', 'the client'),
  };

  const changeTexts: Record<string, {key: string; text: string}> = {
    decrease: {key: 'authorizationDetailTypes:composer.policy.decrease', text: '{{parties}} can lower it.'},
    increase: {key: 'authorizationDetailTypes:composer.policy.increase', text: '{{parties}} can raise it.'},
    decreaseDate: {
      key: 'authorizationDetailTypes:composer.policy.decreaseDate',
      text: '{{parties}} can move it earlier.',
    },
    increaseDate: {
      key: 'authorizationDetailTypes:composer.policy.increaseDate',
      text: '{{parties}} can move it later.',
    },
    removeItems: {
      key: 'authorizationDetailTypes:composer.policy.removeItems',
      text: '{{parties}} can remove items from it.',
    },
    addItems: {key: 'authorizationDetailTypes:composer.policy.addItems', text: '{{parties}} can add items to it.'},
  };

  const answers: Record<string, string> = {
    none: t('authorizationDetailTypes:composer.change.none', 'No, it stays as requested'),
    decrease: t('authorizationDetailTypes:composer.change.decrease', 'Yes, it can only go down'),
    increase: t('authorizationDetailTypes:composer.change.increase', 'Yes, it can only go up'),
    decreaseDate: t('authorizationDetailTypes:composer.change.decreaseDate', 'Yes, it can only move earlier'),
    increaseDate: t('authorizationDetailTypes:composer.change.increaseDate', 'Yes, it can only move later'),
    removeItems: t('authorizationDetailTypes:composer.change.removeItems', 'Yes, items can only be removed'),
    addItems: t('authorizationDetailTypes:composer.change.addItems', 'Yes, items can only be added'),
  };

  // A date moves earlier or later rather than going down or up.
  const variant = (field: SchemaField, modification: Modification): string =>
    isDate(field) && (modification === 'decrease' || modification === 'increase')
      ? `${modification}Date`
      : modification;

  const newFieldLabel = t('authorizationDetailTypes:fields.new', 'New field');

  const supplier: ComposerLabels['supplier'] = ({processorSupplied}) =>
    processorSupplied
      ? t(
          'authorizationDetailTypes:composer.policy.processor',
          'Your external processor fills this in after the request.',
        )
      : t('authorizationDetailTypes:composer.policy.client', 'The client sets this.');

  const change: ComposerLabels['change'] = ({field, processorSupplied}) => {
    const modification = effectiveModification(field);
    if (processorSupplied || modification === 'none') {
      return t('authorizationDetailTypes:composer.policy.fixed', 'Nobody can change it after that.');
    }
    const chosen = modifyingParties(field);
    if (chosen.length === 0) {
      return t('authorizationDetailTypes:composer.policy.noParties', 'Choose who can change it.');
    }
    const names = READING_ORDER.filter((p) => chosen.includes(p)).map((p) => partyNames[p]);
    const text = changeTexts[variant(field, modification)];
    const sentence = t(text.key, text.text, {
      parties: new Intl.ListFormat(language, {type: 'conjunction'}).format(names),
      interpolation: {escapeValue: false},
    });
    return sentence.charAt(0).toLocaleUpperCase(language) + sentence.slice(1);
  };

  return {
    kind,
    palette: {...kind, money: t('authorizationDetailTypes:composer.add.money', 'Money')},
    itemKind: {
      string: t('authorizationDetailTypes:fields.itemKind.string', 'Text'),
      number: t('authorizationDetailTypes:fields.itemKind.number', 'Number'),
    },
    party,
    modification: (field, modification) => answers[variant(field, modification)],
    heading: (field) => field.title.trim() || field.name.trim() || newFieldLabel,
    supplier,
    change,
    policySummary: (flat) => `${supplier(flat)} ${change(flat)}`,
    rowDescription: (flat, hasIssues) =>
      [
        `${kind[flat.field.kind]}${flat.field.required ? `, ${t('common:form.required', 'Required')}` : ''}.`,
        `${supplier(flat)} ${change(flat)}`,
        hasIssues ? `${t('authorizationDetailTypes:composer.canvas.hasIssues', 'This field needs attention')}.` : '',
      ]
        .filter(Boolean)
        .join(' '),
  };
}
