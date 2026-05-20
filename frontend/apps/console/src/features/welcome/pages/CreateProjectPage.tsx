/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {useConfig} from '@thunderid/contexts';
import {
  Box,
  Button,
  Stack,
  Typography,
  Card,
  IconButton,
  LinearProgress,
  Breadcrumbs,
  ColorSchemeSVG,
} from '@wso2/oxygen-ui';
import {ChevronRight, X, Settings, PlayCircle, CheckCircle} from '@wso2/oxygen-ui-icons-react';
import {motion} from 'framer-motion';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import HowSolutionWorksIllustration from '../components/HowSolutionWorksIllustration';

const MotionBox = motion.create(Box);

export default function CreateProjectPage(): JSX.Element {
  const {t} = useTranslation(['common']);
  const navigate = useNavigate();
  const {config} = useConfig();
  const productName = config.brand.product_name;

  const handleContinue = (): void => {
    void navigate('/home');
  };

  const handleClose = (): void => {
    void navigate('/home');
  };

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      {/* Progress bar at the very top */}
      <LinearProgress variant="determinate" value={0} sx={{height: 6}} />

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'column'}}>
        {/* Header with close button and breadcrumb */}
        <Box
          sx={{
            position: 'sticky',
            top: 0,
            zIndex: 10,
            p: 4,
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <Stack direction="row" spacing={2} sx={{alignItems: 'center'}}>
            <IconButton
              aria-label={t('common:actions.close')}
              onClick={handleClose}
              sx={{
                bgcolor: 'background.paper',
                '&:hover': {bgcolor: 'action.hover'},
                boxShadow: 1,
              }}
            >
              <X size={24} />
            </IconButton>
            <Breadcrumbs separator={<ChevronRight size={16} />} aria-label="breadcrumb">
              <Typography
                variant="h5"
                color="inherit"
                role="button"
                tabIndex={0}
                onClick={() => {
                  void navigate('/welcome');
                }}
                onKeyDown={(e: React.KeyboardEvent) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    void navigate('/welcome');
                  }
                }}
                sx={{cursor: 'pointer', '&:hover': {textDecoration: 'underline'}}}
              >
                {t('common:welcome.header')}
              </Typography>
              <Typography variant="h5" color="text.primary">
                {t('common:welcome.createProject.breadcrumb')}
              </Typography>
            </Breadcrumbs>
          </Stack>
        </Box>

        {/* Main Content */}
        <Box
          sx={{
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'center',
            alignItems: 'center',
            px: {xs: 2, md: 4},
            pb: 8,
          }}
        >
          <MotionBox
            initial={{opacity: 0, y: 20}}
            animate={{opacity: 1, y: 0}}
            transition={{duration: 0.5}}
            sx={{
              maxWidth: '900px',
              width: '100%',
            }}
          >
            {/* Title */}
            <Box sx={{textAlign: 'center', mb: 6}}>
              <Typography
                variant="h1"
                sx={{
                  fontSize: {xs: '1.75rem', sm: '2rem', md: '2.5rem'},
                  fontWeight: 600,
                  mb: 2,
                }}
              >
                {t('common:welcome.createProject.title')}
              </Typography>

              <Typography
                variant="body1"
                color="text.secondary"
                sx={{
                  fontSize: {xs: '1rem', sm: '1.125rem'},
                  maxWidth: '600px',
                  mx: 'auto',
                }}
              >
                {t('common:welcome.createProject.subtitle', {productName})}
              </Typography>
            </Box>

            {/* Illustration */}
            <MotionBox
              initial={{opacity: 0, scale: 0.95}}
              animate={{opacity: 1, scale: 1}}
              transition={{duration: 0.5, delay: 0.2}}
              sx={{
                my: 8,
                display: 'flex',
                justifyContent: 'center',
                overflow: 'auto',
              }}
            >
              <ColorSchemeSVG
                svg={HowSolutionWorksIllustration}
                sx={{
                  width: '100%',
                  minWidth: {xs: '280px', sm: 'auto'},
                  maxWidth: {xs: '100%', sm: '600px', md: '800px', lg: '1000px'},
                  height: 'auto',
                }}
              />
            </MotionBox>

            {/* Information Grid */}
            <Box
              sx={{
                display: 'grid',
                gridTemplateColumns: {xs: '1fr', md: 'repeat(3, 1fr)'},
                gap: 3,
                mb: 6,
              }}
            >
              <MotionBox
                initial={{opacity: 0, y: 20}}
                animate={{opacity: 1, y: 0}}
                transition={{duration: 0.4, delay: 0.3}}
              >
                <Card sx={{p: 3, height: '100%', border: '1px solid', borderColor: 'divider'}}>
                  <Stack spacing={1}>
                    <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                      <Settings size={20} />
                      <Typography variant="h3" sx={{fontSize: '1.125rem', fontWeight: 600}}>
                        {t('common:welcome.createProject.cards.configure.title')}
                      </Typography>
                    </Stack>
                    <Typography variant="body2" color="text.secondary">
                      {t('common:welcome.createProject.cards.configure.description')}
                    </Typography>
                  </Stack>
                </Card>
              </MotionBox>

              <MotionBox
                initial={{opacity: 0, y: 20}}
                animate={{opacity: 1, y: 0}}
                transition={{duration: 0.4, delay: 0.4}}
              >
                <Card sx={{p: 3, height: '100%', border: '1px solid', borderColor: 'divider'}}>
                  <Stack spacing={1}>
                    <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                      <CheckCircle size={20} />
                      <Typography variant="h3" sx={{fontSize: '1.125rem', fontWeight: 600}}>
                        {t('common:welcome.createProject.cards.verify.title')}
                      </Typography>
                    </Stack>
                    <Typography variant="body2" color="text.secondary">
                      {t('common:welcome.createProject.cards.verify.description')}
                    </Typography>
                  </Stack>
                </Card>
              </MotionBox>

              <MotionBox
                initial={{opacity: 0, y: 20}}
                animate={{opacity: 1, y: 0}}
                transition={{duration: 0.4, delay: 0.5}}
              >
                <Card sx={{p: 3, height: '100%', border: '1px solid', borderColor: 'divider'}}>
                  <Stack spacing={1}>
                    <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                      <PlayCircle size={20} />
                      <Typography variant="h3" sx={{fontSize: '1.125rem', fontWeight: 600}}>
                        {t('common:welcome.createProject.cards.runServer.title')}
                      </Typography>
                    </Stack>
                    <Typography variant="body2" color="text.secondary">
                      {t('common:welcome.createProject.cards.runServer.description', {productName})}
                    </Typography>
                  </Stack>
                </Card>
              </MotionBox>
            </Box>

            {/* Action Button */}
            <Box sx={{display: 'flex', justifyContent: 'center'}}>
              <MotionBox
                initial={{opacity: 0, y: 20}}
                animate={{opacity: 1, y: 0}}
                transition={{duration: 0.4, delay: 0.6}}
              >
                <Button
                  variant="contained"
                  endIcon={<ChevronRight size={20} />}
                  onClick={handleContinue}
                  sx={{
                    minWidth: 150,
                  }}
                >
                  {t('common:welcome.createProject.actions.getStarted')}
                </Button>
              </MotionBox>
            </Box>
          </MotionBox>
        </Box>
      </Box>
    </Box>
  );
}
