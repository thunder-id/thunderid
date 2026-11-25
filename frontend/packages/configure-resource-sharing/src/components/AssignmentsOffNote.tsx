// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import {Lock} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface AssignmentsOffNoteProps {
  /**
   * `section` when the rule covers every organization unit a section reaches, `unit` for one named unit.
   */
  scope: 'section' | 'unit';
}

/**
 * Says that a role's policy turns assignments off where it shows. The console only keeps such a rule,
 * so the note is read-only.
 */
export default function AssignmentsOffNote({scope}: AssignmentsOffNoteProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Stack direction="row" spacing={0.75} sx={{alignItems: 'flex-start', color: 'text.secondary'}}>
      <Box component="span" sx={{display: 'inline-flex', flexShrink: 0, mt: 0.25}}>
        <Lock size={14} aria-hidden />
      </Box>
      <Typography variant="caption">
        {scope === 'section'
          ? t('resourceSharing:assignmentsOff.section', 'Assignments are turned off for these organization units.')
          : t('resourceSharing:assignmentsOff.unit', 'Assignments are turned off for this organization unit.')}
      </Typography>
    </Stack>
  );
}
