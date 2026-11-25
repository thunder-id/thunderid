// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DragDropProvider, type DragEndEvent} from '@dnd-kit/react';
import {Alert, Box, Button, IconButton, Snackbar} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useMemo, useRef, useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import ComposerLayout from './composer/ComposerLayout';
import ComposerOverview from './composer/ComposerOverview';
import ComposerPanel from './composer/ComposerPanel';
import ConsentCanvas, {type CanvasSelection} from './composer/ConsentCanvas';
import {CANVAS_SUMMARY_ID, fieldRowId} from './composer/elementChrome';
import FieldInspector from './composer/FieldInspector';
import SummaryEditor from './composer/SummaryEditor';
import useComposerLabels from './composer/useComposerLabels';
import type {ConsentSettings} from '../../hooks/useAuthorizationDetailTypeForm';
import type {SchemaField} from '../../models/authorization-detail-type';
import {
  isConsentPreviewInvalid,
  missingStandardFields,
  pathsReferencing,
  previewConsentTemplate,
  renameTemplatePaths,
  type FieldIssue,
} from '../../utils/authorization-detail-schema';
import {
  applyDrop,
  applyMove,
  createPaletteField,
  duplicateField,
  findPosition,
  flattenFields,
  insertField,
  recordPaths,
  removeField,
  summaryRenames,
  updateField,
  type DragSource,
  type FieldMove,
  type FieldPosition,
  type PaletteItem,
  type PathHistory,
  type SummaryRename,
} from '../../utils/schema-composer';

export interface SchemaComposerProps {
  fields: SchemaField[];
  onChange: (fields: SchemaField[]) => void;
  issues: FieldIssue[];
  readOnly?: boolean;
  /** The type identifier, which the consent screen shows when there is no summary. */
  typeValue: string;
  /** The consent summary and whether an approval is reused. */
  consent: ConsentSettings;
  /** Controls for the header of the card's panel, such as the switch to JSON. */
  actions?: ReactNode;
  /**
   * Counts the times the form showed every problem. Each time it grows, the first field with a
   * problem is selected, or the summary when only the summary has one.
   */
  revealCount?: number;
  /**
   * The count the first problem was last selected for, kept by the form while the composer is not
   * shown, so a composer shown after the problems were shown still selects the first one.
   */
  revealHandled?: number;
  /** Called with the count once the first problem is selected for it. */
  onRevealHandled?: (count: number) => void;
}

/** A removed field, kept until the removal is undone or dismissed. */
interface Removal {
  before: SchemaField[];
  after: SchemaField[];
  /** The key of the removed field, which takes focus back when the removal is undone. */
  key: string;
  name: string;
  /** The number of fields inside the removed field. */
  nested: number;
  /** The placeholders of the summary that named the removed field. */
  referenced: string[];
}

/**
 * Composes an authorization details type on its consent screen: the card the user sees is rendered with
 * sample values and edited in place. Selecting the summary or a field on the card edits it in the side
 * panel; with nothing selected, the side panel holds the consent settings and sums up the type.
 */
export default function SchemaComposer({
  fields,
  onChange,
  issues,
  readOnly = false,
  typeValue,
  consent,
  actions = undefined,
  revealCount = 0,
  revealHandled = undefined,
  onRevealHandled = undefined,
}: SchemaComposerProps): JSX.Element {
  const {t} = useTranslation();
  const labels = useComposerLabels();
  const [selection, setSelection] = useState<CanvasSelection>(null);
  // A request to bring the settings of a field into view and move focus into them, made when the field
  // is added or selected on the card, and not when it is selected for a move or a problem.
  const [settingsFocus, setSettingsFocus] = useState<{key: string} | null>(null);
  // The paths each field had when it was shown, selected, or moved, so a rename or a move that leaves
  // the summary naming an old path can offer to update it.
  const [pathHistory, setPathHistory] = useState<PathHistory>(() => recordPaths({}, flattenFields(fields)));
  const [removal, setRemoval] = useState<Removal | null>(null);
  const [handledReveal, setHandledReveal] = useState(revealHandled ?? revealCount);
  // The field row to move focus to, as when it is selected for a problem, moved, or brought back; or
  // null for the undo of a removal that left no field.
  const [focusRequest, setFocusRequest] = useState<{key: string | null} | null>(null);
  const undoRef = useRef<HTMLButtonElement>(null);

  const flat = useMemo(() => flattenFields(fields), [fields]);
  const selectedKey = selection?.kind === 'field' ? selection.key : null;
  const selected = flat.find((row) => row.field.key === selectedKey);
  // Kept while the fields with issues stay the same, so the card's rows redraw only when they change.
  const issueSignature = issues.map((issue) => issue.fieldKey).join(' ');
  const issueKeys = useMemo(() => new Set(issueSignature.split(' ').filter(Boolean)), [issueSignature]);
  // As on the server, `{{type}}` names the type identifier.
  const canvasPreview = useMemo(
    () =>
      consent.preview && {...consent.preview, text: previewConsentTemplate(consent.template, fields, typeValue).text},
    [consent.preview, consent.template, fields, typeValue],
  );

  const rememberPaths = (): void => setPathHistory((history) => recordPaths(history, flat));

  const select = (next: CanvasSelection): void => {
    setSelection(next);
    setSettingsFocus(null);
    rememberPaths();
  };

  // Each time the form shows every problem, the first one is selected and focused.
  if (revealCount !== handledReveal) {
    setHandledReveal(revealCount);
    const first = issues[0]?.fieldKey;
    if (first) {
      select({kind: 'field', key: first});
      setFocusRequest({key: first});
    } else if (
      isConsentPreviewInvalid(consent.preview) ||
      (consent.saveError && consent.saveErrorTarget === 'template')
    ) {
      select({kind: 'summary'});
    }
  }

  // Selecting a field on the card opens its settings, which take focus.
  const open = (next: CanvasSelection): void => {
    select(next);
    if (next?.kind === 'field') setSettingsFocus({key: next.key});
  };

  useEffect(() => {
    onRevealHandled?.(handledReveal);
  }, [handledReveal, onRevealHandled]);

  useEffect(() => {
    if (!focusRequest) return;
    if (focusRequest.key === null) undoRef.current?.focus();
    else document.getElementById(fieldRowId(focusRequest.key))?.focus();
  }, [focusRequest]);

  // Closing the field settings returns focus to the field on the card.
  const closeInspector = (key: string): void => {
    select(null);
    document.getElementById(fieldRowId(key))?.focus();
  };

  // Closing the summary settings returns focus to the summary on the card.
  const closeSummary = (): void => {
    select(null);
    document.getElementById(CANVAS_SUMMARY_ID)?.focus();
  };

  const add = (item: PaletteItem, parentKey: string | null, index: number): void => {
    const field = createPaletteField(item);
    onChange(insertField(fields, field, parentKey, index));
    setSelection({kind: 'field', key: field.key});
    setSettingsFocus({key: field.key});
  };

  const handleDragEnd = (event: DragEndEvent): void => {
    const {source, target} = event.operation;
    if (event.canceled || !source || !target) return;
    const dragged = source.data as DragSource;
    const next = applyDrop(fields, dragged, target.data as FieldPosition);
    if (!next) return;
    onChange(next);
    select({kind: 'field', key: dragged.key});
  };

  // A move without dragging selects the field as a drop does, and keeps focus on it.
  const handleMove = (key: string, move: FieldMove): void => {
    onChange(applyMove(fields, key, move));
    select({kind: 'field', key});
    setFocusRequest({key});
  };

  const handleDuplicate = (key: string): void => {
    const result = duplicateField(fields, key);
    if (!result) return;
    onChange(result.fields);
    select({kind: 'field', key: result.key});
  };

  const handleRemove = (key: string): void => {
    const index = flat.findIndex((r) => r.field.key === key);
    const row = flat[index];
    const next = removeField(fields, key);
    onChange(next);
    // The selection goes with the field, and with any field inside it.
    if (selectedKey && !findPosition(next, selectedKey)) select(null);
    if (row) {
      const nested = flattenFields(row.field.children).length;
      setRemoval({
        before: fields,
        after: next,
        key,
        name: labels.heading(row.field),
        nested,
        referenced: pathsReferencing(consent.template, row.path),
      });
      // Focus moves to the field after the removed one, or the one before it, or else to the undo.
      setFocusRequest({key: (flat[index + nested + 1] ?? flat[index - 1])?.field.key ?? null});
    }
  };

  // The removal can be undone only while nothing else has changed the fields since.
  const undoable = removal?.after === fields;
  const undoRemoval = (): void => {
    if (removal?.after !== fields) return;
    onChange(removal.before);
    setRemoval(null);
    setFocusRequest({key: removal.key});
  };

  let removalMessage = '';
  if (removal) {
    const options = {name: removal.name, count: removal.nested, interpolation: {escapeValue: false}};
    removalMessage =
      removal.nested > 0
        ? t(
            'authorizationDetailTypes:composer.removed.withFields',
            'Removed {{name}} and the {{count}} fields in it.',
            {...options, defaultValue_one: 'Removed {{name}} and the field in it.'},
          )
        : t('authorizationDetailTypes:composer.removed.field', 'Removed {{name}}.', options);
    if (removal.referenced.length > 0) {
      removalMessage = `${removalMessage} ${t(
        'authorizationDetailTypes:composer.removed.summary',
        'The summary still uses {{paths}}. Undo, or edit the summary.',
        {paths: removal.referenced.map((path) => `{{${path}}}`).join(', '), interpolation: {escapeValue: false}},
      )}`;
    }
  }

  // A rename or a move that left the summary naming a field's old path offers to update the summary, on
  // the field and wherever the summary is edited, for as long as the summary names the old path.
  const renames = useMemo(
    () => (readOnly ? [] : summaryRenames(consent.preview?.unknownPaths ?? [], flat, pathHistory)),
    [readOnly, consent.preview, flat, pathHistory],
  );
  const renameNotices = (shown: SummaryRename[]): ReactNode =>
    shown.length > 0 ? (
      <Box sx={{display: 'flex', flexDirection: 'column', gap: 1}}>
        {shown.map((rename) => (
          <Alert
            key={`${rename.from}>${rename.to}`}
            severity="warning"
            data-testid="summary-rename-notice"
            action={
              <Button
                color="inherit"
                size="small"
                onClick={() => consent.setTemplate(renameTemplatePaths(consent.template, rename.from, rename.to))}
              >
                {t('authorizationDetailTypes:composer.rename.update', 'Update summary')}
              </Button>
            }
          >
            {t(
              'authorizationDetailTypes:composer.rename.notice',
              'The summary uses {{from}}, which no longer names a field. Update it to {{to}}?',
              {from: `{{${rename.from}}}`, to: `{{${rename.to}}}`, interpolation: {escapeValue: false}},
            )}
          </Alert>
        ))}
      </Box>
    ) : undefined;

  const standardFields = readOnly ? [] : missingStandardFields(fields);

  const canvas = (
    <ConsentCanvas
      fields={fields}
      typeValue={typeValue}
      consent={canvasPreview}
      issueKeys={issueKeys}
      readOnly={readOnly}
      selection={selection}
      onSelect={open}
      onAdd={add}
      onMove={handleMove}
      onDuplicate={handleDuplicate}
      onRemove={handleRemove}
      standardFields={standardFields}
      onAddStandard={() => onChange([...fields, ...standardFields])}
    />
  );

  const main = (
    <ComposerPanel
      title={t('authorizationDetailTypes:composer.canvas.title', 'Preview')}
      subtitle={t('authorizationDetailTypes:composer.canvas.subtitle', 'As the user sees it, with sample values')}
      aria-label={t('authorizationDetailTypes:composer.canvas.title', 'Preview')}
      actions={actions}
      flush
    >
      {readOnly ? canvas : <DragDropProvider onDragEnd={handleDragEnd}>{canvas}</DragDropProvider>}
    </ComposerPanel>
  );

  let side: JSX.Element;
  if (selected) {
    side = (
      <FieldInspector
        key={selected.field.key}
        field={selected.field}
        path={selected.path}
        inheritsProcessor={selected.processorSupplied && selected.field.policy.suppliedBy !== 'processor'}
        issues={issues.filter((issue) => issue.fieldKey === selected.field.key)}
        readOnly={readOnly}
        focusRequest={settingsFocus?.key === selected.field.key ? settingsFocus : null}
        notice={renameNotices(renames.filter((rename) => rename.keys.includes(selected.field.key)))}
        onChange={(next) => onChange(updateField(fields, next.key, () => next))}
        onClose={() => closeInspector(selected.field.key)}
      />
    );
  } else if (selection?.kind === 'summary') {
    side = (
      <Box
        onKeyDown={(e) => {
          // Escape closes the summary settings, as it closes the field settings.
          if (e.key !== 'Escape' || e.defaultPrevented) return;
          e.preventDefault();
          closeSummary();
        }}
      >
        <ComposerPanel
          title={t('authorizationDetailTypes:composer.summary.title', 'Summary')}
          subtitle={t('authorizationDetailTypes:composer.summary.subtitle', 'The heading of the consent card')}
          aria-label={t('authorizationDetailTypes:composer.summary.label', 'Consent summary')}
          actions={
            <IconButton
              size="small"
              aria-label={t('authorizationDetailTypes:composer.summary.close', 'Close summary settings')}
              onClick={closeSummary}
            >
              <X size={16} />
            </IconButton>
          }
        >
          <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
            {renameNotices(renames)}
            <SummaryEditor
              consent={consent}
              fields={fields}
              typeValue={typeValue}
              readOnly={readOnly}
              focusOnMount={!readOnly}
            />
          </Box>
        </ComposerPanel>
      </Box>
    );
  } else {
    side = (
      <ComposerOverview
        fields={fields}
        typeValue={typeValue}
        consent={consent}
        readOnly={readOnly}
        notice={renameNotices(renames)}
        onSelect={(key) => open({kind: 'field', key})}
        onAdd={(item) => add(item, null, fields.length)}
      />
    );
  }

  return (
    <>
      <ComposerLayout main={main} side={side} />
      <Snackbar
        open={undoable}
        autoHideDuration={removal?.referenced.length ? 10000 : 6000}
        onClose={(_event, reason) => {
          if (reason !== 'clickaway') setRemoval(null);
        }}
        // Above the page's save bar, which sits at the bottom.
        anchorOrigin={{vertical: 'top', horizontal: 'center'}}
      >
        <Alert
          severity={removal?.referenced.length ? 'warning' : 'info'}
          action={
            <>
              <Button ref={undoRef} color="inherit" size="small" onClick={undoRemoval}>
                {t('authorizationDetailTypes:composer.removed.undo', 'Undo')}
              </Button>
              <IconButton
                color="inherit"
                size="small"
                aria-label={t('common:actions.close', 'Close')}
                onClick={() => setRemoval(null)}
              >
                <X size={16} />
              </IconButton>
            </>
          }
          sx={{width: '100%', alignItems: 'center'}}
        >
          {removalMessage}
        </Alert>
      </Snackbar>
    </>
  );
}
