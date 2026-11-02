// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ResourceAvatar, getInitials} from '@thunderid/components';
import {Box, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import useActorName from '../hooks/useActorName';
import type {Actor} from '../models/deploy';

export interface ActorNameProps {
  actor: Actor;
  /** Only the avatar, with the name a hover away. */
  avatarOnly?: boolean;
}

/** The avatar a user is listed with elsewhere in the console: two letters on a circle. */
const AVATAR_PREFIX = 'avatar:shape=circle,variant=two_letter,colors=0,content=';

/** A user as the avatar the users list shows them with, beside their name. */
export default function ActorName({actor, avatarOnly = false}: ActorNameProps): JSX.Element {
  const name = useActorName(actor);
  const avatar = (
    <Box component="span" sx={{display: 'inline-flex'}} aria-label={avatarOnly ? name : undefined}>
      <ResourceAvatar size={18} fallback={`${AVATAR_PREFIX}${getInitials(name)}`} aria-hidden={!avatarOnly} />
    </Box>
  );
  if (avatarOnly) return <Tooltip title={name}>{avatar}</Tooltip>;
  return (
    <Stack direction="row" alignItems="center" spacing={0.75} sx={{minWidth: 0}} data-testid="actor-name">
      {avatar}
      <Typography variant="caption" color="text.secondary" noWrap>
        {name}
      </Typography>
    </Stack>
  );
}
