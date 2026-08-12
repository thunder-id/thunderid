// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getInitials, ResourceAvatar} from '@thunderid/components';
import {UserConstants, useGetUsers} from '@thunderid/configure-users';
import {Box, Skeleton, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {UsersRound} from '@wso2/oxygen-ui-icons-react';
import {motion} from 'framer-motion';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import HomeNextStepCard from './HomeNextStepCard';
import RouteConfig from '../../../../configs/RouteConfig';

const AVATAR_LIMIT = 7;

const avatarVariants = {
  hidden: {opacity: 0, scale: 0.6},
  visible: {opacity: 1, scale: 1, transition: {duration: 0.25}},
};

interface MembersPreviewProps {
  isLoading: boolean;
  isEmpty: boolean;
  users: {id: string; display?: string; attributes?: Record<string, unknown>}[];
  extraCount: number;
  emptyLabel: string;
}

function MembersPreview({isLoading, isEmpty, users, extraCount, emptyLabel}: MembersPreviewProps) {
  if (isLoading) {
    return (
      <Stack direction="row" spacing={0.5}>
        {[0, 1, 2].map((i) => (
          <Skeleton key={i} variant="circular" width={32} height={32} />
        ))}
      </Stack>
    );
  }

  if (isEmpty) {
    return (
      <Typography variant="caption" color="text.disabled">
        {emptyLabel}
      </Typography>
    );
  }

  return (
    <Box sx={{display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 0.5}}>
      <Box
        sx={{
          display: 'flex',
          ...(extraCount > 0 && {'&:hover .member-avatar-extra': {marginLeft: '4px'}}),
        }}
      >
        <Stack component={motion.div} variants={{visible: {transition: {staggerChildren: 0.06}}}} direction="row">
          {users.map((user, index) => {
            const picture = user.attributes?.picture;

            return (
              <motion.div key={user.id} variants={avatarVariants}>
                <Tooltip title={user.display} arrow placement="top">
                  <Box
                    className="member-avatar"
                    sx={{
                      position: 'relative',
                      zIndex: users.length - index,
                      marginLeft: index === 0 ? 0 : '-10px',
                      transition: 'margin-left 0.2s ease, transform 0.2s ease',
                      '&:hover': {
                        transform: 'translateY(-4px)',
                        zIndex: users.length + 1,
                      },
                    }}
                  >
                    <ResourceAvatar
                      value={typeof picture === 'string' ? picture : undefined}
                      size={32}
                      fallback={`${UserConstants.DEFAULT_AVATAR_PREFIX}${getInitials(user.display)}`}
                      sx={{border: '2px solid', borderColor: 'background.paper'}}
                    />
                  </Box>
                </Tooltip>
              </motion.div>
            );
          })}
          {extraCount > 0 && (
            <Box
              className="member-avatar-extra"
              sx={{
                position: 'relative',
                zIndex: 0,
                marginLeft: '-10px',
                transition: 'margin-left 0.2s ease',
                width: 32,
                height: 32,
                borderRadius: '50%',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                bgcolor: 'action.selected',
                border: '2px solid',
                borderColor: 'background.paper',
              }}
            >
              <Typography variant="caption" color="text.secondary" sx={{fontWeight: 600, whiteSpace: 'nowrap'}}>
                +{extraCount}
              </Typography>
            </Box>
          )}
        </Stack>
      </Box>
    </Box>
  );
}

export default function InviteMembersCard(): JSX.Element {
  const {t} = useTranslation('home');
  const {data, isLoading} = useGetUsers({limit: AVATAR_LIMIT});

  const totalResults = data?.totalResults ?? 0;
  const users = data?.users ?? [];
  const extraCount = totalResults > AVATAR_LIMIT ? totalResults - AVATAR_LIMIT : 0;
  const isEmpty = !isLoading && users.length === 0;

  const preview = (
    <Box sx={{minHeight: 32, display: 'flex', alignItems: 'center'}}>
      <MembersPreview
        isLoading={isLoading}
        isEmpty={isEmpty}
        users={users}
        extraCount={extraCount}
        emptyLabel={t('next_steps.invite_members.status.empty', 'No members yet')}
      />
    </Box>
  );

  return (
    <HomeNextStepCard
      icon={<UsersRound size={24} />}
      title={t('next_steps.invite_members.title', 'Add Users')}
      description={t(
        'next_steps.invite_members.description',
        'Add or invite collaborators to help manage your organization.',
      )}
      primaryLabel={t('next_steps.invite_members.actions.primary.label', 'Add User')}
      primaryRoute={RouteConfig.users.add()}
      preview={preview}
    />
  );
}
