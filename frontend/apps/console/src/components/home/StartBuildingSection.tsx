// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useGetApplications} from '@thunderid/configure-applications';
import {useConfig} from '@thunderid/contexts';
import {alpha, Box, Button, Card, Stack, Typography, useMediaQuery, useTheme} from '@wso2/oxygen-ui';
import {ArrowRight} from '@wso2/oxygen-ui-icons-react';
import {motion} from 'framer-motion';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import FrameworkFlipCard from './FrameworkFlipCard';
import RouteConfig from '../../configs/RouteConfig';

export default function StartBuildingSection(): JSX.Element {
  const navigate = useNavigate();
  const theme = useTheme();
  const {t} = useTranslation('home');
  const {data} = useGetApplications({limit: 1});
  const {config} = useConfig();

  const showFrameworks = useMediaQuery(theme.breakpoints.up('lg'));
  const showAllFrameworkSlots = useMediaQuery(theme.breakpoints.up('xl'));

  const {brand} = config;
  const {product_name: productName} = brand || {};
  const totalApps = data?.totalResults ?? 0;
  const hasApps = totalApps > 0;

  const goToApplicationList = () => {
    navigate(RouteConfig.applications.list())?.catch(() => undefined);
  };

  return (
    <Box
      component={motion.div}
      initial={{opacity: 0, y: 20}}
      animate={{opacity: 1, y: 0}}
      transition={{duration: 0.4, ease: 'easeOut'}}
    >
      <Card
        variant="outlined"
        sx={(theme) => ({
          position: 'relative',
          overflow: 'hidden',
          minHeight: 180,
          p: 4,
          borderColor: alpha(theme.palette.primary.main, 0.22),
          background: `linear-gradient(135deg, ${alpha(theme.palette.primary.main, 0.1)}, ${alpha(theme.palette.primary.main, 0.02)} 60%)`,
        })}
      >
        <Box
          sx={(theme) => ({
            position: 'absolute',
            top: -120,
            right: -40,
            width: 420,
            height: 420,
            borderRadius: '50%',
            background: `radial-gradient(circle, ${alpha(theme.palette.primary.main, 0.16)}, transparent 75%)`,
            filter: 'blur(20px)',
            pointerEvents: 'none',
          })}
        />

        <Box
          sx={{
            position: 'relative',
            zIndex: 1,
            display: 'flex',
            flexDirection: {xs: 'column', lg: 'row'},
            alignItems: {xs: 'flex-start', lg: 'center'},
            gap: {xs: 3, lg: 4},
          }}
        >
          <Box sx={{flex: 1, minWidth: 0}}>
            <Stack spacing={2}>
              <Typography
                variant="overline"
                color="primary.light"
                sx={{fontSize: '0.6875rem', fontWeight: 700, letterSpacing: '0.08em'}}
              >
                {t('start_building.hero.eyebrow', 'Get Started')}
              </Typography>
              <Typography variant="h3" fontWeight={600}>
                {t('start_building.hero.title', {
                  product: productName,
                  defaultValue: 'Integrate {{product}} into your application',
                })}
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{maxWidth: 520}}>
                {t(
                  'start_building.hero.description',
                  'Add secure login, token management, and user sessions to your app in minutes.',
                )}
              </Typography>
              <Box sx={{display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 1.5}}>
                {hasApps ? (
                  <>
                    <Button
                      variant="contained"
                      size="small"
                      onClick={() => {
                        navigate(RouteConfig.applications.types())?.catch(() => undefined);
                      }}
                      sx={{textTransform: 'none', whiteSpace: 'nowrap'}}
                    >
                      {t('start_building.hero.actions.view_apps.label', 'Create Applications')}
                    </Button>
                    <Box
                      onClick={goToApplicationList}
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 0.25,
                        color: 'primary.light',
                        cursor: 'pointer',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      <Typography variant="body2" color="inherit">
                        {t('start_building.hero.status.app_count', {
                          count: totalApps,
                          defaultValue: '{{count}} application',
                        })}
                      </Typography>
                      <ArrowRight size={16} />
                    </Box>
                  </>
                ) : (
                  <Button
                    variant="contained"
                    size="small"
                    onClick={() => {
                      navigate(RouteConfig.applications.types())?.catch(() => undefined);
                    }}
                    sx={{textTransform: 'none'}}
                  >
                    {t('start_building.hero.actions.create.label', 'Create Application')}
                  </Button>
                )}
              </Box>
            </Stack>
          </Box>

          {showFrameworks && (
            <Box
              sx={{
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'flex-start',
                gap: 1.25,
                flexShrink: 0,
                pl: 4,
                borderLeft: '1px solid',
                borderColor: 'divider',
              }}
            >
              <Typography
                variant="overline"
                sx={{fontSize: '0.625rem', fontWeight: 600, letterSpacing: '0.06em', color: 'text.disabled'}}
              >
                {t('start_building.frameworks.label', 'Start with a Template')}
              </Typography>
              <FrameworkFlipCard slotCount={showAllFrameworkSlots ? 3 : 1} />
            </Box>
          )}
        </Box>
      </Card>
    </Box>
  );
}
