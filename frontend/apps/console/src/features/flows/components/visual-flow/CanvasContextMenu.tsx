// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Divider, ListItemIcon, ListItemText, Menu, MenuItem, Typography} from '@wso2/oxygen-ui';
import {
  ChevronLeft,
  ChevronRight,
  CogIcon,
  Copy,
  EyeIcon,
  LayoutGrid,
  Maximize,
  PlusIcon,
  TrashIcon,
} from '@wso2/oxygen-ui-icons-react';
import {useState, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {Resource} from '../../models/resources';
import ResourceDisplayImage from '../ResourceDisplayImage';

const IS_APPLE_PLATFORM = typeof navigator !== 'undefined' && /mac|iphone|ipad|ipod/i.test(navigator.platform ?? '');

/**
 * Where the canvas context menu was opened: on a node or on the canvas
 * background, at the given client coordinates.
 */
export interface CanvasContextMenuTarget {
  kind: 'node' | 'pane';
  nodeId?: string;
  position: {left: number; top: number};
}

/**
 * Props interface of {@link CanvasContextMenu}
 */
export interface CanvasContextMenuProps {
  /** The current menu target, or `null` when the menu is closed. */
  target: CanvasContextMenuTarget | null;
  /** Closes the menu. */
  onClose: () => void;
  /** Whether the targeted node can be duplicated. */
  canDuplicate?: boolean;
  /** Whether the targeted node can be deleted. */
  canDelete?: boolean;
  /** Whether the targeted node has a properties panel. */
  hasProperties?: boolean;
  /** Duplicates the targeted node. */
  onDuplicate?: () => void;
  /** Deletes the targeted node. */
  onDelete?: () => void;
  /** Opens the targeted node's properties panel. */
  onOpenProperties?: () => void;
  /** Starts the flow preview from the targeted node. */
  onPreviewFromStep?: () => void;
  /** Step resources offered by the "Add step" submenu. */
  addableSteps?: Resource[];
  /** Adds the given step resource at the menu's canvas position. */
  onAddStep?: (step: Resource) => void;
  /** Runs auto-layout on the canvas. */
  onAutoLayout?: () => void;
  /** Fits the whole flow into the viewport. */
  onFitView?: () => void;
}

/**
 * Right-click context menu for the flow builder canvas. On a node it offers
 * the node actions (duplicate, properties, preview, delete); on the canvas
 * background it offers canvas actions, including an "Add step" drill-in that
 * places the chosen step at the clicked position.
 *
 * @param props - Props injected to the component.
 * @returns The CanvasContextMenu component.
 */
function CanvasContextMenu({
  target,
  onClose,
  canDuplicate = false,
  canDelete = false,
  hasProperties = false,
  onDuplicate = undefined,
  onDelete = undefined,
  onOpenProperties = undefined,
  onPreviewFromStep = undefined,
  addableSteps = [],
  onAddStep = undefined,
  onAutoLayout = undefined,
  onFitView = undefined,
}: CanvasContextMenuProps): ReactElement {
  const {t} = useTranslation();
  const [isAddStepView, setIsAddStepView] = useState<boolean>(false);

  // Each fresh right-click starts at the menu root, never inside the drill-in
  // (state adjustment during render, per the React "adjusting state when props
  // change" pattern).
  const [prevTarget, setPrevTarget] = useState<CanvasContextMenuTarget | null>(target);
  if (prevTarget !== target) {
    setPrevTarget(target);
    if (isAddStepView) {
      setIsAddStepView(false);
    }
  }

  const open = Boolean(target);

  let items: ReactNode[];

  if (target?.kind === 'node') {
    items = [
      <MenuItem
        key="duplicate"
        disabled={!canDuplicate}
        data-testid="canvas-context-menu-duplicate"
        onClick={() => {
          onDuplicate?.();
          onClose();
        }}
      >
        <ListItemIcon>
          <Copy size={16} />
        </ListItemIcon>
        <ListItemText>{t('flows:core.contextMenu.duplicate', 'Duplicate')}</ListItemText>
        <Typography variant="body2" color="text.secondary" sx={{ml: 3}}>
          {IS_APPLE_PLATFORM ? '⌘D' : 'Ctrl+D'}
        </Typography>
      </MenuItem>,
    ];

    if (hasProperties) {
      items.push(
        <MenuItem
          key="open-properties"
          data-testid="canvas-context-menu-open-properties"
          onClick={() => {
            onOpenProperties?.();
            onClose();
          }}
        >
          <ListItemIcon>
            <CogIcon size={16} />
          </ListItemIcon>
          <ListItemText>{t('flows:core.contextMenu.openProperties', 'Open properties')}</ListItemText>
        </MenuItem>,
      );
    }

    items.push(
      <MenuItem
        key="preview"
        data-testid="canvas-context-menu-preview"
        onClick={() => {
          onPreviewFromStep?.();
          onClose();
        }}
      >
        <ListItemIcon>
          <EyeIcon size={16} />
        </ListItemIcon>
        <ListItemText>{t('flows:core.contextMenu.previewFromStep', 'Preview from this step')}</ListItemText>
      </MenuItem>,
      <Divider key="node-divider" />,
      <MenuItem
        key="delete"
        disabled={!canDelete}
        data-testid="canvas-context-menu-delete"
        sx={{color: 'error.main'}}
        onClick={() => {
          onDelete?.();
          onClose();
        }}
      >
        <ListItemIcon sx={{color: 'error.main'}}>
          <TrashIcon size={16} />
        </ListItemIcon>
        <ListItemText>{t('flows:core.contextMenu.delete', 'Delete')}</ListItemText>
        <Typography variant="body2" color="text.secondary" sx={{ml: 3}}>
          {IS_APPLE_PLATFORM ? '⌫' : 'Del'}
        </Typography>
      </MenuItem>,
    );
  } else if (isAddStepView) {
    items = [
      <MenuItem key="back" data-testid="canvas-context-menu-add-step-back" onClick={() => setIsAddStepView(false)}>
        <ListItemIcon>
          <ChevronLeft size={16} />
        </ListItemIcon>
        <ListItemText>{t('flows:core.contextMenu.addStep', 'Add step')}</ListItemText>
      </MenuItem>,
      <Divider key="add-step-divider" />,
      ...addableSteps.map((step: Resource, index: number) => (
        <MenuItem
          key={step.id ?? `${step.type}-${step.display?.label ?? ''}-${index}`}
          data-testid={`canvas-context-menu-add-step-${index}`}
          onClick={() => {
            onAddStep?.(step);
            onClose();
          }}
        >
          <ListItemIcon>
            <ResourceDisplayImage
              image={step.display?.image}
              label={step.display?.label}
              size={16}
              preserveColor={step.display?.preserveImageColor}
            />
          </ListItemIcon>
          <ListItemText>{step.display?.label ?? step.type}</ListItemText>
        </MenuItem>
      )),
    ];
  } else {
    items = [
      <MenuItem key="add-step" data-testid="canvas-context-menu-add-step" onClick={() => setIsAddStepView(true)}>
        <ListItemIcon>
          <PlusIcon size={16} />
        </ListItemIcon>
        <ListItemText>{t('flows:core.contextMenu.addStep', 'Add step')}</ListItemText>
        <ChevronRight size={16} />
      </MenuItem>,
      <Divider key="pane-divider" />,
      <MenuItem
        key="auto-layout"
        data-testid="canvas-context-menu-auto-layout"
        onClick={() => {
          onAutoLayout?.();
          onClose();
        }}
      >
        <ListItemIcon>
          <LayoutGrid size={16} />
        </ListItemIcon>
        <ListItemText>{t('flows:core.headerPanel.autoLayout', 'Auto Layout')}</ListItemText>
      </MenuItem>,
      <MenuItem
        key="fit-view"
        data-testid="canvas-context-menu-fit-view"
        onClick={() => {
          onFitView?.();
          onClose();
        }}
      >
        <ListItemIcon>
          <Maximize size={16} />
        </ListItemIcon>
        <ListItemText>{t('flows:core.headerPanel.fitView', 'Fit view')}</ListItemText>
      </MenuItem>,
    ];
  }

  return (
    <Menu
      open={open}
      onClose={onClose}
      anchorReference="anchorPosition"
      anchorPosition={target ? {left: target.position.left, top: target.position.top} : undefined}
      slotProps={{paper: {sx: {maxHeight: 420, minWidth: 220}}}}
      data-testid="canvas-context-menu"
    >
      {items}
    </Menu>
  );
}

export default CanvasContextMenu;
