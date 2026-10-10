// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Chip, Paper, Stack, Table, TableBody, TableCell, TableHead, TableRow, Typography} from '@wso2/oxygen-ui';
import {Fragment, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {isObject} from './paths';

type Fields = Record<string, unknown>;

const text = (value: unknown): string | undefined => (typeof value === 'string' ? value : undefined);

/** A display name as a schema writes one: text, or text by language. */
function displayNameOf(value: unknown): string | undefined {
  if (typeof value === 'string') return value;
  if (isObject(value)) {
    return text(value.en) ?? text(value['en-US']) ?? Object.values(value).map(text).find(Boolean);
  }
  return undefined;
}

function Rows({schema, depth}: {schema: Fields; depth: number}): JSX.Element {
  const {t} = useTranslation();
  return (
    <>
      {Object.entries(schema).map(([name, definition]: [string, unknown]) => {
        const property: Fields = isObject(definition) ? definition : {};
        const displayName = displayNameOf(property.displayName);
        const nested: Fields | undefined = isObject(property.properties)
          ? property.properties
          : isObject(property.items) && isObject(property.items.properties)
            ? property.items.properties
            : undefined;
        const values = Array.isArray(property.enum) ? (property.enum as unknown[]) : [];
        return (
          <Fragment key={name}>
            <TableRow>
              <TableCell sx={{pl: 2 + depth * 3}}>
                <Typography variant="body2" fontWeight={500}>
                  {displayName ?? name}
                </Typography>
                {displayName && (
                  <Typography variant="caption" color="text.secondary" sx={{fontFamily: 'monospace'}}>
                    {name}
                  </Typography>
                )}
              </TableCell>
              <TableCell>
                <Chip size="small" variant="outlined" label={String(property.type)} />
              </TableCell>
              <TableCell>
                <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                  {property.required === true && (
                    <Chip size="small" variant="outlined" label={t('common:environment.schema.required', 'Required')} />
                  )}
                  {property.unique === true && (
                    <Chip size="small" variant="outlined" label={t('common:environment.schema.unique', 'Unique')} />
                  )}
                  {property.credential === true && (
                    <Chip
                      size="small"
                      variant="outlined"
                      label={t('common:environment.schema.credential', 'Credential')}
                    />
                  )}
                </Stack>
              </TableCell>
              <TableCell>
                {values.length > 0 ? (
                  <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                    {values.map((value: unknown) => (
                      <Chip key={String(value)} size="small" label={String(value)} />
                    ))}
                  </Stack>
                ) : (
                  text(property.regex) && (
                    <Typography variant="body2" sx={{fontFamily: 'monospace'}}>
                      {text(property.regex)}
                    </Typography>
                  )
                )}
              </TableCell>
            </TableRow>
            {nested && <Rows schema={nested} depth={depth + 1} />}
          </Fragment>
        );
      })}
    </>
  );
}

/** A schema as the schema editor lays it out: a row to each property, nested ones indented. */
export default function SchemaTable({schema}: {schema: Fields}): JSX.Element {
  const {t} = useTranslation();
  return (
    <Paper variant="outlined" sx={{overflow: 'auto'}}>
      <Table size="small">
        <TableHead>
          <TableRow sx={{'& th': {bgcolor: 'action.hover'}}}>
            <TableCell>{t('common:environment.schema.property', 'Property')}</TableCell>
            <TableCell sx={{width: 120}}>{t('common:environment.schema.type', 'Type')}</TableCell>
            <TableCell sx={{width: 240}}>{t('common:environment.schema.constraints', 'Constraints')}</TableCell>
            <TableCell>{t('common:environment.schema.values', 'Values')}</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          <Rows schema={schema} depth={0} />
        </TableBody>
      </Table>
    </Paper>
  );
}
