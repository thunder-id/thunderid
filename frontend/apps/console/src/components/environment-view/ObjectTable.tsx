// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getInitials} from '@thunderid/components';
import {
  Avatar,
  Chip,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {fieldLabel} from './labels';
import {isObject} from './paths';
import ReadOnlyValue from './ReadOnlyValue';
import {isEmptyValue} from './values';

type Fields = Record<string, unknown>;

/** The most columns a table of entries shows; the rest of an entry is left to its own view. */
const MAX_COLUMNS = 6;

/** The fields an entry is called by, first to last. */
const NAMES: readonly string[] = ['display', 'displayName', 'name', 'handle'];
/** The fields that say what kind of thing an entry is, shown as tags. */
const KINDS: readonly string[] = ['type', 'kind', 'format', 'flowType', 'operator', 'valueType'];
/** The fields that are identifiers, shown as code and last. */
const IDS: readonly string[] = ['id', 'resourceServerId', 'resourceId', 'ouId'];

const isScalar = (value: unknown): boolean => value === null || ['string', 'number', 'boolean'].includes(typeof value);
const isTags = (value: unknown): boolean =>
  Array.isArray(value) && value.length > 0 && value.every((item: unknown) => isScalar(item));

/**
 * Like entries, such as a group's members or a role's permissions, as a table: a column for each
 * field the entries hold a plain value or a list of plain values in, as far as there is room, with
 * the name first, the kind as a tag and identifiers as code.
 */
export default function ObjectTable({rows}: {rows: Fields[]}): JSX.Element {
  const columns: string[] = [];
  rows.forEach((row: Fields) =>
    Object.entries(row).forEach(([key, value]: [string, unknown]) => {
      if (!columns.includes(key) && (isScalar(value) || isTags(value)) && !isEmptyValue(value)) columns.push(key);
    }),
  );
  const rank = (key: string): number => {
    if (NAMES.includes(key)) return 0;
    if (KINDS.includes(key)) return 1;
    if (IDS.includes(key)) return 3;
    return 2;
  };
  const shown = [...columns].sort((a: string, b: string) => rank(a) - rank(b)).slice(0, MAX_COLUMNS);
  const nameColumn = shown.find((key: string) => NAMES.includes(key));
  const named = (row: Fields): string => {
    const name = row[nameColumn ?? ''];
    return typeof name === 'string' ? name : typeof row.id === 'string' ? row.id : '';
  };

  const cell = (row: Fields, column: string): JSX.Element => {
    const value = row[column];
    if (column === nameColumn && (KINDS.some((kind: string) => kind in row) || 'id' in row)) {
      return (
        <Stack direction="row" spacing={1.5} alignItems="center">
          <Avatar sx={{width: 28, height: 28, fontSize: 12}}>{getInitials(named(row))}</Avatar>
          <Typography variant="body2" fontWeight={500}>
            {String(value)}
          </Typography>
        </Stack>
      );
    }
    if (KINDS.includes(column) && typeof value === 'string') {
      return <Chip size="small" variant="outlined" color="primary" label={value} />;
    }
    if (IDS.includes(column) && typeof value === 'string' && !isObject(value)) {
      return <ReadOnlyValue value={value} />;
    }
    return <ReadOnlyValue value={value} />;
  };

  return (
    <Paper variant="outlined" sx={{overflow: 'auto'}}>
      <Table size="small">
        <TableHead>
          <TableRow sx={{'& th': {bgcolor: 'action.hover'}}}>
            {shown.map((column: string) => (
              <TableCell key={column}>{fieldLabel(column)}</TableCell>
            ))}
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((row: Fields, index: number) => (
            // Entries have no identity of their own to key on.
            // eslint-disable-next-line react/no-array-index-key
            <TableRow key={index}>
              {shown.map((column: string) => (
                <TableCell key={column} sx={{verticalAlign: 'top'}}>
                  {cell(row, column)}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Paper>
  );
}
