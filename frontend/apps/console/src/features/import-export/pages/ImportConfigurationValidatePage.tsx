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

import {useLogger} from '@thunderid/logger/react';
import {Box, Breadcrumbs, Button, IconButton, LinearProgress, Stack, Typography, Alert, Chip} from '@wso2/oxygen-ui';
import {CheckCircle, ChevronRight, X, AlertCircle} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useLocation, useNavigate} from 'react-router';
import type {ValidationStep, ParseError} from '../models/import-configuration';

export default function ImportConfigurationValidatePage(): JSX.Element {
  const {t} = useTranslation('importExport');
  const navigate = useNavigate();
  const location = useLocation();
  const logger = useLogger('ImportConfigurationValidatePage');

  const state = location.state as {
    parseErrors?: ParseError[];
    parseStats?: {successCount: number; failCount: number};
    configData?: unknown;
  } | null;

  const hasParseErrors = state?.parseErrors && state.parseErrors.length > 0;

  const [validationSteps, setValidationSteps] = useState<ValidationStep[]>([
    {id: 'file', label: t('validate.steps.readingFile'), status: 'pending'},
    {id: 'schema', label: t('validate.steps.validatingYaml'), status: 'pending'},
    {id: 'compatibility', label: t('validate.steps.checkingCompatibility'), status: 'pending'},
    {id: 'resources', label: t('validate.steps.validatingResources'), status: 'pending'},
  ]);

  const handleClose = (): void => {
    void navigate('/home');
  };

  const handleCancel = (): void => {
    void navigate('/welcome');
  };

  useEffect(() => {
    let currentStep = 0;

    // File reading step always completes
    setValidationSteps((prev) => prev.map((step, index) => (index === 0 ? {...step, status: 'completed'} : step)));

    const interval = setInterval(() => {
      if (currentStep < validationSteps.length - 1) {
        currentStep++;

        setValidationSteps((prev) =>
          prev.map((step, index) => {
            if (index === currentStep) {
              return {...step, status: 'validating'};
            }
            if (index < currentStep) {
              return {...step, status: 'completed'};
            }
            return step;
          }),
        );

        setTimeout(() => {
          setValidationSteps((prev) =>
            prev.map((step, index) => {
              if (index === currentStep) {
                // Check if this is the schema validation step and we have parse errors
                if (step.id === 'schema' && hasParseErrors) {
                  return {...step, status: 'failed'};
                }
                return {...step, status: 'completed'};
              }
              return step;
            }),
          );

          // If schema validation failed, stop here
          if (currentStep === 1 && hasParseErrors) {
            clearInterval(interval);
            return;
          }

          if (currentStep === validationSteps.length - 1) {
            clearInterval(interval);
            // Navigate to summary after all steps complete (only if no errors)
            setTimeout(() => {
              (async () => {
                await navigate('/welcome/open-project/summary', {
                  state: location.state as Record<string, unknown>,
                });
              })().catch((_error: unknown) => {
                logger.error('Failed to navigate to summary', {error: _error});
              });
            }, 500);
          }
        }, 1000);
      }
    }, 1500);

    return () => clearInterval(interval);
  }, [navigate, location.state, logger, validationSteps.length, hasParseErrors]);

  const completedSteps = validationSteps.filter((step) => step.status === 'completed').length;
  const progress = (completedSteps / validationSteps.length) * 100;

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <LinearProgress variant="determinate" value={66} sx={{height: 6}} />

      <Box
        sx={{
          p: 4,
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          flexShrink: 0,
        }}
      >
        <Stack direction="row" spacing={2} sx={{alignItems: 'center'}}>
          <IconButton
            aria-label={t('common:actions.close')}
            onClick={handleClose}
            sx={{bgcolor: 'background.paper', '&:hover': {bgcolor: 'action.hover'}, boxShadow: 1}}
          >
            <X size={24} />
          </IconButton>
          <Breadcrumbs separator={<ChevronRight size={16} />} aria-label="breadcrumb">
            <Typography
              variant="h5"
              onClick={() => void navigate('/welcome')}
              sx={{cursor: 'pointer', '&:hover': {textDecoration: 'underline'}}}
            >
              {t('common:welcome.header')}
            </Typography>
            <Typography variant="h5" color="text.primary">
              {t('upload.breadcrumb.openProject')}
            </Typography>
          </Breadcrumbs>
        </Stack>
      </Box>

      <Box
        sx={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          py: 8,
          px: {xs: 2, sm: 3, md: 8, lg: 20},
          alignItems: 'flex-start',
        }}
      >
        <Box
          sx={{
            width: '100%',
            maxWidth: 1600,
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          <Stack spacing={1} sx={{mb: 4}}>
            <Typography variant="h2" fontWeight={600}>
              {t('validate.title')}
            </Typography>
            <Typography variant="body1" color="text.secondary">
              {t('validate.subtitle')}
            </Typography>
          </Stack>

          <Box sx={{mb: 4}}>
            <Stack spacing={0.5} sx={{mb: 2}}>
              <Stack direction="row" sx={{justifyContent: 'space-between', alignItems: 'center'}}>
                <Typography variant="body2" color="text.secondary">
                  {t('validate.progress')}
                </Typography>
                <Typography variant="body2" fontWeight={600}>
                  {completedSteps} / {validationSteps.length}
                </Typography>
              </Stack>
              <LinearProgress variant="determinate" value={progress} sx={{height: 8, borderRadius: 1}} />
            </Stack>
          </Box>

          <Stack spacing={2}>
            {validationSteps.map((step) => (
              <Box
                key={step.id}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 2,
                  p: 2.5,
                  border: '1px solid',
                  borderColor: step.status === 'validating' ? 'primary.main' : 'divider',
                  borderRadius: 2,
                  bgcolor: 'background.paper',
                  transition: 'all 0.3s',
                }}
              >
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    width: 40,
                    height: 40,
                    borderRadius: '50%',
                    bgcolor:
                      step.status === 'completed'
                        ? 'success.main'
                        : step.status === 'failed'
                          ? 'error.main'
                          : 'action.hover',
                    flexShrink: 0,
                    color: step.status === 'completed' || step.status === 'failed' ? '#fff' : 'text.secondary',
                  }}
                >
                  {step.status === 'completed' ? (
                    <CheckCircle size={20} />
                  ) : step.status === 'failed' ? (
                    <AlertCircle size={20} />
                  ) : step.status === 'validating' ? (
                    <Box
                      sx={{
                        width: 20,
                        height: 20,
                        border: '2px solid',
                        borderColor: 'primary.main',
                        borderTopColor: 'transparent',
                        borderRadius: '50%',
                        animation: 'spin 1s linear infinite',
                        '@keyframes spin': {
                          '0%': {transform: 'rotate(0deg)'},
                          '100%': {transform: 'rotate(360deg)'},
                        },
                      }}
                    />
                  ) : (
                    <Box
                      sx={{
                        width: 8,
                        height: 8,
                        borderRadius: '50%',
                        bgcolor: 'text.disabled',
                      }}
                    />
                  )}
                </Box>
                <Typography
                  variant="body1"
                  fontWeight={step.status === 'validating' ? 600 : 400}
                  color={step.status === 'pending' ? 'text.secondary' : 'text.primary'}
                >
                  {step.label}
                </Typography>
              </Box>
            ))}
          </Stack>

          {hasParseErrors && (
            <Box sx={{mt: 4}}>
              <Alert severity="error" sx={{mb: 2}}>
                <Typography variant="body2" fontWeight={600} gutterBottom>
                  {t('validate.parseErrors.invalidSections', {count: state.parseErrors!.length})}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {state.parseStats &&
                    t('validate.parseErrors.summary', {
                      successCount: state.parseStats.successCount,
                      failCount: state.parseStats.failCount,
                    })}
                </Typography>
              </Alert>

              <Stack spacing={2}>
                <Typography variant="h6" fontWeight={600}>
                  {t('validate.parseErrors.title')}
                </Typography>
                {state.parseErrors!.map((error) => (
                  <Box
                    key={`${error.resourceType}-${error.fileName}-${error.error.substring(0, 20)}`}
                    sx={{
                      p: 2,
                      border: '1px solid',
                      borderColor: 'error.main',
                      borderRadius: 1,
                      bgcolor: 'error.lighter',
                    }}
                  >
                    <Stack spacing={1}>
                      <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                        <Typography variant="body2" fontWeight={600}>
                          {error.fileName ?? t('validate.parseErrors.unknownFile')}
                        </Typography>
                        <Chip label={error.resourceType} size="small" />
                      </Stack>
                      <Typography variant="caption" color="error.dark" sx={{fontFamily: 'monospace'}}>
                        {error.error}
                      </Typography>
                    </Stack>
                  </Box>
                ))}
              </Stack>

              <Stack direction="row" spacing={2} sx={{mt: 4}}>
                <Button variant="outlined" onClick={handleCancel}>
                  {t('common:actions.cancel')}
                </Button>
                <Button variant="outlined" color="error" onClick={() => void navigate('/welcome/open-project')}>
                  {t('validate.actions.uploadDifferentFile')}
                </Button>
              </Stack>
            </Box>
          )}
        </Box>
      </Box>
    </Box>
  );
}
