// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box} from '@wso2/oxygen-ui';

/**
 * `components` map for react-i18next's `<Trans>`. Renders `<code>` mentions of exact
 * configuration keys (e.g. `client_credentials`) in monospace, visually separating them from
 * the surrounding explanatory prose.
 */
export const codeComponents = {
  code: (
    <Box
      component="code"
      sx={{
        fontFamily: 'monospace',
        fontSize: '0.85em',
        color: 'primary.main',
        bgcolor: 'action.selected',
        borderRadius: 0.5,
        px: 0.5,
      }}
    />
  ),
};
