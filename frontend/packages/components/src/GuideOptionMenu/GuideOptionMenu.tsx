// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Button, ListItemIcon, ListItemText, Menu, MenuItem} from '@wso2/oxygen-ui';
import {Check, ChevronDown} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type ReactNode} from 'react';

/**
 * One choice of a {@link GuideOptionMenu}.
 *
 * @public
 */
export interface GuideOption {
  /** What identifies the choice */
  value: string;
  /** The choice's name */
  label: string;
  /** The choice's logo */
  icon?: ReactNode;
}

/**
 * Props for the {@link GuideOptionMenu} component.
 *
 * @public
 */
export interface GuideOptionMenuProps {
  /** The menu's id, unique on the page */
  id: string;
  /** What the choices are, for someone who cannot see the menu (e.g. "Technology") */
  menuLabel: string;
  /** The choices */
  options: GuideOption[];
  /** The choice made */
  value: string;
  /** Makes a choice */
  onSelect: (value: string) => void;
}

/**
 * What a guide is written for, such as a technology or a language, as a button naming the choice
 * made that opens a menu of the others.
 *
 * @public
 */
export default function GuideOptionMenu({id, menuLabel, options, value, onSelect}: GuideOptionMenuProps): JSX.Element {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const selected = options.find((option) => option.value === value) ?? options[0];

  return (
    <>
      <Button
        variant="outlined"
        color="inherit"
        aria-haspopup="menu"
        aria-expanded={Boolean(anchor)}
        aria-controls={anchor ? id : undefined}
        startIcon={selected.icon}
        endIcon={<ChevronDown size={18} aria-hidden />}
        onClick={(event) => setAnchor(event.currentTarget)}
        sx={{
          font: 'inherit',
          textTransform: 'none',
          borderRadius: 1,
          py: 0.25,
          px: 1.25,
          borderColor: 'divider',
          '&:hover': {borderColor: 'text.secondary'},
        }}
      >
        {selected.label}
      </Button>
      <Menu
        id={id}
        anchorEl={anchor}
        open={Boolean(anchor)}
        onClose={() => setAnchor(null)}
        slotProps={{list: {'aria-label': menuLabel}}}
      >
        {options.map((option) => (
          <MenuItem
            key={option.value}
            role="menuitemradio"
            aria-checked={option.value === selected.value}
            selected={option.value === selected.value}
            onClick={() => {
              onSelect(option.value);
              setAnchor(null);
            }}
          >
            {option.icon && <ListItemIcon>{option.icon}</ListItemIcon>}
            <ListItemText>{option.label}</ListItemText>
            {option.value === selected.value && <Check size={16} aria-hidden />}
          </MenuItem>
        ))}
      </Menu>
    </>
  );
}
