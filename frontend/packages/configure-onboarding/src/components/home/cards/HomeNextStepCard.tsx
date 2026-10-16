// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, Card, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import {ArrowRight} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';

export type FeatureStatus = 'new' | 'coming_soon';

const FEATURE_STATUS_STYLES: Record<FeatureStatus, {bgcolor: string; color: string}> = {
  new: {bgcolor: 'success.main', color: 'success.contrastText'},
  coming_soon: {bgcolor: 'warning.main', color: 'warning.contrastText'},
};

export interface HomeNextStepCardProps {
  icon: ReactNode;
  title: string;
  description: string;
  primaryLabel?: string;
  primaryRoute?: string;
  secondaryLabel?: string;
  secondaryRoute?: string;
  secondaryHref?: string;
  /** Feature status badge rendered at the top-right of the card */
  featureStatus?: FeatureStatus;
  /** Slot for dynamic preview content rendered between description and actions */
  preview?: ReactNode;
  /** Additional sx props merged into the root Card */
  cardSx?: object;
}

export default function HomeNextStepCard({
  icon,
  title,
  description,
  primaryLabel = undefined,
  primaryRoute = undefined,
  secondaryLabel = undefined,
  secondaryRoute = undefined,
  secondaryHref = undefined,
  featureStatus = undefined,
  preview = undefined,
  cardSx = undefined,
}: HomeNextStepCardProps): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation('home');

  const statusStyles = featureStatus ? FEATURE_STATUS_STYLES[featureStatus] : null;
  const statusLabel = featureStatus
    ? t(`feature_status.${featureStatus}`, featureStatus === 'new' ? 'New' : 'Coming Soon')
    : null;

  return (
    <Card
      variant="outlined"
      sx={{
        p: 3,
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        gap: 1.5,
        position: 'relative',
        ...cardSx,
      }}
    >
      {statusLabel && statusStyles && (
        <Chip
          label={statusLabel}
          size="small"
          sx={{
            position: 'absolute',
            top: 12,
            right: 12,
            height: 22,
            fontSize: '0.68rem',
            fontWeight: 700,
            letterSpacing: '0.03em',
            border: 'none',
            ...statusStyles,
          }}
        />
      )}
      <Box
        sx={{
          width: 40,
          height: 40,
          borderRadius: 1,
          bgcolor: 'action.hover',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: 'text.secondary',
        }}
      >
        {icon}
      </Box>

      <Typography variant="subtitle1" fontWeight={600}>
        {title}
      </Typography>

      <Typography variant="body2" color="text.secondary" sx={{flex: 1}}>
        {description}
      </Typography>

      {preview && <Box>{preview}</Box>}

      {(primaryLabel ?? secondaryLabel) && (
        <Stack direction="row" spacing={1} alignItems="center">
          {primaryLabel && primaryRoute && (
            <Button
              variant="text"
              size="small"
              endIcon={<ArrowRight size={14} />}
              onClick={() => {
                navigate(primaryRoute)?.catch(() => undefined);
              }}
              sx={{textTransform: 'none', whiteSpace: 'nowrap'}}
            >
              {primaryLabel}
            </Button>
          )}
          {secondaryLabel &&
            (secondaryRoute ? (
              <Button
                variant="text"
                size="small"
                onClick={() => {
                  navigate(secondaryRoute)?.catch(() => undefined);
                }}
                sx={{textTransform: 'none'}}
              >
                {secondaryLabel}
              </Button>
            ) : (
              <Button
                variant="text"
                size="small"
                component="a"
                href={secondaryHref ?? '#'}
                sx={{textTransform: 'none'}}
              >
                {secondaryLabel}
              </Button>
            ))}
        </Stack>
      )}
    </Card>
  );
}
