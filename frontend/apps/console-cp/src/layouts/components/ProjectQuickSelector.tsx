// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Divider, IconButton, ListItemText, Menu, MenuItem, SearchBar, Tooltip, Typography} from '@wso2/oxygen-ui';
import {ChevronRight} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {Project} from '../../features/projects/useGetProjects';

export interface ProjectQuickSelectorProps {
  projects: Project[];
  onSelect: (id: string) => void;
}

/**
 * The header button shown while no project is selected: it opens a searchable menu of the
 * projects, and choosing one enters it.
 */
export default function ProjectQuickSelector({projects, onSelect}: ProjectQuickSelectorProps): ReactNode {
  const {t} = useTranslation();
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const [search, setSearch] = useState('');

  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (!query) {
      return projects;
    }

    return projects.filter((project) =>
      [project.displayName, project.handle].some((value) => value.toLowerCase().includes(query)),
    );
  }, [projects, search]);

  const handleClose = (): void => {
    setAnchorEl(null);
    setSearch('');
  };

  const selectLabel = t('common:header.selectProject', 'Select project');

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
          <ChevronRight size={16} />
        </IconButton>
      </Tooltip>
      <Menu
        anchorEl={anchorEl}
        open={anchorEl !== null}
        onClose={handleClose}
        anchorOrigin={{vertical: 'bottom', horizontal: 'left'}}
        transformOrigin={{vertical: 'top', horizontal: 'left'}}
        slotProps={{paper: {sx: {mt: 1, width: 280}}}}
      >
        <Box sx={{px: 1.5, pt: 1, pb: 1}} onKeyDown={(event) => event.stopPropagation()}>
          <Typography variant="overline" color="text.secondary" component="div">
            {t('common:header.projects', 'Projects')}
          </Typography>
          <SearchBar
            size="small"
            fullWidth
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t('common:projects.search', 'Search projects')}
            slotProps={{htmlInput: {'aria-label': t('common:projects.search', 'Search projects')}}}
          />
        </Box>
        <Divider />
        {filtered.length === 0 ? (
          <MenuItem disabled>{t('common:projects.noMatches', 'No matching projects')}</MenuItem>
        ) : (
          filtered.map((project) => (
            <MenuItem
              key={project.id}
              onClick={() => {
                handleClose();
                onSelect(project.id);
              }}
            >
              <ListItemText primary={project.displayName} secondary={project.handle} />
            </MenuItem>
          ))
        )}
      </Menu>
    </>
  );
}
