// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box, Stack, ToggleButton, ToggleButtonGroup, Tooltip} from '@wso2/oxygen-ui';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ComposerLayout from './composer/ComposerLayout';
import ComposerOverview from './composer/ComposerOverview';
import ComposerPanel from './composer/ComposerPanel';
import SchemaComposer from './SchemaComposer';
import SchemaJsonEditor from './SchemaJsonEditor';
import type {AuthorizationDetailTypeForm, SchemaMode} from '../../hooks/useAuthorizationDetailTypeForm';
import type {SchemaField} from '../../models/authorization-detail-type';
import {MAX_SCHEMA_BYTES, schemaKeywordIssues} from '../../utils/authorization-detail-schema';

export interface FieldsSectionProps {
  form: AuthorizationDetailTypeForm;
  readOnly?: boolean;
  /**
   * Leaves out the fields that need a processor that is not set, for a place where the processor is
   * set later.
   */
  deferProcessorIssues?: boolean;
}

/** Returns the dotted path of the field with the given key, so an issue found in JSON can name it. */
function fieldPath(fields: SchemaField[], key: string, prefix = ''): string | undefined {
  for (const field of fields) {
    const path = prefix ? `${prefix}.${field.name}` : field.name;
    if (field.key === key) return path;
    const nested = fieldPath(field.children, key, path);
    if (nested) return nested;
  }
  return undefined;
}

/**
 * Edits the values a request of an authorization details type carries, their rules, and the consent the
 * user is asked for: on the consent screen itself with the composer, or as JSON with the consent
 * settings beside it.
 */
export default function FieldsSection({
  form,
  readOnly = false,
  deferProcessorIssues = false,
}: FieldsSectionProps): JSX.Element {
  const {t} = useTranslation();
  const {activeFields, jsonInvalid, schemaMode} = form;
  const fieldsKnown = activeFields !== null;
  const issues = deferProcessorIssues ? form.fieldIssues : [...form.fieldIssues, ...form.processorIssues];
  // A schema the composer cannot show is checked against the keywords the server allows, so what the
  // server would refuse is named before saving.
  const keywordIssues = useMemo(() => {
    if (schemaMode !== 'json' || jsonInvalid || fieldsKnown) return [];
    const parsed = JSON.parse(form.jsonText) as Record<string, unknown>;
    return schemaKeywordIssues(parsed);
  }, [schemaMode, jsonInvalid, fieldsKnown, form.jsonText]);

  // A save the server refused for the schema is reported here; one refused for the summary is reported
  // on the summary. As JSON, it names where in the schema it is, as the problems found in the JSON do.
  const serverErrorPath = schemaMode === 'json' ? form.saveErrorPath : null;
  const serverError =
    form.saveError && form.saveErrorTarget === 'schema' ? (
      <Alert severity="error" data-testid="schema-save-error">
        {serverErrorPath ? `${serverErrorPath}: ${form.saveError}` : form.saveError}
      </Alert>
    ) : null;
  const sizeError = form.schemaTooLarge ? (
    <Alert severity="error" data-testid="schema-too-large">
      {t(
        'authorizationDetailTypes:schema.errors.tooLarge',
        'The schema can be at most {{max}} KiB. Remove fields or shorten their settings.',
        {max: MAX_SCHEMA_BYTES / 1024},
      )}
    </Alert>
  ) : null;

  const modeControl = (
    <ToggleButtonGroup
      value={schemaMode}
      exclusive
      size="small"
      onChange={(_, next: SchemaMode | null) => form.setSchemaMode(next)}
      aria-label={t('authorizationDetailTypes:schema.mode', 'Schema editor')}
      sx={{'& .MuiToggleButton-root': {textTransform: 'none'}}}
    >
      <ToggleButton value="builder" disabled={schemaMode === 'json' && !fieldsKnown}>
        {schemaMode === 'json' && !fieldsKnown ? (
          <Tooltip
            describeChild
            title={
              jsonInvalid
                ? t(
                    'authorizationDetailTypes:schema.composerUnavailableInvalid',
                    'Fix the JSON to open it in the composer.',
                  )
                : t(
                    'authorizationDetailTypes:schema.composerUnavailable',
                    'The composer cannot show this schema. Edit it as JSON.',
                  )
            }
          >
            <Box component="span" sx={{pointerEvents: 'auto'}}>
              {t('authorizationDetailTypes:schema.modeComposer', 'Composer')}
            </Box>
          </Tooltip>
        ) : (
          t('authorizationDetailTypes:schema.modeComposer', 'Composer')
        )}
      </ToggleButton>
      <ToggleButton value="json">{t('authorizationDetailTypes:schema.modeJson', 'JSON')}</ToggleButton>
    </ToggleButtonGroup>
  );

  if (schemaMode === 'builder') {
    return (
      <Stack spacing={2}>
        {serverError}
        {sizeError}
        <SchemaComposer
          fields={form.fields}
          onChange={form.setFields}
          issues={issues}
          readOnly={readOnly}
          typeValue={form.typeValue}
          consent={form}
          actions={modeControl}
          revealCount={form.revealCount}
          revealHandled={form.revealHandled}
          onRevealHandled={form.setRevealHandled}
        />
      </Stack>
    );
  }

  return (
    <ComposerLayout
      main={
        <ComposerPanel
          title={t('authorizationDetailTypes:schema.jsonTitle', 'Schema')}
          aria-label={t('authorizationDetailTypes:schema.jsonTitle', 'Schema')}
          actions={modeControl}
        >
          <Stack spacing={2}>
            {serverError}
            {sizeError}
            {jsonInvalid ? (
              <Alert severity="error" role="alert">
                {form.jsonError
                  ? t(
                      'authorizationDetailTypes:schema.jsonSyntax',
                      'The JSON is not valid near line {{line}}, column {{column}}. Check for a missing comma, quote, or bracket.',
                      {line: form.jsonError.line, column: form.jsonError.column},
                    )
                  : t('authorizationDetailTypes:schema.jsonInvalid', 'Enter the schema as a JSON object.')}
              </Alert>
            ) : (
              !fieldsKnown && (
                <Alert severity="info">
                  {t(
                    'authorizationDetailTypes:schema.builderUnavailable',
                    'This schema uses features the composer cannot show, so it can be edited only as JSON. It is checked when you save.',
                  )}
                </Alert>
              )
            )}
            {(issues.length > 0 || keywordIssues.length > 0) && (
              <Alert severity="error" data-testid="schema-json-issues">
                <Stack spacing={0.5}>
                  {issues.map((issue) => {
                    const path = activeFields ? fieldPath(activeFields, issue.fieldKey) : undefined;
                    const message = t(`authorizationDetailTypes:${issue.messageKey}`, issue.defaultMessage);
                    return (
                      <span key={`${issue.fieldKey}-${issue.messageKey}`}>
                        {path ? `${path}: ${message}` : message}
                      </span>
                    );
                  })}
                  {keywordIssues.map((issue) => {
                    const message = t(`authorizationDetailTypes:${issue.messageKey}`, issue.defaultMessage, {
                      ...issue.values,
                      interpolation: {escapeValue: false},
                    });
                    return (
                      <span key={`${issue.path}-${issue.messageKey}-${JSON.stringify(issue.values ?? {})}`}>
                        {issue.path ? `${issue.path}: ${message}` : message}
                      </span>
                    );
                  })}
                </Stack>
              </Alert>
            )}
            <SchemaJsonEditor
              value={form.jsonText}
              onChange={form.setJsonText}
              invalid={jsonInvalid}
              readOnly={readOnly}
              ariaLabel={t('authorizationDetailTypes:schema.jsonTitle', 'Schema')}
            />
          </Stack>
        </ComposerPanel>
      }
      side={
        <ComposerOverview
          fields={activeFields}
          typeValue={form.typeValue}
          consent={form}
          readOnly={readOnly}
          editSummary
        />
      }
    />
  );
}
