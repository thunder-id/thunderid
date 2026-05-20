/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {getInitials} from '@thunderid/components';
import {useGetUsers} from '@thunderid/configure-users';
import {Avatar, Box, Skeleton, Stack, Typography} from '@wso2/oxygen-ui';
import {UsersRound} from '@wso2/oxygen-ui-icons-react';
import {motion} from 'framer-motion';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import HomeNextStepCard from './HomeNextStepCard';

const AVATAR_LIMIT = 5;

const avatarVariants = {
  hidden: {opacity: 0, scale: 0.6},
  visible: {opacity: 1, scale: 1, transition: {duration: 0.25}},
};

interface MembersPreviewProps {
  isLoading: boolean;
  isEmpty: boolean;
  users: {id: string; display?: string}[];
  extraCount: number;
  emptyLabel: string;
  countLabel: string;
}

function MembersPreview({isLoading, isEmpty, users, extraCount, emptyLabel, countLabel}: MembersPreviewProps) {
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
    <Stack direction="row" spacing={0.5} alignItems="center">
      <Stack
        component={motion.div}
        variants={{visible: {transition: {staggerChildren: 0.06}}}}
        direction="row"
        spacing={0.5}
      >
        {users.map((user) => (
          <motion.div key={user.id} variants={avatarVariants}>
            <Avatar
              sx={{
                width: 32,
                height: 32,
                fontSize: '0.7rem',
                bgcolor: 'primary.light',
                color: 'primary.contrastText',
              }}
            >
              {getInitials(user.display)}
            </Avatar>
          </motion.div>
        ))}
      </Stack>
      {extraCount > 0 && (
        <Typography variant="caption" color="text.secondary" sx={{ml: 0.5}}>
          +{extraCount}
        </Typography>
      )}
      <Typography variant="caption" color="text.secondary" sx={{ml: 1}}>
        {countLabel}
      </Typography>
    </Stack>
  );
}

export default function InviteMembersCard(): JSX.Element {
  const {t} = useTranslation('home');
  const {data, isLoading} = useGetUsers({limit: AVATAR_LIMIT});

  const totalResults = data?.totalResults ?? 0;
  const users = data?.users ?? [];
  const extraCount = totalResults > AVATAR_LIMIT ? totalResults - AVATAR_LIMIT : 0;
  // totalResults <= 1 means only the admin — treat as empty
  const isEmpty = !isLoading && totalResults <= 1;

  const preview = (
    <Box sx={{minHeight: 32, display: 'flex', alignItems: 'center'}}>
      <MembersPreview
        isLoading={isLoading}
        isEmpty={isEmpty}
        users={users}
        extraCount={extraCount}
        emptyLabel={t('next_steps.invite_members.status.empty', 'No members yet — add collaborators')}
        countLabel={t('next_steps.invite_members.status.count', {
          count: totalResults,
          defaultValue: '{{count}} member',
        })}
      />
    </Box>
  );

  return (
    <HomeNextStepCard
      icon={<UsersRound size={24} />}
      title={t('next_steps.invite_members.title', 'Invite Members')}
      description={t(
        'next_steps.invite_members.description',
        'Add collaborators to help manage your organization and act as a backup.',
      )}
      primaryLabel={t('next_steps.invite_members.actions.primary.label', 'Add User')}
      primaryRoute="/users/invite"
      secondaryLabel={t('next_steps.invite_members.actions.secondary.label', 'Invite User')}
      secondaryRoute="/users?invite=true"
      preview={preview}
    />
  );
}
