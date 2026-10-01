// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Stack} from '@wso2/oxygen-ui';
import {Palette} from '@wso2/oxygen-ui-icons-react';
import {motion} from 'framer-motion';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import HomeNextStepCard from './HomeNextStepCard';

const SWATCH_COLORS = ['#FF6B00', '#6366F1', '#0EA5E9', '#10B981'];

export default function LoginBoxCard(): JSX.Element {
  const {t} = useTranslation('home');

  const preview = (
    <Stack direction="row" spacing={0.75} alignItems="center">
      {SWATCH_COLORS.map((color, i) => (
        <motion.div
          key={color}
          initial={{opacity: 0, scaleX: 0}}
          animate={{opacity: 1, scaleX: 1}}
          transition={{duration: 0.35, delay: i * 0.07, ease: 'easeOut'}}
          style={{transformOrigin: 'left'}}
        >
          <Box
            sx={{
              width: 28,
              height: 12,
              borderRadius: 1,
              bgcolor: color,
            }}
          />
        </motion.div>
      ))}
    </Stack>
  );

  return (
    <HomeNextStepCard
      icon={<Palette size={24} />}
      title={t('next_steps.login_box.title', 'Sign-in Box')}
      description={t(
        'next_steps.login_box.description',
        'Build themes and attach them to your applications to personalise the sign-in experience.',
      )}
      primaryLabel={t('next_steps.login_box.actions.primary.label', 'Open Design Studio')}
      primaryRoute="/design"
      preview={preview}
    />
  );
}
