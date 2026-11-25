// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AuthenticationMethods,
  getServiceAuthenticationState,
  outboundAuthenticationFromFormValues,
  outboundAuthenticationToFormValues,
  type ServiceAuthenticationErrors,
  type ServiceAuthenticationFormValues,
  type ServiceAuthenticationState,
} from '@thunderid/components';
import {generateHandle, getErrorMessage} from '@thunderid/utils';
import {useCallback, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import type {
  AuthorizationDetailType,
  AuthorizationDetailTypeRequest,
  SchemaField,
} from '../models/authorization-detail-type';
import {
  checkConsentTemplate,
  fieldsToSchema,
  isConsentPreviewInvalid,
  isSchemaTooLarge,
  isValidProcessorEndpoint,
  isValidTypeValue,
  previewConsentTemplate,
  schemaDefinitionText,
  schemaKeywordIssues,
  schemaPaths,
  schemaToFields,
  usesProcessor,
  validateFields,
  type ConsentPreview,
  type FieldIssue,
} from '../utils/authorization-detail-schema';
import {flattenFields} from '../utils/schema-composer';

/** How the schema is edited: with the field builder, or as JSON. */
export type SchemaMode = 'builder' | 'json';

/**
 * The parts of the form, each shown as a wizard step and as an edit tab. The fields part also holds
 * the consent the user is asked for, since the fields are edited on the consent screen they make up.
 */
export type AuthorizationDetailTypeFormSection = 'general' | 'fields' | 'processor';

/**
 * The state, validation, and request of the form that creates or edits an authorization details type.
 */
export interface AuthorizationDetailTypeForm {
  /** Whether the form edits an existing type, whose type identifier cannot change. */
  isEdit: boolean;

  name: string;
  setName: (name: string) => void;
  nameError: boolean;
  typeValue: string;
  setTypeValue: (typeValue: string) => void;
  typeError: boolean;
  description: string;
  setDescription: (description: string) => void;

  schemaMode: SchemaMode;
  setSchemaMode: (mode: SchemaMode | null) => void;
  fields: SchemaField[];
  setFields: (fields: SchemaField[]) => void;
  jsonText: string;
  setJsonText: (text: string) => void;
  jsonInvalid: boolean;
  /** The fields of the schema being edited, or null when the builder cannot show it. */
  activeFields: SchemaField[] | null;
  /** The problems to show on the fields. */
  fieldIssues: FieldIssue[];
  /** The fields that need the processor that is not set. */
  processorIssues: FieldIssue[];
  /** The JSON text's syntax error, with where it is, or null when the text parses or is not shown. */
  jsonError: JsonSyntaxError | null;
  /** Whether the schema is larger than the server stores. */
  schemaTooLarge: boolean;

  template: string;
  setTemplate: (template: string) => void;
  /** The consent summary rendered with sample values, or null when there is no summary or no fields. */
  preview: ConsentPreview | null;
  askEveryTime: boolean;
  setAskEveryTime: (askEveryTime: boolean) => void;

  processorEnabled: boolean;
  setProcessorEnabled: (enabled: boolean) => void;
  endpoint: string;
  setEndpoint: (endpoint: string) => void;
  endpointError: boolean;
  /** The processor authentication, as the shared service authentication form edits it. */
  authentication: Required<ServiceAuthenticationFormValues>;
  /** Sets one value of the processor authentication. Choosing another method clears the credentials. */
  setAuthenticationValue: (name: string, value: string) => void;
  /** Whether the user chose to replace the stored processor credential. */
  secretReplacing: boolean;
  setSecretReplacing: (replacing: boolean) => void;
  /** Which processor credentials are stored and which have to be entered. */
  authenticationState: ServiceAuthenticationState;
  /** The errors to show on the processor credentials, once problems are shown. */
  authenticationErrors: ServiceAuthenticationErrors;
  /** Whether fields are set or changed by a processor that is not set. */
  processorMissing: boolean;

  /**
   * Whether every problem is shown, including values not entered yet. A value is reported as wrong
   * once it is entered; a missing one only after the user tries to go on or save.
   */
  issuesRevealed: boolean;
  /** Shows every problem, as when the user tries to go on or save. */
  revealIssues: () => void;
  /** Counts the times every problem was shown, so the fields can select the first one each time. */
  revealCount: number;
  /**
   * The count of the last time the fields selected the first problem, kept while they are not shown,
   * so fields shown after a save from another tab still select it.
   */
  revealHandled: number;
  setRevealHandled: (count: number) => void;
  /** Whether the name is missing, once problems are shown. */
  nameMissing: boolean;
  /** Whether the type identifier is missing, once problems are shown. */
  typeMissing: boolean;
  /** Whether the processor endpoint is missing, once problems are shown. */
  endpointMissing: boolean;
  /** Whether a credential of the processor authentication is missing, once problems are shown. */
  authenticationMissing: boolean;

  /** Whether each part of the form can be saved as it is. */
  sectionValid: Record<AuthorizationDetailTypeFormSection, boolean>;
  isValid: boolean;
  /** Whether the form differs from the type it started from. */
  hasChanges: boolean;
  /**
   * Whether the fields, the consent summary, whether approvals are reused, or whether there is a processor
   * differ from the type the form started from, so saving gives the type a new version.
   */
  definitionChanged: boolean;

  buildRequest: () => AuthorizationDetailTypeRequest;

  /** The error of the last save, until the form changes. */
  saveError: string | null;
  /** The value the last save error is about, while the error is shown, or null when it names none. */
  saveErrorTarget: SaveErrorTarget | null;
  /** The dotted path of the field a schema save error is about, while the error is shown, or null. */
  saveErrorPath: string | null;
  /** Records the error of a failed save, and returns the value of the form it is shown beside, if any. */
  reportSaveError: (err: Error) => SaveErrorTarget | null;
  clearSaveError: () => void;
}

/** A value of the form that a save error can be about. */
export type SaveErrorTarget = 'type' | 'name' | 'schema' | 'template' | 'endpoint' | 'authentication';

/** The part of the form each value a save error can be about is edited in. */
export const SAVE_ERROR_SECTION: Record<SaveErrorTarget, AuthorizationDetailTypeFormSection> = {
  type: 'general',
  name: 'general',
  schema: 'fields',
  template: 'fields',
  endpoint: 'processor',
  authentication: 'processor',
};

/** Where JSON text stops parsing, from 1. */
export interface JsonSyntaxError {
  line: number;
  column: number;
}

/**
 * The consent settings of the form: the summary the user sees, with the error of the last save, which
 * the summary shows when it is about the summary.
 */
export type ConsentSettings = Pick<AuthorizationDetailTypeForm, 'template' | 'setTemplate' | 'preview'> &
  Partial<Pick<AuthorizationDetailTypeForm, 'saveError' | 'saveErrorTarget'>>;

/** Parses schema text, returning undefined unless it is a JSON object. */
function parseSchemaText(text: string): Record<string, unknown> | undefined {
  try {
    const parsed = JSON.parse(text) as unknown;
    return typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : undefined;
  } catch {
    return undefined;
  }
}

/**
 * Returns where JSON text stops parsing, or null when it parses or the parser does not say where.
 * Browsers report a position, or a line and a column.
 */
export function locateJsonError(text: string): JsonSyntaxError | null {
  try {
    JSON.parse(text);
    return null;
  } catch (err) {
    const message = err instanceof Error ? err.message : '';
    const lineColumn = /line (\d+) column (\d+)/.exec(message);
    if (lineColumn) return {line: Number(lineColumn[1]), column: Number(lineColumn[2])};
    const position = /position (\d+)/.exec(message);
    if (position) {
      const before = text.slice(0, Number(position[1])).split('\n');
      return {line: before.length, column: before[before.length - 1].length + 1};
    }
    // A text that ends too early names no position: the problem is at its end.
    if (/end of (JSON )?input|Unterminated/i.test(message)) {
      const lines = text.split('\n');
      return {line: lines.length, column: lines[lines.length - 1].length + 1};
    }
    return null;
  }
}

/** Formats a schema for the JSON editor. */
function schemaText(schema: Record<string, unknown>): string {
  return JSON.stringify(schema, null, 2);
}

/** The value each save error code is about. */
const ERROR_TARGETS: Record<string, SaveErrorTarget> = {
  'RAR-1003': 'type',
  'RAR-1004': 'type',
  'RAR-1005': 'name',
  'RAR-1006': 'schema',
  'RAR-1007': 'endpoint',
  'RAR-1008': 'authentication',
  'RAR-1009': 'authentication',
  'RAR-1010': 'type',
  'RAR-1011': 'template',
};

/** The processor credential field a save error about the processor authentication is shown on, by method. */
const AUTHENTICATION_ERROR_FIELDS: Partial<Record<string, keyof ServiceAuthenticationErrors>> = {
  [AuthenticationMethods.BEARER]: 'bearerToken',
  [AuthenticationMethods.BASIC]: 'basicPassword',
  [AuthenticationMethods.API_KEY]: 'httpHeaders',
};

interface SaveErrorData {
  code?: string;
  description?: {params?: {path?: unknown}};
}

/** Returns the error code of a failed save, or undefined when the server sent none. */
export function saveErrorCode(err: Error): string | undefined {
  return (err as {response?: {data?: SaveErrorData}}).response?.data?.code;
}

/**
 * Returns what to log about a failed request: its status and error code only, since the error also
 * holds the request, with its body and the administrator's authorization header.
 */
export function errorLogDetails(err: Error): {status?: number; code?: string} {
  return {status: (err as {response?: {status?: number}}).response?.status, code: saveErrorCode(err)};
}

/** The field problems that a schema written as JSON cannot keep, since its fields are keyed by name. */
const JSON_BLOCKING_ISSUES: Record<string, string> = {
  'fields.errors.nameDuplicate': 'Another field at this level has the same name. Rename it to edit the schema as JSON.',
  'fields.errors.nameRequired': 'Enter a field name to edit the schema as JSON.',
};

/**
 * Writes a schema with the keywords of each object in a fixed order, so keywords written in another
 * order compare the same. The properties keep their order, which is the order the consent screen shows.
 */
function canonicalSchema(value: unknown, isProperties = false): unknown {
  if (Array.isArray(value)) return value.map((item) => canonicalSchema(item));
  if (typeof value !== 'object' || value === null) return value;
  const entries = Object.entries(value as Record<string, unknown>);
  if (!isProperties) entries.sort(([a], [b]) => a.localeCompare(b));
  return Object.fromEntries(
    entries.map(([key, child]) => [key, canonicalSchema(child, !isProperties && key === 'properties')]),
  );
}

/** Returns the value of the form a failed save is about, or null when it names none. */
function saveErrorTargetOf(err: Error): SaveErrorTarget | null {
  const code = saveErrorCode(err);
  return code ? (ERROR_TARGETS[code] ?? null) : null;
}

/**
 * Returns the dotted path of the field a schema error is about, or null when the error names none. An
 * empty path is about the schema as a whole.
 */
function schemaErrorPath(err: Error): string | null {
  const data = (err as {response?: {data?: SaveErrorData}}).response?.data;
  if (data?.code !== 'RAR-1006') return null;
  const path = data.description?.params?.path;
  return typeof path === 'string' && path !== '' ? path : null;
}

/**
 * Holds the form of an authorization details type: its type value and name, its fields and their
 * rules, the consent summary, and the processor. The form starts from the type it is given, so mount
 * it again to edit another or to discard the changes.
 */
export default function useAuthorizationDetailTypeForm(
  detailType: AuthorizationDetailType | null = null,
): AuthorizationDetailTypeForm {
  const {t} = useTranslation();
  const isEdit = detailType !== null;

  // Resolves an error through the `authorizationDetailTypes` catalog. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with
  // `authorizationDetailTypes:`, per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `authorizationDetailTypes:${key}`, options),
    [t],
  );

  // A schema the builder cannot show opens as JSON, and is saved as it is unless edited there.
  const [initialFields] = useState(() => (detailType ? schemaToFields(detailType.schema) : []));
  const [schemaMode, setSchemaModeState] = useState<SchemaMode>(initialFields === null ? 'json' : 'builder');
  const [jsonText, setJsonText] = useState(() => schemaText(detailType?.schema ?? fieldsToSchema([])));
  const [typeValue, setTypeValueState] = useState(detailType?.type ?? '');
  // A new type's identifier follows its name until the identifier is edited.
  const [typeValueEdited, setTypeValueEdited] = useState(isEdit);
  const [name, setNameState] = useState(detailType?.name ?? '');
  const [description, setDescription] = useState(detailType?.description ?? '');
  const [fields, setFields] = useState<SchemaField[]>(initialFields ?? []);
  const [template, setTemplate] = useState(detailType?.consentTemplate ?? '');
  const [processorEnabled, setProcessorEnabled] = useState(Boolean(detailType?.processor));
  const [endpoint, setEndpoint] = useState(detailType?.processor?.endpoint ?? '');
  const [authentication, setAuthentication] = useState<Required<ServiceAuthenticationFormValues>>(() =>
    outboundAuthenticationToFormValues(detailType?.processor?.authentication),
  );
  const [secretReplacing, setSecretReplacing] = useState(false);
  // A type asks every time unless it is set to reuse approvals.
  const [askEveryTime, setAskEveryTime] = useState(detailType ? !detailType.reuse : true);
  const [lastSaveError, setLastSaveError] = useState<{
    message: string;
    target: SaveErrorTarget | null;
    path: string | null;
    form: string;
  } | null>(null);
  const [revealCount, setRevealCount] = useState(0);
  const [revealHandled, setRevealHandled] = useState(0);
  // The fields as they were when switching to JSON was refused, while they are unchanged.
  const [jsonRefusedFields, setJsonRefusedFields] = useState<SchemaField[] | null>(null);
  const issuesRevealed = revealCount > 0;

  // In JSON mode the schema is the parsed text, and its fields are known only when the builder can show
  // it; the server checks whatever the builder cannot.
  const jsonSchema = useMemo(
    () => (schemaMode === 'json' ? parseSchemaText(jsonText) : undefined),
    [schemaMode, jsonText],
  );
  const jsonInvalid = schemaMode === 'json' && jsonSchema === undefined;
  const jsonError = useMemo(() => (jsonInvalid ? locateJsonError(jsonText) : null), [jsonInvalid, jsonText]);
  const activeFields = useMemo(
    () => (schemaMode === 'builder' ? fields : jsonSchema ? schemaToFields(jsonSchema) : null),
    [schemaMode, fields, jsonSchema],
  );
  const schema = useMemo(
    () => (schemaMode === 'builder' ? fieldsToSchema(fields) : (jsonSchema ?? {})),
    [schemaMode, fields, jsonSchema],
  );
  const issues = useMemo(() => (activeFields ? validateFields(activeFields) : []), [activeFields]);
  const schemaTooLarge = useMemo(() => isSchemaTooLarge(schema), [schema]);
  // A schema the composer cannot show is checked against the keywords the server allows.
  const keywordIssueCount = useMemo(
    () => (schemaMode === 'json' && jsonSchema && !activeFields ? schemaKeywordIssues(jsonSchema).length : 0),
    [schemaMode, jsonSchema, activeFields],
  );
  // A schema the builder cannot show still declares the paths a summary can name.
  let preview: ConsentPreview | null = null;
  if (template.trim() && activeFields) preview = previewConsentTemplate(template, activeFields, typeValue.trim());
  else if (template.trim() && jsonSchema) preview = checkConsentTemplate(template, schemaPaths(jsonSchema));

  // A save error is stale once the form changes, so it is shown only while the form is as it was saved.
  const formSnapshot = JSON.stringify([
    typeValue,
    name,
    description,
    schema,
    template,
    processorEnabled,
    endpoint,
    authentication,
    secretReplacing,
    askEveryTime,
  ]);
  const currentSaveError = lastSaveError?.form === formSnapshot ? lastSaveError : null;
  // The form has changes only when it would save something else, so spaces around a value or keywords
  // in another order do not count.
  const changeSnapshot = JSON.stringify([
    typeValue.trim(),
    name.trim(),
    description.trim(),
    canonicalSchema(schema),
    template.trim(),
    processorEnabled,
    endpoint.trim(),
    authentication,
    secretReplacing,
    askEveryTime,
  ]);
  const [initialSnapshot] = useState(changeSnapshot);

  // The server gives a type a new version when its schema, its consent summary or whether it reuses
  // approvals changes, or when a processor is added or removed.
  // The schema is compared as the server compares it, so a change that means the same, such as fields in
  // another order, keeps the version.
  const definitionSnapshot = JSON.stringify([
    schemaDefinitionText(schema),
    template.trim(),
    processorEnabled,
    askEveryTime,
  ]);
  const [initialDefinitionSnapshot] = useState(definitionSnapshot);

  // A value is reported as wrong once it is entered; an empty required value only keeps saving disabled.
  const typeError = typeValue.trim() !== '' && !isValidTypeValue(typeValue.trim());
  const nameError = Array.from(name.trim()).length > 100;
  const endpointError = processorEnabled && endpoint.trim() !== '' && !isValidProcessorEndpoint(endpoint.trim());
  // Fields the processor sets or changes need a processor, as the server requires.
  const processorMissing = !processorEnabled && activeFields !== null && usesProcessor(activeFields);
  // A field without a name is reported only once problems are shown, so a new field is not flagged
  // before it is filled in.
  // A schema the server refused for one field is reported on that field too.
  const refusedPath = currentSaveError?.target === 'schema' ? currentSaveError.path : null;
  const policyErrorKey =
    refusedPath && activeFields
      ? flattenFields(activeFields).find((row) => row.path === refusedPath)?.field.key
      : undefined;
  const jsonRefused = schemaMode === 'builder' && jsonRefusedFields === fields;
  const fieldIssues = useMemo((): FieldIssue[] => {
    const revealed = issuesRevealed
      ? issues
      : issues.filter((issue) => issue.messageKey !== 'fields.errors.nameRequired');
    // After switching to JSON was refused, the problems that kept it say so.
    const shown = jsonRefused
      ? revealed.map((issue) =>
          JSON_BLOCKING_ISSUES[issue.messageKey]
            ? {
                ...issue,
                messageKey: issue.messageKey.replace('fields.errors.', 'fields.errors.json.'),
                defaultMessage: JSON_BLOCKING_ISSUES[issue.messageKey],
              }
            : issue,
        )
      : revealed;
    return policyErrorKey
      ? [
          {
            fieldKey: policyErrorKey,
            messageKey: 'fields.errors.policyRefused',
            defaultMessage: "The server refused this field's rules. Check its type, limits and who can change it.",
          },
          ...shown,
        ]
      : shown;
  }, [issues, issuesRevealed, jsonRefused, policyErrorKey]);
  // Point at the fields that need the missing processor, not only at the processor settings.
  const processorIssues = useMemo(
    (): FieldIssue[] =>
      processorMissing && activeFields
        ? activeFields
            .filter((field) => usesProcessor([field]))
            .map((field) => ({
              fieldKey: field.key,
              messageKey: 'fields.errors.processorMissing',
              defaultMessage: 'The external processor sets or changes this field, but no processor is set.',
            }))
        : [],
    [processorMissing, activeFields],
  );

  // A stored credential is kept only while the endpoint it was saved for is unchanged.
  const authenticationState = getServiceAuthenticationState(authentication, detailType?.processor?.authentication, {
    replacing: secretReplacing,
    targetChanged: isEdit && endpoint.trim() !== (detailType?.processor?.endpoint ?? '').trim(),
    requiredMessage: t('common:outboundAuthentication.required', 'This field is required.'),
  });
  const authenticationMissing =
    issuesRevealed && processorEnabled && !authenticationState.hasRequiredAuthenticationCredential;
  // A save the server refused for the processor authentication is shown on the credential it is about,
  // while that credential can be edited.
  const authenticationErrorField = AUTHENTICATION_ERROR_FIELDS[authenticationState.selectedAuthenticationScheme];
  const authenticationErrorShown =
    processorEnabled &&
    authenticationErrorField !== undefined &&
    (authenticationErrorField === 'httpHeaders' ||
      secretReplacing ||
      !authenticationState.hasStoredCredentialForSelectedScheme);
  const authenticationErrors: ServiceAuthenticationErrors = {
    ...(authenticationMissing ? authenticationState.authenticationErrors : {}),
    ...(currentSaveError?.target === 'authentication' && authenticationErrorShown
      ? {[authenticationErrorField]: currentSaveError.message}
      : {}),
  };

  const sectionValid: Record<AuthorizationDetailTypeFormSection, boolean> = {
    general: isValidTypeValue(typeValue.trim()) && name.trim() !== '' && !nameError,
    fields:
      !jsonInvalid &&
      !schemaTooLarge &&
      issues.length === 0 &&
      keywordIssueCount === 0 &&
      !isConsentPreviewInvalid(preview),
    processor:
      !processorMissing &&
      (!processorEnabled ||
        (isValidProcessorEndpoint(endpoint.trim()) && authenticationState.hasRequiredAuthenticationCredential)),
  };

  const setName = (next: string): void => {
    setNameState(next);
    // Accented letters keep their base letter, as in étranger to etranger.
    if (!typeValueEdited)
      setTypeValueState(
        generateHandle(
          next
            .replace(/ß/g, 'ss')
            .normalize('NFD')
            .replace(/[\u0300-\u036f]/g, ''),
          '_',
        ),
      );
  };

  const setTypeValue = (next: string): void => {
    setTypeValueState(next);
    setTypeValueEdited(true);
  };

  const setSchemaMode = (next: SchemaMode | null): void => {
    if (!next || next === schemaMode) return;
    if (next === 'json') {
      // JSON keys each field by its name, so a field without a name, or with the name of another, would
      // be lost. The switch waits until they are named, and the first is selected.
      if (issues.some((issue) => JSON_BLOCKING_ISSUES[issue.messageKey])) {
        setJsonRefusedFields(fields);
        setRevealCount((count) => count + 1);
        return;
      }
      setJsonText(schemaText(fieldsToSchema(fields)));
    } else if (activeFields) {
      setFields(activeFields);
    } else {
      return;
    }
    setSchemaModeState(next);
  };

  const setAuthenticationValue = (fieldName: string, value: string): void => {
    if (fieldName === 'authenticationScheme') {
      setSecretReplacing(false);
      setAuthentication({
        authenticationScheme: value,
        bearerToken: '',
        basicUsername: '',
        basicPassword: '',
        httpHeaders: '',
      });
      return;
    }
    setAuthentication((previous) => ({...previous, [fieldName]: value}));
  };

  const buildRequest = (): AuthorizationDetailTypeRequest => {
    const request: AuthorizationDetailTypeRequest = {
      type: typeValue.trim(),
      name: name.trim(),
      schema,
      reuse: !askEveryTime,
    };
    if (description.trim()) request.description = description.trim();
    if (template.trim()) request.consentTemplate = template.trim();
    if (processorEnabled) {
      // An unchanged authentication is left out, so the server keeps the stored credential.
      request.processor = {
        endpoint: endpoint.trim(),
        ...(authenticationState.authenticationChanged
          ? {authentication: outboundAuthenticationFromFormValues(authentication)}
          : {}),
      };
    }
    return request;
  };

  const reportSaveError = (err: Error): SaveErrorTarget | null => {
    let target = saveErrorTargetOf(err);
    // Without a credential field to show it on, an error about the processor authentication is shown
    // as one about the whole form.
    if (target === 'authentication' && !authenticationErrorShown) target = null;
    // An error about the schema or the summary selects what it is about, as saving with a problem does.
    if (target === 'schema' || target === 'template') setRevealCount((count) => count + 1);
    setLastSaveError({
      message: getErrorMessage(err, tForErrors, 'save.error', 'Failed to save the intent. Please try again.'),
      target,
      path: schemaErrorPath(err),
      form: formSnapshot,
    });
    return target;
  };

  return {
    isEdit,
    name,
    setName,
    nameError,
    typeValue,
    setTypeValue,
    typeError,
    description,
    setDescription,
    schemaMode,
    setSchemaMode,
    fields,
    setFields,
    jsonText,
    setJsonText,
    jsonInvalid,
    activeFields,
    fieldIssues,
    processorIssues,
    jsonError,
    schemaTooLarge,
    template,
    setTemplate,
    preview,
    askEveryTime,
    setAskEveryTime,
    processorEnabled,
    setProcessorEnabled,
    endpoint,
    setEndpoint,
    endpointError,
    authentication,
    setAuthenticationValue,
    secretReplacing,
    setSecretReplacing,
    authenticationState,
    authenticationErrors,
    processorMissing,
    issuesRevealed,
    revealIssues: () => setRevealCount((count) => count + 1),
    revealCount,
    revealHandled,
    setRevealHandled,
    // The name of a type being edited was there before, so clearing it is reported at once.
    nameMissing: (issuesRevealed || isEdit) && name.trim() === '',
    // A name with no letter the type identifier can take leaves it empty, which is reported at once.
    typeMissing: (issuesRevealed || name.trim() !== '') && typeValue.trim() === '',
    endpointMissing: issuesRevealed && processorEnabled && endpoint.trim() === '',
    authenticationMissing,
    sectionValid,
    isValid: Object.values(sectionValid).every(Boolean),
    hasChanges: changeSnapshot !== initialSnapshot,
    definitionChanged: definitionSnapshot !== initialDefinitionSnapshot,
    buildRequest,
    saveError: currentSaveError?.message ?? null,
    saveErrorTarget: currentSaveError?.target ?? null,
    saveErrorPath: currentSaveError?.path ?? null,
    reportSaveError,
    clearSaveError: () => setLastSaveError(null),
  };
}
