// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Chip, FormHelperText, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useEffect, useRef, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {ConsentSettings} from '../../../hooks/useAuthorizationDetailTypeForm';
import type {SchemaField} from '../../../models/authorization-detail-type';
import {
  hasUnsafeText,
  malformedPlaceholders,
  MAX_TEMPLATE_BYTES,
  optionalPaths,
  valuePaths,
} from '../../../utils/authorization-detail-schema';

export interface SummaryEditorProps {
  consent: Pick<ConsentSettings, 'template' | 'setTemplate' | 'preview' | 'saveError' | 'saveErrorTarget'>;
  /** The fields a placeholder can name, or null when they are not known. */
  fields: SchemaField[] | null;
  /** The type identifier, which the consent screen shows when there is no summary. */
  typeValue: string;
  readOnly: boolean;
  /** Moves focus to the summary, as when it is selected on the card. */
  focusOnMount?: boolean;
}

/**
 * Edits the consent summary, the heading of the consent card: text with a field's value written in
 * double braces. A field is inserted where the cursor is, or at the end, and a placeholder that names no
 * field, or text in double braces that is no placeholder, is reported, since the server refuses it.
 */
export default function SummaryEditor({
  consent,
  fields,
  typeValue,
  readOnly,
  focusOnMount = false,
}: SummaryEditorProps): JSX.Element {
  const {t} = useTranslation();
  const inputRef = useRef<HTMLTextAreaElement>(null);
  // Whether the cursor was placed in the summary: a summary starts with it at its start, so a field is
  // inserted at the end until it is.
  const caretPlaced = useRef(false);
  const {template, setTemplate, preview} = consent;
  // Two fields of the same name at one level share a path, which is offered once.
  const paths = fields && !readOnly ? [...new Set(valuePaths(fields))] : [];
  const unknownPaths = preview?.unknownPaths ?? [];
  const malformed = malformedPlaceholders(template);
  const unsafeText = hasUnsafeText(template);
  const optional = fields ? optionalPaths(template, fields) : [];
  const tooLong = Boolean(preview?.tooLong);
  // A save the server refused for the summary is reported here.
  const serverError = consent.saveErrorTarget === 'template' ? consent.saveError : null;
  const invalid = unknownPaths.length > 0 || malformed.length > 0 || unsafeText || tooLong || Boolean(serverError);

  useEffect(() => {
    if (!focusOnMount || !inputRef.current) return;
    const input = inputRef.current;
    input.focus();
    input.setSelectionRange(input.value.length, input.value.length);
  }, [focusOnMount]);

  const insert = (path: string): void => {
    const input = inputRef.current;
    const start = input && caretPlaced.current ? input.selectionStart : template.length;
    const end = input && caretPlaced.current ? input.selectionEnd : template.length;
    const before = template.slice(0, start);
    const after = template.slice(end);
    // Kept apart from the text around it by a space.
    const placeholder = `${before && !/\s$/.test(before) ? ' ' : ''}{{${path}}}${after && !/^\s/.test(after) ? ' ' : ''}`;
    setTemplate(`${before}${placeholder}${after}`);
    const caret = start + placeholder.length;
    caretPlaced.current = true;
    requestAnimationFrame(() => {
      input?.focus();
      input?.setSelectionRange(caret, caret);
    });
  };

  return (
    <Stack spacing={2}>
      <Stack>
        <TextField
          labelPlacement="top"
          label={t('authorizationDetailTypes:consent.template', 'Summary')}
          fullWidth
          id="authz-type-template"
          size="small"
          multiline
          minRows={2}
          inputRef={inputRef}
          value={template}
          disabled={readOnly}
          error={invalid}
          placeholder={t(
            'authorizationDetailTypes:consent.templatePlaceholder',
            'e.g., Pay {{amount}} {{currency}} to {{payee}}',
            {
              amount: '{{amount}}',
              currency: '{{currency}}',
              payee: '{{payee}}',
              interpolation: {escapeValue: false},
            },
          )}
          onChange={(e) => setTemplate(e.target.value)}
          onSelect={() => {
            caretPlaced.current = true;
          }}
          helperText={
            tooLong ? (
              t('authorizationDetailTypes:consent.tooLong', 'The summary can be at most {{max}} bytes.', {
                max: MAX_TEMPLATE_BYTES,
              })
            ) : unsafeText ? (
              <span data-testid="template-unsafe">
                {t(
                  'authorizationDetailTypes:consent.unsafeText',
                  'Remove the control or text direction characters from the summary.',
                )}
              </span>
            ) : malformed.length > 0 ? (
              <span data-testid="template-malformed">
                {t(
                  'authorizationDetailTypes:consent.malformed',
                  '{{text}} is not a field placeholder. Write a field name in double braces, with no spaces inside.',
                  {text: malformed.join(', '), interpolation: {escapeValue: false}},
                )}
              </span>
            ) : unknownPaths.length > 0 ? (
              t(
                'authorizationDetailTypes:consent.unknownFields',
                'No field matches {{paths}}. Use a field name from the list.',
                {paths: unknownPaths.join(', '), interpolation: {escapeValue: false}},
              )
            ) : serverError ? (
              <span data-testid="template-save-error">{serverError}</span>
            ) : (
              t(
                'authorizationDetailTypes:consent.templateHint',
                'Write a field in double braces, such as {{example}}, to show its requested value.',
                {example: '{{amount}}', interpolation: {escapeValue: false}},
              )
            )
          }
        />
        {optional.length > 0 && (
          <FormHelperText data-testid="template-optional">
            {t(
              'authorizationDetailTypes:consent.optionalFields',
              'A request can leave out {{paths}}. When it does, the consent screen shows the type identifier instead of this summary.',
              {paths: optional.map((path) => `{{${path}}}`).join(', '), interpolation: {escapeValue: false}},
            )}
          </FormHelperText>
        )}
      </Stack>
      {paths.length > 0 && (
        <Box>
          <Typography variant="caption" color="text.secondary" component="div" sx={{mb: 0.75}}>
            {t('authorizationDetailTypes:consent.insert', 'Insert a field:')}
          </Typography>
          <Box sx={{display: 'flex', gap: 0.75, flexWrap: 'wrap'}}>
            {paths.map((path) => (
              <Chip
                key={path}
                label={path}
                aria-label={t('authorizationDetailTypes:consent.insertField', 'Insert {{path}}', {path})}
                size="small"
                variant="outlined"
                sx={{fontFamily: 'monospace'}}
                onClick={() => insert(path)}
              />
            ))}
          </Box>
        </Box>
      )}
      {!template.trim() && (
        <Typography variant="caption" color="text.secondary">
          {t(
            'authorizationDetailTypes:consent.noSummary',
            'Without a summary, the consent screen shows the type identifier, {{type}}.',
            {type: typeValue.trim(), interpolation: {escapeValue: false}},
          )}
        </Typography>
      )}
    </Stack>
  );
}
