// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
        <Grid size={{xs: 12, md: 6}}>
          <MotionBox variants={cardVariants} sx={{height: '100%'}}>
            <InviteMembersCard />
          </MotionBox>
        </Grid>
        <Grid size={{xs: 12, md: 6}}>
          <MotionBox variants={cardVariants} sx={{height: '100%'}}>
            <LoginBoxCard />
          </MotionBox>
        </Grid>
        <Grid size={{xs: 12, md: 6}}>
          <MotionBox variants={cardVariants} sx={{height: '100%'}}>
            <ConnectionsCard />
          </MotionBox>
        </Grid>
        <Grid size={{xs: 12, md: 6}}>
          <MotionBox variants={cardVariants} sx={{height: '100%'}}>
            <MFACard />
          </MotionBox>
        </Grid>
      </Grid>
    </Box>
  );
}
