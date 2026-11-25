// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Button, Divider, IconButton, ListItemIcon, ListItemText, Menu, MenuItem, Tooltip} from '@wso2/oxygen-ui';
import {ListPlus, Plus} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type MouseEvent} from 'react';
import {useTranslation} from 'react-i18next';
import KindIcon from './KindIcon';
import useComposerLabels from './useComposerLabels';
import type {SchemaField} from '../../../models/authorization-detail-type';
import {PALETTE_ITEMS, type PaletteItem} from '../../../utils/schema-composer';

/** The dashed add button of the consent card, as the flow composer's screens show theirs. */
const dashedAddSx = {
  py: 0.75,
  borderRadius: 1.5,
  border: '1.5px dashed',
  borderColor: 'divider',
  color: 'primary.main',
  fontWeight: 500,
  textTransform: 'none',
  bgcolor: 'action.hover',
  transition: 'all 0.18s ease',
  '&:hover': {border: '1.5px dashed', borderColor: 'primary.main', bgcolor: 'action.selected'},
} as const;

export interface AddFieldMenuProps {
  onAdd: (item: PaletteItem) => void;
  /** The accessible name of the button, and its tooltip as an insertion point. */
  ariaLabel?: string;
  /**
   * How the button shows: a dashed button across the card, or a small plus at an insertion point
   * between rows.
   */
  variant?: 'dashed' | 'insert';
  /** The label of a dashed button. */
  label?: string;
  /** The RFC 9396 common fields the type does not declare yet, offered at the top level. */
  standardFields?: SchemaField[];
  onAddStandard?: () => void;
  /** Takes the button out of the Tab order with -1, for one inside the fields, which take one stop. */
  tabIndex?: number;
}

/** A button that opens the kinds of field to add, with a short hint for each. */
export default function AddFieldMenu({
  onAdd,
  ariaLabel = undefined,
  variant = 'dashed',
  label = undefined,
  standardFields = [],
  onAddStandard = undefined,
  tabIndex = undefined,
}: AddFieldMenuProps): JSX.Element {
  const {t} = useTranslation();
  const labels = useComposerLabels();
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const hints: Record<PaletteItem, string> = {
    string: t('authorizationDetailTypes:composer.add.stringHint', 'Short free text'),
    number: t('authorizationDetailTypes:composer.add.numberHint', 'An amount or a count'),
    date: t('authorizationDetailTypes:composer.add.dateHint', 'A calendar day'),
    dateTime: t('authorizationDetailTypes:composer.add.dateTimeHint', 'A moment in time'),
    boolean: t('authorizationDetailTypes:composer.add.booleanHint', 'On or off'),
    array: t('authorizationDetailTypes:composer.add.arrayHint', 'Several values'),
    object: t('authorizationDetailTypes:composer.add.objectHint', 'Groups other fields'),
    money: t('authorizationDetailTypes:composer.add.moneyHint', 'An amount and a currency'),
  };

  const standardLabel = t('authorizationDetailTypes:fields.addStandard', 'Add standard fields ({{names}})', {
    names: standardFields.map((f) => f.name).join(', '),
    interpolation: {escapeValue: false},
  });

  const open = (e: MouseEvent<HTMLElement>): void => {
    // The button sits on the card, whose own click selects the card.
    e.stopPropagation();
    setAnchor(e.currentTarget);
  };
  const close = (): void => setAnchor(null);

  return (
    <>
      {variant === 'insert' ? (
        <Tooltip title={ariaLabel} placement="left">
          <IconButton
            size="small"
            className="canvas-insert-button"
            aria-label={ariaLabel}
            aria-haspopup="menu"
            aria-expanded={Boolean(anchor)}
            tabIndex={tabIndex}
            onClick={open}
            sx={{
              p: 0.25,
              bgcolor: 'primary.main',
              color: 'primary.contrastText',
              boxShadow: 1,
              '&:hover': {bgcolor: 'primary.dark'},
            }}
          >
            <Plus size={12} />
          </IconButton>
        </Tooltip>
      ) : (
        <Button
          fullWidth
          size="small"
          startIcon={<Plus size={15} />}
          aria-label={ariaLabel}
          aria-haspopup="menu"
          tabIndex={tabIndex}
          onClick={open}
          sx={dashedAddSx}
        >
          {label ?? t('authorizationDetailTypes:composer.add.button', 'Add field')}
        </Button>
      )}
      <Menu
        anchorEl={anchor}
        open={Boolean(anchor)}
        onClose={close}
        onClick={(e) => e.stopPropagation()}
        slotProps={{paper: {sx: {minWidth: 260}}}}
      >
        {PALETTE_ITEMS.map((item) => (
          <MenuItem
            key={item}
            aria-label={labels.palette[item]}
            onClick={() => {
              close();
              onAdd(item);
            }}
          >
            <ListItemIcon>
              <KindIcon kind={item} />
            </ListItemIcon>
            <ListItemText
              primary={labels.palette[item]}
              secondary={hints[item]}
              slotProps={{secondary: {variant: 'caption'}}}
            />
          </MenuItem>
        ))}
        {standardFields.length > 0 && onAddStandard && <Divider />}
        {standardFields.length > 0 && onAddStandard && (
          <MenuItem
            aria-label={standardLabel}
            onClick={() => {
              close();
              onAddStandard();
            }}
          >
            <ListItemIcon>
              <ListPlus size={16} aria-hidden />
            </ListItemIcon>
            <ListItemText
              primary={standardLabel}
              secondary={t('authorizationDetailTypes:composer.add.standardHint', 'Common RFC 9396 fields')}
              slotProps={{secondary: {variant: 'caption'}}}
            />
          </MenuItem>
        )}
      </Menu>
    </>
  );
}
