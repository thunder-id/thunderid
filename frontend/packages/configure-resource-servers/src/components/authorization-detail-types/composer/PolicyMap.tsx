// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, ButtonBase, Typography} from '@wso2/oxygen-ui';
import {Lock, PencilLine, Server} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useComposerLabels from './useComposerLabels';
import type {SchemaField} from '../../../models/authorization-detail-type';
import {effectiveModification} from '../../../utils/authorization-detail-schema';
import {flattenFields} from '../../../utils/schema-composer';

export interface PolicyMapProps {
  fields: SchemaField[];
  /** Selects a field for editing; left out where fields cannot be selected. */
  onSelect?: (key: string) => void;
}

/**
 * Lists each field that holds a value with who sets it and who may change it, in a plain sentence, so
 * the rules of the whole type can be read at a glance.
 */
export default function PolicyMap({fields, onSelect = undefined}: PolicyMapProps): JSX.Element {
  const {t} = useTranslation();
  const labels = useComposerLabels();
  // An object only groups its fields, which carry the rules, so only fields that hold a value are listed.
  const rows = flattenFields(fields).filter((row) => row.field.name.trim() && row.field.kind !== 'object');

  if (rows.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        {t('authorizationDetailTypes:composer.canvas.none', 'This intent has no fields.')}
      </Typography>
    );
  }

  return (
    <Box
      component="ul"
      aria-label={t('authorizationDetailTypes:composer.overview.policy', 'Who can change what')}
      sx={{listStyle: 'none', m: 0, mx: -1, p: 0}}
    >
      {rows.map((row) => {
        const changeable = !row.processorSupplied && effectiveModification(row.field) !== 'none';
        const summary = labels.policySummary(row);
        let icon = <Lock size={14} />;
        if (row.processorSupplied) icon = <Server size={14} />;
        else if (changeable) icon = <PencilLine size={14} />;
        return (
          <Box component="li" key={row.field.key} data-testid={`policy-row-${row.field.key}`}>
            <ButtonBase
              disabled={!onSelect}
              aria-label={`${labels.heading(row.field)}: ${summary}`}
              onClick={() => onSelect?.(row.field.key)}
              sx={{
                width: '100%',
                display: 'flex',
                alignItems: 'flex-start',
                gap: 1.25,
                py: 1,
                px: 1,
                borderRadius: 1,
                textAlign: 'left',
                '&:hover': {bgcolor: 'action.hover'},
                '&:focus-visible': {outline: '2px solid', outlineColor: 'primary.main'},
              }}
            >
              <Box
                component="span"
                sx={{display: 'flex', mt: 0.25, color: changeable ? 'primary.main' : 'text.secondary'}}
              >
                {icon}
              </Box>
              <Box component="span" sx={{minWidth: 0, flex: 1}}>
                <Box component="span" sx={{display: 'flex', alignItems: 'baseline', gap: 1, minWidth: 0}}>
                  <Typography variant="body2" component="span" sx={{fontWeight: 500}} noWrap>
                    {labels.heading(row.field)}
                  </Typography>
                  <Typography
                    variant="caption"
                    component="span"
                    color="text.secondary"
                    sx={{fontFamily: 'monospace'}}
                    noWrap
                  >
                    {row.path}
                  </Typography>
                </Box>
                <Typography variant="caption" component="span" color="text.secondary" sx={{display: 'block'}}>
                  {summary}
                </Typography>
              </Box>
            </ButtonBase>
          </Box>
        );
      })}
    </Box>
  );
}
