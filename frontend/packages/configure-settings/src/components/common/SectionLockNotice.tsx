// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box} from '@wso2/oxygen-ui';
import {Lock} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {Trans} from 'react-i18next';

/**
 * Props for {@link SectionLockNotice}.
 *
 * @public
 */
export interface SectionLockNoticeProps {
  /** The section's name, which also names the declarative resource file that sets it. */
  section: string;
}

/**
 * Says that a declarative resource sets a server-config section, so it can't be changed here, and
 * names the file where it is changed instead.
 *
 * @param props - The section a declarative resource sets
 * @returns The rendered notice
 *
 * @public
 */
export default function SectionLockNotice({section}: SectionLockNoticeProps): JSX.Element {
  return (
    <Alert severity="info" icon={<Lock size={20} />}>
      <Trans
        i18nKey="settings:lock.declarative"
        defaults="These values come from a declarative resource, so they can't be changed here. To change them, edit <code>{{file}}</code> and redeploy."
        values={{file: `server_configs/${section}.yaml`}}
        components={{
          code: (
            <Box
              component="code"
              sx={{fontFamily: 'monospace', bgcolor: 'action.selected', borderRadius: 0.5, px: 0.5}}
            />
          ),
        }}
      />
    </Alert>
  );
}
