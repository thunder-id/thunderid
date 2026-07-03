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

import {Box, Grid, Typography} from '@wso2/oxygen-ui';
import {motion} from 'framer-motion';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ConnectionsCard from './cards/ConnectionsCard';
import InviteMembersCard from './cards/InviteMembersCard';
import LoginBoxCard from './cards/LoginBoxCard';
import MFACard from './cards/MFACard';

const MotionBox = motion.create(Box);

const containerVariants = {
  hidden: {},
  visible: {
    transition: {
      staggerChildren: 0.08,
    },
  },
};

const cardVariants = {
  hidden: {opacity: 0, y: 16},
  visible: {opacity: 1, y: 0, transition: {duration: 0.3, ease: 'easeOut' as const}},
};

export default function NextStepsSection(): JSX.Element {
  const {t} = useTranslation('home');

  return (
    <Box>
      <Typography variant="h6" sx={{mb: 2, fontWeight: 600}}>
        {t('next_steps.section.title', 'Quick Links')}
      </Typography>
      <Grid
        container
        spacing={2}
        component={motion.div}
        variants={containerVariants}
        initial="hidden"
        animate="visible"
      >
        <Grid size={{xs: 12, sm: 6}}>
          <MotionBox variants={cardVariants} sx={{height: '100%'}}>
            <InviteMembersCard />
          </MotionBox>
        </Grid>
        <Grid size={{xs: 12, sm: 6}}>
          <MotionBox variants={cardVariants} sx={{height: '100%'}}>
            <LoginBoxCard />
          </MotionBox>
        </Grid>
        <Grid size={{xs: 12, sm: 6}}>
          <MotionBox variants={cardVariants} sx={{height: '100%'}}>
            <ConnectionsCard />
          </MotionBox>
        </Grid>
        <Grid size={{xs: 12, sm: 6}}>
          <MotionBox variants={cardVariants} sx={{height: '100%'}}>
            <MFACard />
          </MotionBox>
        </Grid>
      </Grid>
    </Box>
  );
}
