// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {IconButton, ListItemIcon, ListItemText, Menu, MenuItem, Tooltip} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronRight, Plus} from '@wso2/oxygen-ui-icons-react';
import {useState, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {Project} from '../models/project';

export interface ProjectQuickSelectorProps {
  projects: Project[];
  onSelect: (id: string) => void;
  /** Opens the project creation, offered first in the menu when given. */
  onCreate?: () => void;
}

/**
 * The header button shown while no project is selected: it opens a menu that offers to create a
 * project, then lists the projects, and choosing one enters it.
 */
export default function ProjectQuickSelector({
  projects,
  onSelect,
  onCreate = undefined,
}: ProjectQuickSelectorProps): ReactNode {
  const {t} = useTranslation();
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);

  const handleClose = (): void => setAnchorEl(null);

  const selectLabel = t('projects:switcher.select', 'Select project');

  return (
    <>
      <Tooltip title={selectLabel}>
        <IconButton
          size="small"
          aria-label={selectLabel}
          aria-haspopup="menu"
          aria-expanded={anchorEl ? 'true' : undefined}
          onClick={(event) => setAnchorEl(event.currentTarget)}
          sx={{width: 32, height: 32, border: 1, borderColor: 'divider'}}
        >
          {anchorEl ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
        </IconButton>
      </Tooltip>
      <Menu
        anchorEl={anchorEl}
        open={anchorEl !== null}
        onClose={handleClose}
        anchorOrigin={{vertical: 'bottom', horizontal: 'left'}}
        transformOrigin={{vertical: 'top', horizontal: 'left'}}
        slotProps={{paper: {sx: {mt: 1, minWidth: 200}}}}
      >
        {onCreate && (
          <MenuItem
            onClick={() => {
              handleClose();
              onCreate();
            }}
            data-testid="project-create"
          >
            <ListItemIcon>
              <Plus size={18} />
            </ListItemIcon>
            <ListItemText primary={t('projects:switcher.create', 'Create a Project')} />
          </MenuItem>
        )}
        {projects.map((project) => (
          <MenuItem
            key={project.id}
            onClick={() => {
              handleClose();
              onSelect(project.id);
            }}
          >
            <ListItemText primary={project.displayName} />
          </MenuItem>
        ))}
      </Menu>
    </>
  );
}
