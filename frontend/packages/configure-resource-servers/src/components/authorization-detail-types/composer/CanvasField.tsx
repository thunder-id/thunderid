// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {pointerIntersection} from '@dnd-kit/collision';
import {useDraggable, useDroppable} from '@dnd-kit/react';
import {AuthorizationDetailValue, getAuthorizationDetailRowLayout} from '@thunderid/design';
import {
  Box,
  Divider,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ArrowDown,
  ArrowUp,
  CircleAlert,
  Copy,
  EllipsisVertical,
  GripVertical,
  ListIndentDecrease,
  ListIndentIncrease,
  Lock,
  PencilLine,
  Trash2,
} from '@wso2/oxygen-ui-icons-react';
import {memo, useCallback, useState, type JSX, type KeyboardEvent, type ReactNode, type Ref} from 'react';
import {useTranslation} from 'react-i18next';
import AddFieldMenu from './AddFieldMenu';
import elementChrome, {CANVAS_CONTAINER, fieldRowId} from './elementChrome';
import KindIcon from './KindIcon';
import useComposerLabels from './useComposerLabels';
import type {SchemaField} from '../../../models/authorization-detail-type';
import {effectiveModification} from '../../../utils/authorization-detail-schema';
import {
  FIELD_DRAG_TYPE,
  previewRow,
  type DragSource,
  type FieldMove,
  type FieldPosition,
  type MoveAbility,
  type PaletteItem,
} from '../../../utils/schema-composer';

/**
 * The width of the canvas's content from which a row's toolbar fits beside the card, reaching into the
 * canvas's padding.
 */
const TOOLBAR_BESIDE_MIN_WIDTH = 550;

/** The purpose and detail ids the card's controls are named under; the card never submits them. */
const CANVAS_PURPOSE = 'composer';
const CANVAS_DETAIL = 'composer';
const NO_VALUES: Record<string, string> = {};
const ignoreInput = (): void => undefined;

/** The description of a field on the card, read by assistive technology after its name and not shown. */
function RowDescription({id, children}: {id: string; children: string}): JSX.Element {
  return (
    <Box
      component="span"
      id={id}
      sx={{
        position: 'absolute',
        width: 1,
        height: 1,
        p: 0,
        m: -1,
        overflow: 'hidden',
        clip: 'rect(0 0 0 0)',
        whiteSpace: 'nowrap',
        border: 0,
      }}
    >
      {children}
    </Box>
  );
}

/**
 * What the card does with its fields, by key, so the rows keep the same callbacks between edits and
 * redraw only when their own field changes.
 */
export interface CanvasActions {
  select: (key: string) => void;
  move: (key: string, move: FieldMove) => void;
  duplicate: (key: string) => void;
  remove: (key: string) => void;
  add: (item: PaletteItem, parentKey: string | null, index: number) => void;
}

interface FieldCallbacks {
  onSelect: () => void;
  onMove: (move: FieldMove) => void;
  onDuplicate: () => void;
  onRemove: () => void;
}

/** Returns the callbacks of a field, bound to its key. */
function fieldCallbacks(actions: CanvasActions, key: string): FieldCallbacks {
  return {
    onSelect: () => actions.select(key),
    onMove: (move) => actions.move(key, move),
    onDuplicate: () => actions.duplicate(key),
    onRemove: () => actions.remove(key),
  };
}

interface FieldElementProps {
  field: SchemaField;
  /** The level of the field among the fields, from 1. */
  level: number;
  /** Whether the processor supplies the field, itself or through an object it belongs to. */
  processorSupplied: boolean;
  selected: boolean;
  hasIssues: boolean;
  readOnly: boolean;
  canMove: MoveAbility;
  /**
   * Whether the field is the one stop of the fields in the Tab order, with its controls; the others are
   * reached with the arrow keys, as in a tree.
   */
  tabStop: boolean;
  actions: CanvasActions;
}

/** Moves a focused field with Alt and the arrow keys, and selects it with Enter or Space. */
function useFieldKeys(
  readOnly: boolean,
  canMove: MoveAbility,
  {onSelect, onMove}: Pick<FieldCallbacks, 'onSelect' | 'onMove'>,
) {
  return (e: KeyboardEvent<HTMLElement>): void => {
    if (e.target !== e.currentTarget) return;
    const moves: Record<string, FieldMove> = {
      ArrowUp: 'up',
      ArrowDown: 'down',
      ArrowRight: 'indent',
      ArrowLeft: 'outdent',
    };
    if (e.altKey && !readOnly && moves[e.key]) {
      e.preventDefault();
      if (canMove[moves[e.key]]) onMove(moves[e.key]);
    } else if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      onSelect();
    }
  };
}

interface ToolbarProps extends Omit<FieldCallbacks, 'onSelect'> {
  field: SchemaField;
  processorSupplied: boolean;
  hasIssues: boolean;
  readOnly: boolean;
  canMove: MoveAbility;
  handleRef: Ref<HTMLElement>;
  /** Whether the toolbar's controls are in the Tab order, as they are for the field that is. */
  tabStop: boolean;
  /** Shows the toolbar inside an object's tag rather than at the end of a row. */
  inline?: boolean;
}

/**
 * The small toolbar over a field on the card: its rule and any issue at a glance, a handle to drag it,
 * and a menu that duplicates, moves, or removes it.
 */
function FieldToolbar({
  field,
  processorSupplied,
  hasIssues,
  readOnly,
  canMove,
  handleRef,
  tabStop,
  inline = false,
  onMove,
  onDuplicate,
  onRemove,
}: ToolbarProps): JSX.Element | null {
  const {t} = useTranslation();
  const labels = useComposerLabels();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const heading = labels.heading(field);
  const suppliedHere = field.policy.suppliedBy === 'processor';
  const hasRule = suppliedHere || effectiveModification(field) !== 'none';
  const ruleSummary = hasRule ? labels.policySummary({field, processorSupplied}) : '';
  const issueLabel = t('authorizationDetailTypes:composer.canvas.hasIssues', 'This field needs attention');

  const moveLabels: Record<FieldMove, string> = {
    up: t('authorizationDetailTypes:composer.move.up', 'Move up'),
    down: t('authorizationDetailTypes:composer.move.down', 'Move down'),
    indent: t('authorizationDetailTypes:composer.move.indent', 'Move into the object above'),
    outdent: t('authorizationDetailTypes:composer.move.outdent', 'Move out of this object'),
  };
  const moveIcons: Record<FieldMove, JSX.Element> = {
    up: <ArrowUp size={16} />,
    down: <ArrowDown size={16} />,
    indent: <ListIndentIncrease size={16} />,
    outdent: <ListIndentDecrease size={16} />,
  };
  const closeMenu = (): void => setMenuAnchor(null);

  if (readOnly && !hasRule && !hasIssues) return null;

  return (
    <Box
      className={inline ? undefined : 'canvas-toolbar'}
      onClick={(e) => e.stopPropagation()}
      sx={{
        display: 'flex',
        alignItems: 'center',
        color: inline ? 'inherit' : 'text.secondary',
        '& > *': {display: 'flex', alignItems: 'center', px: inline ? 0.25 : 0.5},
        '& .MuiIconButton-root': {color: 'inherit'},
        ...(!inline && {
          // Over the end of its own row, shown only while the row is hovered, focused or selected, so the
          // values sit where the consent screen puts them and the toolbar never covers a neighbouring row.
          position: 'absolute',
          top: '50%',
          right: 8,
          transform: 'translateY(-50%)',
          // Beside the card where the canvas leaves room for it, so it never covers the row either.
          [`@container ${CANVAS_CONTAINER} (min-width: ${TOOLBAR_BESIDE_MIN_WIDTH}px)`]: {
            right: 'auto',
            left: 'calc(100% + 14px)',
            // Keeps the row hovered while the pointer crosses to the toolbar.
            '&::after': {content: '""', position: 'absolute', right: '100%', top: 0, bottom: 0, width: 16},
          },
          zIndex: 4,
          height: 26,
          px: 0.25,
          borderRadius: 1,
          border: '1px solid',
          borderColor: 'divider',
          // Opaque, as the paper of the theme is not, so the value under the toolbar does not show through.
          bgcolor: 'background.default',
          boxShadow: 1,
          transition: 'opacity 0.15s ease-in-out',
        }),
      }}
    >
      {hasRule && (
        <Tooltip title={ruleSummary}>
          <Box role="img" aria-label={ruleSummary}>
            {suppliedHere ? <Lock size={13} /> : <PencilLine size={13} />}
          </Box>
        </Tooltip>
      )}
      {hasIssues && (
        <Tooltip title={issueLabel}>
          <Box role="img" aria-label={issueLabel} sx={{color: 'error.main'}}>
            <CircleAlert size={14} />
          </Box>
        </Tooltip>
      )}
      {!readOnly && (
        <Box
          ref={handleRef}
          role="button"
          tabIndex={tabStop ? 0 : -1}
          aria-label={t('authorizationDetailTypes:composer.canvas.drag', 'Drag {{name}}', {
            name: heading,
            interpolation: {escapeValue: false},
          })}
          sx={{
            cursor: 'grab',
            '&:active': {cursor: 'grabbing'},
            '&:focus-visible': {outline: '2px solid', outlineColor: 'primary.main', borderRadius: 0.5},
          }}
        >
          <GripVertical size={14} />
        </Box>
      )}
      {!readOnly && (
        <Box>
          <IconButton
            size="small"
            aria-label={t('authorizationDetailTypes:composer.canvas.actions', 'Field actions')}
            aria-haspopup="menu"
            tabIndex={tabStop ? 0 : -1}
            onClick={(e) => setMenuAnchor(e.currentTarget)}
            sx={{p: 0.25}}
          >
            <EllipsisVertical size={14} />
          </IconButton>
          <Menu anchorEl={menuAnchor} open={Boolean(menuAnchor)} onClose={closeMenu}>
            <MenuItem
              onClick={() => {
                closeMenu();
                onDuplicate();
              }}
            >
              <ListItemIcon>
                <Copy size={16} />
              </ListItemIcon>
              <ListItemText>{t('authorizationDetailTypes:composer.canvas.duplicate', 'Duplicate')}</ListItemText>
            </MenuItem>
            {(['up', 'down', 'indent', 'outdent'] as FieldMove[]).map((move) => (
              <MenuItem
                key={move}
                disabled={!canMove[move]}
                onClick={() => {
                  closeMenu();
                  onMove(move);
                }}
              >
                <ListItemIcon>{moveIcons[move]}</ListItemIcon>
                <ListItemText>{moveLabels[move]}</ListItemText>
              </MenuItem>
            ))}
            <Divider />
            <MenuItem
              onClick={() => {
                closeMenu();
                onRemove();
              }}
              sx={{color: 'error.main'}}
            >
              <ListItemIcon sx={{color: 'inherit'}}>
                <Trash2 size={16} />
              </ListItemIcon>
              <ListItemText>{t('authorizationDetailTypes:fields.remove', 'Remove field')}</ListItemText>
            </MenuItem>
          </Menu>
        </Box>
      )}
    </Box>
  );
}

export interface CanvasRowProps extends FieldElementProps {
  /** The dotted path of the field. */
  path: string;
  /**
   * Marks a field the consent screen never shows, such as the type member, which still sits on the
   * card so it can be selected, moved, and removed.
   */
  notShown?: boolean;
}

/**
 * One field on the consent card, shown as the consent screen shows it: its label, and its value with
 * the control the user gets when the user may change it. The controls are inert, since selecting the
 * row edits the field instead.
 */
function CanvasRowElement({
  field,
  path,
  notShown = false,
  level,
  processorSupplied,
  selected,
  hasIssues,
  readOnly,
  canMove,
  tabStop,
  actions,
}: CanvasRowProps): JSX.Element {
  const {t} = useTranslation();
  const labels = useComposerLabels();
  const {onSelect, onMove, onDuplicate, onRemove} = fieldCallbacks(actions, field.key);
  const row = previewRow({field, path, processorSupplied});
  const data: DragSource = {key: field.key};
  const {ref, handleRef, isDragging} = useDraggable({
    id: `field:${field.key}`,
    type: FIELD_DRAG_TYPE,
    data,
    disabled: readOnly,
  });
  const handleKeyDown = useFieldKeys(readOnly, canMove, {onSelect, onMove});
  const heading = labels.heading(field);
  const layout = getAuthorizationDetailRowLayout(row);

  return (
    <Box
      // A read-only field cannot be dragged, so the row is not marked as a disabled draggable: it can
      // still be selected to see its settings.
      ref={readOnly ? undefined : ref}
      id={fieldRowId(field.key)}
      data-field-key={field.key}
      role="treeitem"
      tabIndex={tabStop ? 0 : -1}
      aria-level={level}
      aria-selected={selected}
      aria-label={heading}
      aria-describedby={`${field.key}-row-description`}
      data-testid={`field-row-${field.key}`}
      onClick={(e) => {
        e.stopPropagation();
        onSelect();
      }}
      onKeyDown={handleKeyDown}
      // Laid out as the consent screen lays out the field.
      sx={[
        layout.row,
        {
          opacity: isDragging ? 0.5 : 1,
          ...elementChrome(selected, hasIssues),
        },
      ]}
    >
      <Typography
        variant="body2"
        color="text.secondary"
        sx={[layout.label, {fontStyle: field.name.trim() ? 'normal' : 'italic'}]}
      >
        {field.name.trim() ? row.label : heading}
      </Typography>
      {notShown ? (
        <Typography variant="caption" color="text.secondary" sx={[layout.value, {fontStyle: 'italic'}]}>
          {t('authorizationDetailTypes:composer.canvas.notShown', 'Not shown on the consent screen')}
        </Typography>
      ) : (
        <Box
          inert
          sx={[
            layout.value,
            {
              // The consent screen shows a number's stepper once the pointer is over it, which never happens
              // on the inert card, so the card shows it all the time.
              '& input[type=number]::-webkit-inner-spin-button, & input[type=number]::-webkit-outer-spin-button': {
                opacity: 1,
              },
            },
          ]}
        >
          <AuthorizationDetailValue
            align={layout.stacked ? 'start' : 'end'}
            purposeId={CANVAS_PURPOSE}
            detailId={CANVAS_DETAIL}
            row={row}
            formValues={NO_VALUES}
            onInputChange={ignoreInput}
          />
        </Box>
      )}
      <FieldToolbar
        field={field}
        processorSupplied={processorSupplied}
        hasIssues={hasIssues}
        readOnly={readOnly}
        canMove={canMove}
        handleRef={handleRef}
        tabStop={tabStop}
        onMove={onMove}
        onDuplicate={onDuplicate}
        onRemove={onRemove}
      />
      <RowDescription id={`${field.key}-row-description`}>
        {labels.rowDescription({field, processorSupplied}, hasIssues)}
      </RowDescription>
    </Box>
  );
}

export const CanvasRow = memo(CanvasRowElement);

export interface CanvasGroupProps extends FieldElementProps {
  /** Whether a field inside the object is selected, which keeps the object's outline in view. */
  containsSelection: boolean;
  /** The level of nesting, which sets the order in which nested objects take a dropped field. */
  depth: number;
  children: ReactNode;
}

/**
 * An object on the consent card. The consent screen shows an object's fields as rows of their own, so
 * the object adds nothing to the card itself: a bracket and a tag that name the object show only while
 * it is hovered, focused, or selected, or while it has no fields.
 */
function CanvasGroupElement({
  field,
  level,
  depth,
  processorSupplied,
  selected,
  containsSelection,
  hasIssues,
  readOnly,
  canMove,
  tabStop,
  actions,
  children,
}: CanvasGroupProps): JSX.Element {
  const labels = useComposerLabels();
  const {onSelect, onMove, onDuplicate, onRemove} = fieldCallbacks(actions, field.key);
  const data: DragSource = {key: field.key};
  const {
    ref: dragRef,
    handleRef,
    isDragging,
  } = useDraggable({
    id: `field:${field.key}`,
    type: FIELD_DRAG_TYPE,
    data,
    disabled: readOnly,
  });
  const into: FieldPosition = {parentKey: field.key, index: field.children.length};
  const {ref: dropRef, isDropTarget} = useDroppable({
    id: `into:${field.key}`,
    accept: FIELD_DRAG_TYPE,
    data: into,
    disabled: readOnly,
    collisionDetector: pointerIntersection,
    // A nested object takes a dropped field before the object around it.
    collisionPriority: depth + 1,
  });
  const ref = useCallback(
    (element: Element | null) => {
      dragRef(element);
      dropRef(element);
    },
    [dragRef, dropRef],
  );
  const handleKeyDown = useFieldKeys(readOnly, canMove, {onSelect, onMove});
  const heading = labels.heading(field);
  const empty = field.children.length === 0;
  const shown = selected || containsSelection || hasIssues || isDropTarget;
  const pinned = selected || hasIssues || isDropTarget;
  const accent = hasIssues ? 'error.main' : selected || isDropTarget ? 'primary.main' : 'text.disabled';

  return (
    <Box
      ref={readOnly ? undefined : ref}
      className="canvas-group"
      data-testid={`field-group-${field.key}`}
      sx={{
        position: 'relative',
        opacity: isDragging ? 0.5 : 1,
        borderRadius: 1,
        ...(empty && {pb: 1}),
        ...((selected || isDropTarget) && {bgcolor: 'action.hover'}),
        // The bracket along the object's fields, in the card's padding.
        '&::before': {
          content: '""',
          position: 'absolute',
          left: -7,
          top: 2,
          bottom: 2,
          width: 5,
          border: '1.5px solid',
          borderRight: 'none',
          borderRadius: '4px 0 0 4px',
          borderColor: shown ? accent : 'transparent',
          pointerEvents: 'none',
          transition: 'border-color 0.15s ease-in-out',
        },
        '&:hover::before, &:focus-within::before': {borderColor: shown ? accent : 'text.disabled'},
        '& > .canvas-group-tag .canvas-group-tools': {
          opacity: selected || hasIssues ? 1 : 0,
          pointerEvents: selected || hasIssues ? 'auto' : 'none',
        },
        '& > .canvas-group-tag:focus-within .canvas-group-tools': {opacity: 1, pointerEvents: 'auto'},
        // As for a row, only a pointer that hovers reveals the object's controls.
        '@media (hover: hover)': {
          '&:hover > .canvas-group-tag .canvas-group-tools': {opacity: 1, pointerEvents: 'auto'},
        },
        // The consent screen does not show objects, so the card names one over the row above it, and only
        // while the object is in use. An empty object keeps its name in a line of its own, above its plus.
        ...(!empty && {
          '& > .canvas-group-tag': {
            position: 'absolute',
            bottom: 'calc(100% - 6px)',
            left: 0,
            zIndex: 5,
            minHeight: 22,
            border: '1px solid',
            borderColor: 'divider',
            bgcolor: 'background.default',
            boxShadow: 1,
            opacity: pinned ? 1 : 0,
            pointerEvents: pinned ? 'auto' : 'none',
            transition: 'opacity 0.15s ease-in-out',
          },
          // Only the innermost object the pointer is in, so the names of nested objects never overlap.
          '&:hover:not(:has(.canvas-group:hover)) > .canvas-group-tag, & > .canvas-group-tag:focus-within': {
            opacity: 1,
            pointerEvents: 'auto',
          },
        }),
      }}
    >
      {/* The object's name, inside the card, so it is never cut off by the canvas. */}
      <Box
        className="canvas-group-tag"
        id={fieldRowId(field.key)}
        data-field-key={field.key}
        role="treeitem"
        tabIndex={tabStop ? 0 : -1}
        aria-level={level}
        aria-selected={selected}
        aria-label={heading}
        aria-describedby={`${field.key}-row-description`}
        data-testid={`field-row-${field.key}`}
        onClick={(e) => {
          e.stopPropagation();
          onSelect();
        }}
        onKeyDown={handleKeyDown}
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 0.75,
          minHeight: 24,
          px: 1,
          borderRadius: 1,
          color: hasIssues ? 'error.main' : selected ? 'primary.main' : 'text.secondary',
          cursor: 'pointer',
          '&:hover': {color: hasIssues ? 'error.main' : 'primary.main'},
          '&:focus-visible': {outline: '2px solid', outlineColor: 'primary.main', outlineOffset: 1},
        }}
      >
        <KindIcon kind="object" size={12} />
        <Typography
          variant="caption"
          sx={{
            // Named as the consent screen names a field: by its display name, or else by its name.
            fontFamily: field.title.trim() ? 'inherit' : 'monospace',
            lineHeight: 1.2,
            color: 'inherit',
            fontWeight: selected ? 600 : 400,
            minWidth: 0,
          }}
          noWrap
        >
          {heading}
        </Typography>
        <Box
          className="canvas-group-tools"
          sx={{ml: 'auto', display: 'flex', color: 'text.secondary', transition: 'opacity 0.15s ease-in-out'}}
        >
          <FieldToolbar
            inline
            field={field}
            processorSupplied={processorSupplied}
            hasIssues={hasIssues}
            readOnly={readOnly}
            canMove={canMove}
            handleRef={handleRef}
            tabStop={tabStop}
            onMove={onMove}
            onDuplicate={onDuplicate}
            onRemove={onRemove}
          />
        </Box>
        <RowDescription id={`${field.key}-row-description`}>
          {labels.rowDescription({field, processorSupplied}, hasIssues)}
        </RowDescription>
      </Box>
      {children}
    </Box>
  );
}

export const CanvasGroup = memo(CanvasGroupElement);

export interface CanvasGapProps extends FieldPosition {
  /** The accessible name of the insertion point. */
  label: string;
  /**
   * How many levels in the gap's plus sits from the start of the row, or 0 for a gap across the row
   * with its plus in the middle. The end of an object shares its place with the gap after the object,
   * so its plus is set in by the object's level, apart from the plus of every gap there.
   */
  indent?: number;
  readOnly: boolean;
  /** Whether the plus is in the Tab order, as it is beside the field that is the fields' stop in it. */
  tabStop: boolean;
  onAdd: (item: PaletteItem, parentKey: string | null, index: number) => void;
}

/** The width a set-in gap takes for each level, in pixels. */
const GAP_INDENT = 24;

/**
 * A place between rows of the card, which takes up no room: a dragged field is dropped there, and on
 * hover or focus it offers a plus that adds a field there.
 */
function CanvasGapElement({
  parentKey,
  index,
  label,
  indent = 0,
  readOnly,
  tabStop,
  onAdd,
}: CanvasGapProps): JSX.Element | null {
  const position: FieldPosition = {parentKey, index};
  const {ref, isDropTarget} = useDroppable({
    id: `gap:${parentKey ?? 'root'}:${index}`,
    accept: FIELD_DRAG_TYPE,
    data: position,
    disabled: readOnly,
    collisionDetector: pointerIntersection,
    // A gap takes a dropped field before any object it sits in.
    collisionPriority: 100,
  });

  if (readOnly) return null;

  // A set-in gap takes only its own place at the start of the row, above the gap across the row, so the
  // pointer reaches each plus; its drop line still runs across the fields.
  const start = (indent - 1) * GAP_INDENT;
  const placement =
    indent > 0 ? {left: `${start}px`, width: `${GAP_INDENT}px`, zIndex: 3} : {left: 0, right: 0, zIndex: 2};

  return (
    <Box sx={{position: 'relative', height: 0}}>
      <Box
        ref={ref}
        data-testid={`gap-${parentKey ?? 'root'}-${index}`}
        sx={{
          position: 'absolute',
          ...placement,
          top: -5,
          height: 10,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          '&::before': {
            content: '""',
            position: 'absolute',
            left: 4,
            // The fields are a container, so a set-in gap's line runs to their end.
            ...(indent > 0 ? {width: `calc(100cqi - ${start + 8}px)`} : {right: 4}),
            top: '50%',
            height: '2px',
            transform: 'translateY(-50%)',
            borderRadius: '1px',
            bgcolor: 'primary.main',
            opacity: isDropTarget ? 1 : 0,
            pointerEvents: 'none',
          },
          '& .canvas-insert-button': {opacity: 0, transition: 'opacity 0.15s ease-in-out'},
          '&:hover::before': {opacity: 0.5},
          '&:hover .canvas-insert-button, & .canvas-insert-button:focus-visible, & .canvas-insert-button[aria-expanded="true"]':
            {opacity: 1},
          // Without a pointer that hovers, the plus never shows, so the gap leaves a tap to the row under it.
          '@media (hover: none)': {pointerEvents: 'none'},
        }}
      >
        <AddFieldMenu
          variant="insert"
          ariaLabel={label}
          tabIndex={tabStop ? 0 : -1}
          onAdd={(item) => onAdd(item, parentKey, index)}
        />
      </Box>
    </Box>
  );
}

export const CanvasGap = memo(CanvasGapElement);
