// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {zodResolver} from '@hookform/resolvers/zod';
import {Box, Button, FormControl, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {Controller, useForm} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import {z} from 'zod';
import useCreateEnvironmentVariable from '../api/useCreateEnvironmentVariable';

const formSchema = z.object({
  key: z
    .string()
    .trim()
    .min(1)
    .regex(/^[A-Za-z_][A-Za-z0-9_]*$/),
  value: z.string().min(1),
  description: z.string().trim().optional(),
});
type FormData = z.infer<typeof formSchema>;

/**
 * Full-screen page for creating an environment variable.
 */
export default function CreateEnvironmentVariablePage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {envId = ''} = useParams<{envId: string}>();
  const createEnvironmentVariable = useCreateEnvironmentVariable(envId);

  const {
    control,
    handleSubmit,
    formState: {isValid},
  } = useForm<FormData>({
    resolver: zodResolver(formSchema),
    mode: 'onChange',
    defaultValues: {key: '', value: '', description: ''},
  });

  const onSubmit = (formData: FormData): void => {
    createEnvironmentVariable.mutate(
      {key: formData.key, value: formData.value, description: formData.description},
      {
        onSuccess: () => {
          void navigate(`/promotions/${envId}/variables`);
        },
      },
    );
  };

  return (
    <Box sx={{maxWidth: 640, mx: 'auto', width: '100%', py: 6, px: 3}}>
      <Typography variant="h5" gutterBottom>
        {t('environmentVariables:create.title', 'Add Environment Variable')}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
        {t(
          'environmentVariables:create.subtitle',
          'The key is the declarative placeholder this value resolves when configuration is applied.',
        )}
      </Typography>

      <Stack spacing={3}>
        <Controller
          name="key"
          control={control}
          render={({field, fieldState}) => (
            <FormControl fullWidth>
              <FormLabel>{t('environmentVariables:form.key.label', 'Key')}</FormLabel>
              <TextField
                {...field}
                placeholder="CONSOLE_REDIRECT_URIS"
                error={Boolean(fieldState.error)}
                helperText={
                  fieldState.error
                    ? t(
                        'environmentVariables:form.key.invalid',
                        'Use letters, digits and underscores; it must not start with a digit.',
                      )
                    : t(
                        'environmentVariables:form.key.help',
                        'The declarative placeholder name this variable resolves.',
                      )
                }
                fullWidth
              />
            </FormControl>
          )}
        />

        <Controller
          name="value"
          control={control}
          render={({field, fieldState}) => (
            <FormControl fullWidth>
              <FormLabel>{t('environmentVariables:form.value.label', 'Value')}</FormLabel>
              <TextField
                {...field}
                error={Boolean(fieldState.error)}
                helperText={t(
                  'environmentVariables:form.value.help',
                  'For a list, use a JSON array such as ["https://app.example.com/callback"].',
                )}
                fullWidth
              />
            </FormControl>
          )}
        />

        <Controller
          name="description"
          control={control}
          render={({field}) => (
            <FormControl fullWidth>
              <FormLabel>{t('environmentVariables:form.description.label', 'Description')}</FormLabel>
              <TextField {...field} fullWidth />
            </FormControl>
          )}
        />

        <Stack direction="row" spacing={2} justifyContent="flex-end">
          <Button
            onClick={() => {
              void navigate(`/promotions/${envId}/variables`);
            }}
          >
            {t('common:actions.cancel', 'Cancel')}
          </Button>
          <Button
            variant="contained"
            onClick={() => {
              void handleSubmit(onSubmit)();
            }}
            disabled={!isValid || createEnvironmentVariable.isPending}
          >
            {createEnvironmentVariable.isPending
              ? t('common:status.saving', 'Saving...')
              : t('environmentVariables:create.submit', 'Create Variable')}
          </Button>
        </Stack>
      </Stack>
    </Box>
  );
}
