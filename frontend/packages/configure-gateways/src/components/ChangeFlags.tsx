// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Chip} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {Change} from '../models/deploy';

export interface ChangeFlagsProps {
  change: Change;
  /** Where the changes are applied, to say a change the target refused was refused there. */
  target?: string;
  /** Where the changes come from, to say a change held back there is held back on it. */
  source?: string;
  /** The name of the selected change that brings this one along, when one does. */
  neededBy?: string;
}

const QUIET = {height: 18, fontSize: 11, bgcolor: 'action.selected', color: 'text.secondary'};

/**
 * What a change carries beyond its kind: held back where it comes from, refused by the target last time,
 * left out by an earlier deploy, sent for a changed value, or brought along for another's need.
 */
export default function ChangeFlags({
  change,
  target = undefined,
  source = undefined,
  neededBy = undefined,
}: ChangeFlagsProps): JSX.Element {
  const {t} = useTranslation('deploy');
  return (
    <>
      {change.sourceExcluded && (
        <Chip
          size="small"
          label={
            source
              ? t('changes.heldBackOn', 'Held back on {{target}}', {target: source})
              : t('changes.heldBack', 'Held back')
          }
          data-testid="change-source-held-back"
          title={t('changes.sourceHeldBackHint', '{{source}} does not run this change, so it cannot be promoted.', {
            source: source ?? '',
          })}
          sx={QUIET}
        />
      )}
      {change.failed && (
        <Chip
          size="small"
          color="error"
          variant="outlined"
          label={
            target
              ? t('changes.refusedOn', 'Refused by {{target}} last time', {target})
              : t('changes.refused', 'Refused last time')
          }
          data-testid="change-refused"
          sx={{height: 18, fontSize: 11}}
        />
      )}
      {change.heldBack && (
        <Chip
          size="small"
          label={t('changes.heldBackHere', 'Left out before')}
          data-testid="change-held-back"
          title={t(
            'changes.heldBackHereHint',
            'An earlier deploy left this out, so it stays out until you tick it. Ticking it deploys it and stops leaving it out.',
          )}
          sx={QUIET}
        />
      )}
      {change.valueChanged && (
        <Chip
          size="small"
          label={t('changes.valueChanged', 'Value changed')}
          data-testid="change-value-changed"
          title={t('changes.valueChangedHint', 'A value it refers to was set again, so it is written to read it.')}
          sx={QUIET}
        />
      )}
      {neededBy !== undefined && (
        <Chip
          size="small"
          label={t('changes.neededBy', 'Needed by {{name}}', {name: neededBy})}
          data-testid="change-needed-by"
          sx={QUIET}
        />
      )}
    </>
  );
}
