// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AuthorizationDetailSummary} from '@thunderid/design';
import {Box, Divider, FormControlLabel, Switch, Typography} from '@wso2/oxygen-ui';
import {memo, useEffect, useMemo, useRef, useState, type FocusEvent, type JSX, type KeyboardEvent} from 'react';
import {useTranslation} from 'react-i18next';
import AddFieldMenu from './AddFieldMenu';
import {CanvasGap, CanvasGroup, CanvasRow, type CanvasActions} from './CanvasField';
import elementChrome, {CANVAS_CONTAINER, CANVAS_SUMMARY_ID} from './elementChrome';
import useComposerLabels from './useComposerLabels';
import type {SchemaField} from '../../../models/authorization-detail-type';
import {isConsentPreviewInvalid, type ConsentPreview} from '../../../utils/authorization-detail-schema';
import {
  findField,
  moveAbilities,
  type FieldMove,
  type MoveAbility,
  type PaletteItem,
} from '../../../utils/schema-composer';

/** What is selected on the card: a field, the summary, or nothing, which stands for the card itself. */
export type CanvasSelection = {kind: 'field'; key: string} | {kind: 'summary'} | null;

export interface ConsentCanvasProps {
  fields: SchemaField[];
  /** The type identifier, which the consent screen shows when there is no summary. */
  typeValue: string;
  /** The consent summary rendered with sample values, or null when there is no summary. */
  consent: ConsentPreview | null;
  /** The keys of the fields that have issues. */
  issueKeys: ReadonlySet<string>;
  readOnly: boolean;
  selection: CanvasSelection;
  onSelect: (selection: CanvasSelection) => void;
  /** Adds a field of a kind into the object with the given key, or at the top level for null, at the index. */
  onAdd: (item: PaletteItem, parentKey: string | null, index: number) => void;
  onMove: (key: string, move: FieldMove) => void;
  onDuplicate: (key: string) => void;
  onRemove: (key: string) => void;
  /** The RFC 9396 common fields the type does not declare yet. */
  standardFields: SchemaField[];
  onAddStandard: () => void;
}

/** What the fields of the card share: the selection, the Tab stop, the issues, and the actions. */
interface CanvasShared {
  selectedKey: string | null;
  tabKey: string | null;
  issueKeys: ReadonlySet<string>;
  readOnly: boolean;
  abilities: ReadonlyMap<string, MoveAbility>;
  actions: CanvasActions;
}

/** The moves of a field that has none, as before it is known. */
const NO_MOVES: MoveAbility = {up: false, down: false, indent: false, outdent: false};

interface CanvasLevelProps {
  list: SchemaField[];
  /** The object the fields belong to, or null at the top level. */
  parent: SchemaField | null;
  depth: number;
  /** The dotted path of the object the fields belong to. */
  prefix: string;
  /** Whether the processor supplies the object the fields belong to. */
  inherited: boolean;
  shared: CanvasShared;
}

/** The fields of one level of the card, each with the plus before it, and the plus at the end. */
function CanvasLevel({list, parent, depth, prefix, inherited, shared}: CanvasLevelProps): JSX.Element {
  const {t} = useTranslation();
  const labels = useComposerLabels();
  const parentKey = parent?.key ?? null;
  return (
    <>
      {list.map((field, index) => (
        <CanvasItem
          key={field.key}
          field={field}
          index={index}
          parentKey={parentKey}
          depth={depth}
          prefix={prefix}
          inherited={inherited}
          shared={shared}
        />
      ))}
      <CanvasGap
        parentKey={parentKey}
        index={list.length}
        indent={parent ? depth : 0}
        readOnly={shared.readOnly}
        // The plus at the end of the last field goes with the field in the Tab order.
        tabStop={list[list.length - 1]?.key === shared.tabKey}
        label={
          parent
            ? t('authorizationDetailTypes:composer.add.toObject', 'Add field to {{name}}', {
                name: labels.heading(parent),
                interpolation: {escapeValue: false},
              })
            : t('authorizationDetailTypes:composer.canvas.insertEnd', 'Insert a field at the end')
        }
        onAdd={shared.actions.add}
      />
    </>
  );
}

interface CanvasItemProps extends Omit<CanvasLevelProps, 'list' | 'parent'> {
  field: SchemaField;
  index: number;
  parentKey: string | null;
}

/**
 * One field of the card with the plus before it: a row, or an object with its fields. It redraws only
 * when its field or what the fields share changes, so an edit redraws the edited field alone.
 */
const CanvasItem = memo(function CanvasItem({
  field,
  index,
  parentKey,
  depth,
  prefix,
  inherited,
  shared,
}: CanvasItemProps): JSX.Element {
  const {t} = useTranslation();
  const labels = useComposerLabels();
  const {selectedKey, tabKey, issueKeys, readOnly, abilities, actions} = shared;
  const name = field.name.trim();
  const path = prefix ? `${prefix}.${name}` : name;
  const processorSupplied = inherited || field.policy.suppliedBy === 'processor';
  const common = {
    field,
    level: depth + 1,
    processorSupplied,
    selected: selectedKey === field.key,
    hasIssues: issueKeys.has(field.key),
    readOnly,
    tabStop: field.key === tabKey,
    canMove: abilities.get(field.key) ?? NO_MOVES,
    actions,
  };
  return (
    <Box>
      <CanvasGap
        parentKey={parentKey}
        index={index}
        readOnly={readOnly}
        // The plus before a field goes with the field in the Tab order.
        tabStop={field.key === tabKey}
        label={t('authorizationDetailTypes:composer.canvas.insertBefore', 'Insert a field before {{name}}', {
          name: labels.heading(field),
          interpolation: {escapeValue: false},
        })}
        onAdd={actions.add}
      />
      {field.kind === 'object' ? (
        <CanvasGroup
          {...common}
          depth={depth}
          containsSelection={selectedKey !== null && findField(field.children, selectedKey) !== undefined}
        >
          {field.children.length === 0 &&
            (readOnly ? (
              <Typography variant="caption" color="text.secondary" component="div" sx={{px: 1}}>
                {t('authorizationDetailTypes:composer.canvas.objectEmpty', 'No fields in this object.')}
              </Typography>
            ) : (
              <Box sx={{px: 1, pt: 0.5}}>
                <AddFieldMenu
                  label={t('authorizationDetailTypes:composer.add.toObject', 'Add field to {{name}}', {
                    name: labels.heading(field),
                    interpolation: {escapeValue: false},
                  })}
                  tabIndex={field.key === tabKey ? 0 : -1}
                  onAdd={(item) => actions.add(item, field.key, 0)}
                />
              </Box>
            ))}
          <CanvasLevel
            list={field.children}
            parent={field}
            depth={depth + 1}
            prefix={path}
            inherited={processorSupplied}
            shared={shared}
          />
        </CanvasGroup>
      ) : (
        // The consent screen never shows the type member, which every detail carries.
        <CanvasRow {...common} path={path} notShown={depth === 0 && name === 'type'} />
      )}
    </Box>
  );
});

/**
 * The consent screen of the type, rendered as the user sees it and edited in place, as the flow
 * composer edits the elements of a screen: the summary and each field are selected on the card to edit
 * them in the side panel, a plus between rows adds a field there, and a row is dragged by its handle to
 * move it, or moved with its menu or Alt and the arrow keys.
 */
export default function ConsentCanvas({
  fields,
  typeValue,
  consent,
  issueKeys,
  readOnly,
  selection,
  onSelect,
  onAdd,
  onMove,
  onDuplicate,
  onRemove,
  standardFields,
  onAddStandard,
}: ConsentCanvasProps): JSX.Element {
  const {t} = useTranslation();
  const selectedKey = selection?.kind === 'field' ? selection.key : null;
  const summarySelected = selection?.kind === 'summary';
  const summaryInvalid = isConsentPreviewInvalid(consent);
  // As on the server, the card is headed by the summary, or by the type identifier without one.
  const heading = consent?.text.trim() ? consent.text : typeValue.trim();
  const [focusedKey, setFocusedKey] = useState<string | null>(null);
  // As in a tree, the fields take one stop in the Tab order, with the controls of that field: the field
  // focused last, or else the selected one, or else the first.
  const tabKey =
    [focusedKey, selectedKey].find((key) => key !== null && findField(fields, key) !== undefined) ??
    fields[0]?.key ??
    null;

  // The latest callbacks, called through actions that stay the same, so the rows keep their props.
  const callbacks = useRef({onSelect, onAdd, onMove, onDuplicate, onRemove});
  useEffect(() => {
    callbacks.current = {onSelect, onAdd, onMove, onDuplicate, onRemove};
  }, [onSelect, onAdd, onMove, onDuplicate, onRemove]);
  const actions = useMemo<CanvasActions>(
    () => ({
      select: (key) => callbacks.current.onSelect({kind: 'field', key}),
      move: (key, move) => callbacks.current.onMove(key, move),
      duplicate: (key) => callbacks.current.onDuplicate(key),
      remove: (key) => callbacks.current.onRemove(key),
      add: (item, parentKey, index) => callbacks.current.onAdd(item, parentKey, index),
    }),
    [],
  );
  // The moves of the fields, kept while they are the same, as they are while a field is edited.
  const abilitiesText = useMemo(() => JSON.stringify([...moveAbilities(fields)]), [fields]);
  const abilities = useMemo(
    () => new Map<string, MoveAbility>(JSON.parse(abilitiesText) as [string, MoveAbility][]),
    [abilitiesText],
  );
  const shared = useMemo<CanvasShared>(
    () => ({selectedKey, tabKey, issueKeys, readOnly, abilities, actions}),
    [selectedKey, tabKey, issueKeys, readOnly, abilities, actions],
  );

  const handleTreeFocus = (e: FocusEvent<HTMLElement>): void => {
    const key = (e.target as HTMLElement).closest<HTMLElement>('[data-field-key]')?.dataset['fieldKey'];
    if (key) setFocusedKey(key);
  };

  // The arrow keys, Home, and End move focus between the fields, as in a tree; Alt and an arrow moves a field.
  const handleTreeKeyDown = (e: KeyboardEvent<HTMLElement>): void => {
    const target = e.target as HTMLElement;
    if (e.altKey || e.defaultPrevented || target.getAttribute('role') !== 'treeitem') return;
    const items = Array.from(e.currentTarget.querySelectorAll<HTMLElement>('[role="treeitem"]'));
    const index = items.indexOf(target);
    const next: Record<string, number> = {ArrowUp: index - 1, ArrowDown: index + 1, Home: 0, End: items.length - 1};
    if (!(e.key in next)) return;
    e.preventDefault();
    items[next[e.key]]?.focus();
  };

  return (
    <Box
      data-testid="consent-canvas"
      onClick={() => onSelect(null)}
      sx={(theme) => ({
        display: 'flex',
        justifyContent: 'center',
        px: {xs: 2, sm: 4},
        py: {xs: 3, sm: 5},
        minHeight: 420,
        // The card's rows place their toolbars by the room the canvas leaves beside the card.
        containerType: 'inline-size',
        containerName: CANVAS_CONTAINER,
        bgcolor: 'background.default',
        backgroundImage: `radial-gradient(${theme.palette.divider} 1px, transparent 1px)`,
        backgroundSize: '16px 16px',
      })}
    >
      {/* The page the consent screen shows the detail on, as the gate lays it out. */}
      <Box
        role="region"
        aria-label={t('authorizationDetailTypes:composer.canvas.card', 'Consent card')}
        sx={{
          width: '100%',
          // As wide as the consent screen lays the detail out.
          maxWidth: 426,
          alignSelf: 'flex-start',
          p: 2.5,
          pt: 1.5,
          borderRadius: 3,
          border: '1px solid',
          borderColor: 'divider',
          bgcolor: 'background.paper',
          boxShadow: (theme) => theme.shadows[2],
        }}
      >
        <Box sx={{display: 'flex', flexDirection: 'column', gap: 2, mt: 1}}>
          <Box sx={{border: 1, borderColor: 'divider', borderRadius: 1, p: 1.5, mt: 1}}>
            <Box
              id={CANVAS_SUMMARY_ID}
              role="button"
              tabIndex={0}
              aria-pressed={summarySelected}
              aria-label={t('authorizationDetailTypes:composer.summary.label', 'Consent summary')}
              data-testid="canvas-summary"
              onClick={(e) => {
                e.stopPropagation();
                onSelect({kind: 'summary'});
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  onSelect({kind: 'summary'});
                }
              }}
              sx={{mb: 1, ...elementChrome(summarySelected, summaryInvalid)}}
            >
              {/* Clamped as on the consent screen; its toggle only shows that it is. */}
              <Box inert>
                <AuthorizationDetailSummary text={heading} />
              </Box>
            </Box>

            {fields.length === 0 && (
              <Typography variant="body2" color="text.secondary" component="p" sx={{px: 1, py: 2, textAlign: 'center'}}>
                {readOnly
                  ? t('authorizationDetailTypes:composer.canvas.none', 'This intent has no fields.')
                  : t(
                      'authorizationDetailTypes:composer.canvas.empty',
                      'Add the values a request carries, such as an amount.',
                    )}
              </Typography>
            )}

            <Box
              role="tree"
              aria-label={t('authorizationDetailTypes:composer.canvas.fields', 'Fields')}
              onKeyDown={handleTreeKeyDown}
              onFocus={handleTreeFocus}
              // A container, whose width the drop line of a gap set in at the end of an object runs across.
              sx={{display: 'flex', flexDirection: 'column', mb: 1, containerType: 'inline-size'}}
            >
              <CanvasLevel list={fields} parent={null} depth={0} prefix="" inherited={false} shared={shared} />
            </Box>

            {!readOnly && (
              <Box sx={{mb: 1}}>
                <AddFieldMenu
                  onAdd={(item) => onAdd(item, null, fields.length)}
                  standardFields={standardFields}
                  onAddStandard={onAddStandard}
                />
              </Box>
            )}

            {/* The approval the user gives; shown as on the consent screen, and never changed here. */}
            <Box inert sx={{px: 1}}>
              <Divider sx={{opacity: 0.5}} />
              <FormControlLabel
                control={<Switch checked size="small" />}
                label={
                  <Typography variant="body2" sx={{fontWeight: 500}}>
                    {t('consent:authorizationDetails.approve', 'Approve')}
                  </Typography>
                }
                labelPlacement="start"
                sx={{m: 0, width: '100%', justifyContent: 'space-between', py: 0.5}}
              />
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
