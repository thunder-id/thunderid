// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Box, Stack, Tab, Tabs, Typography} from '@wso2/oxygen-ui';
import {useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {useEnvironmentView} from './EnvironmentViewContext';
import FieldList, {FieldRows, type FieldRow} from './FieldList';
import {fieldLabel} from './labels';
import {
  ALWAYS_HIDDEN,
  nameOf,
  type FieldSpec,
  type Label,
  type ResourceLayout,
  type SectionSpec,
  type TabSpec,
} from './layouts';
import ObjectTable from './ObjectTable';
import {isObject, isSchema, rootOf, valueAt} from './paths';
import ReadOnlyValue from './ReadOnlyValue';
import SchemaTable from './SchemaTable';
import {isEmptyValue} from './values';

type Fields = Record<string, unknown>;

/** Entries of a list this short read better one by one than as a table. */
const MAX_ENTRIES_SHOWN_APART = 3;

/** Fields the page's header shows, so no tab repeats them. */
const HEADER_FIELDS: readonly string[] = ['name', 'displayName', 'description', 'logoUrl', 'language'];

/** One tab of a resource's view. */
interface SectionTab {
  key: string;
  title: string;
  content: ReactNode;
}

/** The entries of a part, such as a group's members, wherever its read keeps them. */
function entriesOf(part: unknown): unknown[] | undefined {
  if (Array.isArray(part)) return part as unknown[];
  if (isObject(part)) return Object.values(part).find((value: unknown): value is unknown[] => Array.isArray(value));
  return undefined;
}

/** A list of entries: a table when there are many, one under another when there are a few. */
function Entries({entries}: {entries: unknown[]}): JSX.Element {
  if (!entries.every(isObject)) {
    return <ReadOnlyValue value={entries} />;
  }
  if (entries.length > MAX_ENTRIES_SHOWN_APART) {
    return <ObjectTable rows={entries} />;
  }
  return (
    <Stack spacing={2}>
      {entries.map((entry: Fields, index: number) => (
        // Entries have no identity of their own to key on.
        // eslint-disable-next-line react/no-array-index-key
        <Stack key={index} spacing={1}>
          {entries.length > 1 && <Typography variant="subtitle2">{nameOf(entry, String(index + 1))}</Typography>}
          <FieldList fields={entry} />
        </Stack>
      ))}
    </Stack>
  );
}

/** The runtime endpoints a developer copies, below the environment's base URL. */
const ENDPOINTS: readonly [key: string, label: string, path: string][] = [
  ['openIdConfiguration', 'OpenID configuration', '/.well-known/openid-configuration'],
  ['authorization', 'Authorization', '/oauth2/authorize'],
  ['token', 'Token', '/oauth2/token'],
  ['userInfo', 'Userinfo', '/oauth2/userinfo'],
  ['jwks', 'JWKS', '/oauth2/jwks'],
];

/** The environment's own endpoints, as an application's Overview lists them for the configuration. */
function EndpointsCard(): JSX.Element | null {
  const {t} = useTranslation();
  const {environment} = useEnvironmentView();
  if (!environment.baseUrl) return null;
  const base = environment.baseUrl.replace(/\/+$/, '');
  return (
    <SettingsCard
      title={t('common:environment.endpoints.title', 'Useful Endpoints')}
      description={t('common:environment.endpoints.description', 'Where {{environment}} answers.', {
        environment: environment.name,
      })}
    >
      <FieldRows
        copy
        rows={ENDPOINTS.map(([key, label, path]: [string, string, string]) => ({
          key,
          label: t(`common:environment.endpoints.${key}`, label),
          value: `${base}${path}`,
        }))}
      />
    </SettingsCard>
  );
}

/** What a field holds, shown as the whole of its card when it is the card's one field and has fields of its own. */
function Value({value}: {value: unknown}): JSX.Element {
  if (Array.isArray(value)) return <Entries entries={value as unknown[]} />;
  if (isObject(value)) return isSchema(value) ? <SchemaTable schema={value} /> : <FieldList fields={value} />;
  return <ReadOnlyValue value={value} />;
}

export interface ResourceSectionsProps {
  resource: unknown;
  parts?: Record<string, unknown>;
  layout?: ResourceLayout;
}

/**
 * Lays a resource out for reading in the tabs its page in the configuration has, each with the
 * cards that page groups its fields in. A tab or card with nothing to show is left out, and the
 * fields no tab names are shown under "Other" at the end, so nothing the environment runs is hidden.
 * A resource whose page has no tabs shows its plain fields under "General" and a tab for each field
 * that holds fields of its own. One tab is shown without the tab bar.
 */
export default function ResourceSections({
  resource,
  parts = undefined,
  layout = undefined,
}: ResourceSectionsProps): JSX.Element {
  const {t} = useTranslation();
  // The tab chosen, by its key; untouched, the first.
  const [selected, setSelected] = useState<string | undefined>(undefined);
  const fields: Fields = isObject(resource) ? resource : {};
  const label = ([key, defaultValue]: Label): string => (key ? t(key, defaultValue) : defaultValue);
  const specs: TabSpec[] | undefined = typeof layout?.tabs === 'function' ? layout.tabs(fields) : layout?.tabs;

  const rowsOf = (spec: SectionSpec): FieldRow[] =>
    spec.fields
      .map((field: FieldSpec) => {
        const [path, title] = typeof field === 'string' ? [field, undefined] : field;
        return {
          key: path,
          label: title
            ? t(`common:environment.fields.${title.replace(/[^A-Za-z0-9]+/g, '')}`, title)
            : fieldLabel(path.split('.').pop() ?? path),
          value: valueAt(fields, path),
        };
      })
      .filter((row: FieldRow) => !isEmptyValue(row.value));

  // A card the configuration's page leaves untitled takes the name of its tab.
  const cardOf = (spec: SectionSpec, index: number, tabTitle: string): ReactNode => {
    const rows = rowsOf(spec);
    if (rows.length === 0) return null;
    const whole = rows.length === 1 && (isObject(rows[0].value) || Array.isArray(rows[0].value));
    return (
      <SettingsCard
        key={`${spec.title?.[1] ?? 'card'}-${String(index)}`}
        title={spec.title ? label(spec.title) : tabTitle}
      >
        {whole ? <Value value={rows[0].value} /> : <FieldRows rows={rows} copy={spec.copy} />}
      </SettingsCard>
    );
  };

  const partOf = (part: string, many: boolean, tabTitle: string): ReactNode => {
    const entries = entriesOf(parts?.[part]);
    if (!entries || entries.length === 0) return null;
    return (
      <SettingsCard key={`part:${part}`} title={many ? fieldLabel(part) : tabTitle}>
        <Entries entries={entries} />
      </SettingsCard>
    );
  };

  const tabs: SectionTab[] = [];
  let rest: string[];
  if (specs) {
    const named = new Set<string>(
      specs.flatMap((spec: TabSpec) =>
        (spec.sections ?? []).flatMap((card: SectionSpec) =>
          card.fields.map((field: FieldSpec) => rootOf(typeof field === 'string' ? field : field[0])),
        ),
      ),
    );
    specs.forEach((spec: TabSpec) => {
      const title = label(spec.title);
      const cards = [
        ...(spec.sections ?? []).map((card: SectionSpec, index: number) => cardOf(card, index, title)),
        ...(spec.parts ?? []).map((part: string) => partOf(part, (spec.parts?.length ?? 0) > 1, title)),

        ...(spec.endpoints ? [<EndpointsCard key="endpoints" />] : []),
      ].filter(Boolean);
      if (cards.length > 0) {
        tabs.push({key: spec.key, title, content: <Stack spacing={3}>{cards}</Stack>});
      }
    });
    rest = Object.keys(fields).filter((key: string) => !named.has(key));
  } else {
    const shown = Object.keys(fields).filter(
      (key: string) => !ALWAYS_HIDDEN.includes(key) && !HEADER_FIELDS.includes(key),
    );
    const grouped = shown.filter((key: string) => isObject(fields[key]) || Array.isArray(fields[key]));
    const plain = shown.filter((key: string) => !grouped.includes(key) && !isEmptyValue(fields[key]));
    if (plain.length > 0) {
      tabs.push({
        key: 'general',
        title: t('common:environment.tabs.general', 'General'),
        content: (
          <SettingsCard title={t('common:environment.tabs.general', 'General')}>
            <FieldList fields={fields} order={plain} />
          </SettingsCard>
        ),
      });
    }
    grouped
      .filter((key: string) => !isEmptyValue(fields[key]))
      .forEach((key: string) =>
        tabs.push({
          key,
          title: fieldLabel(key),
          content: (
            <SettingsCard title={fieldLabel(key)}>
              <Value value={fields[key]} />
            </SettingsCard>
          ),
        }),
      );
    rest = [];
  }
  rest = rest.filter(
    (key: string) => !ALWAYS_HIDDEN.includes(key) && !HEADER_FIELDS.includes(key) && !isEmptyValue(fields[key]),
  );
  if (rest.length > 0) {
    tabs.push({
      key: 'other',
      title: t('common:environment.tabs.other', 'Other'),
      content: (
        <SettingsCard title={t('common:environment.tabs.other', 'Other')}>
          <FieldList fields={fields} order={rest} />
        </SettingsCard>
      ),
    });
  }

  const shown: SectionTab | undefined = tabs.find((tab: SectionTab) => tab.key === selected) ?? tabs[0];
  if (!shown) {
    return <Box />;
  }
  return (
    <Box>
      {tabs.length > 1 && (
        <Tabs
          value={shown.key}
          onChange={(_event, value: string) => setSelected(value)}
          variant="scrollable"
          scrollButtons="auto"
          aria-label={t('common:environment.sections.label', 'Sections')}
        >
          {tabs.map((tab: SectionTab) => (
            <Tab
              key={tab.key}
              value={tab.key}
              label={tab.title}
              id={`environment-tab-${tab.key}`}
              aria-controls={`environment-tabpanel-${tab.key}`}
              sx={{textTransform: 'none'}}
            />
          ))}
        </Tabs>
      )}
      <Box role="tabpanel" id={`environment-tabpanel-${shown.key}`} aria-label={shown.title} sx={{mt: 3}}>
        {shown.content}
      </Box>
    </Box>
  );
}
