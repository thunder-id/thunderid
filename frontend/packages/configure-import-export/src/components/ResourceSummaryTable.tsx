// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Box,
  Chip,
  Collapse,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from '@wso2/oxygen-ui';
import {ChevronRight} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {Fragment, useState} from 'react';
import {useTranslation} from 'react-i18next';
import type {ConfigSummaryItem} from '../models/import-configuration';

/**
 * Props for the {@link ResourceSummaryTable} component.
 *
 * @public
 */
export interface ResourceSummaryTableProps {
  /**
   * Array of resource items to display
   */
  items: ConfigSummaryItem[];
  /**
   * Show status column (for export)
   */
  showStatus?: boolean;
  /**
   * Show dependencies column (for export)
   */
  showDependencies?: boolean;
  /**
   * Translation function for status and dependency labels
   */
  t?: (key: string, params?: Record<string, unknown>) => string;
}

/**
 * Shared resource summary table component used in both import and export flows.
 * Displays resource types with collapsible details.
 *
 * @public
 */
export default function ResourceSummaryTable({
  items,
  showStatus = false,
  showDependencies = false,
  t = (key: string) => key,
}: ResourceSummaryTableProps): JSX.Element {
  const {t: ti18n} = useTranslation('importExport');
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  const toggle = (id: string): void => setExpanded((prev) => ({...prev, [id]: !prev[id]}));

  return (
    <Paper variant="outlined" sx={{overflow: 'hidden'}}>
      <Table size="small">
        <TableHead>
          <TableRow sx={{'& th': {bgcolor: 'action.hover'}}}>
            <TableCell sx={{width: 48}} />
            <TableCell>
              <Typography variant="caption" sx={{fontWeight: 'bold'}}>
                {showStatus || showDependencies ? ti18n('table.item') : ti18n('table.resourceType')}
              </Typography>
            </TableCell>
            {showStatus && (
              <TableCell sx={{width: 110}}>
                <Typography variant="caption" sx={{fontWeight: 'bold'}}>
                  {ti18n('table.status')}
                </Typography>
              </TableCell>
            )}
            {showDependencies && (
              <TableCell sx={{width: 140}}>
                <Typography variant="caption" sx={{fontWeight: 'bold'}}>
                  {ti18n('table.dependencies')}
                </Typography>
              </TableCell>
            )}
            {!showStatus && !showDependencies && (
              <TableCell sx={{width: 120}}>
                <Typography variant="caption" sx={{fontWeight: 'bold'}}>
                  {ti18n('table.count')}
                </Typography>
              </TableCell>
            )}
          </TableRow>
        </TableHead>
        <TableBody>
          {items.length === 0 ? (
            <TableRow>
              <TableCell colSpan={showStatus || showDependencies ? 4 : 3} sx={{py: 4, textAlign: 'center'}}>
                <Typography variant="body2" color="text.secondary">
                  {ti18n('table.noResources')}
                </Typography>
              </TableCell>
            </TableRow>
          ) : (
            items.map((item) => (
              <Fragment key={item.id}>
                <TableRow hover sx={{cursor: 'pointer'}} onClick={() => toggle(item.id)}>
                  <TableCell sx={{py: 1, pl: 1.5}}>
                    <IconButton
                      size="small"
                      onClick={(e) => {
                        e.stopPropagation();
                        toggle(item.id);
                      }}
                    >
                      <Box
                        component="span"
                        sx={{
                          display: 'flex',
                          transform: expanded[item.id] ? 'rotate(90deg)' : 'rotate(0deg)',
                          transition: 'transform 0.2s',
                        }}
                      >
                        <ChevronRight size={16} />
                      </Box>
                    </IconButton>
                  </TableCell>
                  <TableCell>
                    <Stack direction="row" spacing={1} alignItems="center">
                      {item.icon}
                      <Typography variant="body2" sx={{fontWeight: 'medium'}}>
                        {item.label}
                      </Typography>
                    </Stack>
                  </TableCell>
                  {showStatus && (
                    <TableCell>
                      {item.status === 'ready' ? (
                        <Chip label={t('export.status.ready')} size="small" color="success" />
                      ) : (
                        <Chip label={t('export.status.warning')} size="small" color="warning" />
                      )}
                    </TableCell>
                  )}
                  {showDependencies && (
                    <TableCell>
                      <Typography variant="body2" color="text.secondary">
                        {item.dependencyCount && item.dependencyCount > 0
                          ? t('export.table.dependencyCount', {count: item.dependencyCount})
                          : t('export.table.noDependencies')}
                      </Typography>
                    </TableCell>
                  )}
                  {!showStatus && !showDependencies && (
                    <TableCell>
                      <Typography variant="body2" fontWeight={600}>
                        {item.value}
                      </Typography>
                    </TableCell>
                  )}
                </TableRow>
                <TableRow>
                  <TableCell colSpan={showStatus || showDependencies ? 4 : 3} sx={{p: 0, border: 0}}>
                    <Collapse in={Boolean(expanded[item.id])} timeout="auto" unmountOnExit>
                      {item.content ?? (
                        <Box sx={{px: 5, py: 1.5, bgcolor: 'background.default'}}>
                          <Typography variant="body2" color="text.secondary">
                            {ti18n('table.noDetails')}
                          </Typography>
                        </Box>
                      )}
                    </Collapse>
                  </TableCell>
                </TableRow>
              </Fragment>
            ))
          )}
        </TableBody>
      </Table>
    </Paper>
  );
}
