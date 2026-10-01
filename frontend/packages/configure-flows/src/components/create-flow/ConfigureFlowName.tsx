// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {zodResolver} from '@hookform/resolvers/zod';
import {NameSuggestion} from '@thunderid/components';
import {FormControl, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useEffect, useRef} from 'react';
import {Controller, useForm} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {z} from 'zod';

const formSchema = z.object({
  name: z.string().trim().min(1),
  handle: z
    .string()
    .trim()
    .min(1)
    .regex(/^[a-z0-9-]+$/),
});

type FormData = z.infer<typeof formSchema>;

export interface ConfigureFlowNameValue {
  name: string;
  handle: string;
}

interface ConfigureFlowNameProps {
  value: ConfigureFlowNameValue;
  onChange: (value: ConfigureFlowNameValue) => void;
  onReadyChange: (ready: boolean) => void;
}

export default function ConfigureFlowName({value, onChange, onReadyChange}: ConfigureFlowNameProps): JSX.Element {
  const {t} = useTranslation();
  const isHandleManuallyEditedRef = useRef(false);

  const generateHandle = (name: string): string =>
    name
      .toLowerCase()
      .replace(/\s+/g, '-')
      .replace(/[^a-z0-9-]/g, '');

  const {
    control,
    setValue,
    formState: {isValid},
  } = useForm<FormData>({
    resolver: zodResolver(formSchema),
    mode: 'onChange',
    defaultValues: {name: value.name, handle: value.handle},
  });

  useEffect(() => {
    onReadyChange(isValid);
  }, [isValid, onReadyChange]);

  const handleNameChange = (newName: string): void => {
    setValue('name', newName, {shouldValidate: true});
    onChange({
      name: newName,
      handle: isHandleManuallyEditedRef.current ? value.handle : generateHandle(newName),
    });
    if (!isHandleManuallyEditedRef.current) {
      setValue('handle', generateHandle(newName), {shouldValidate: true});
    }
  };

  const handleHandleChange = (newHandle: string): void => {
    isHandleManuallyEditedRef.current = true;
    setValue('handle', newHandle, {shouldValidate: true});
    onChange({name: value.name, handle: newHandle});
  };

  const handleSuggestionSelect = (suggestion: string): void => {
    setValue('name', suggestion, {shouldValidate: true});
    onChange({
      name: suggestion,
      handle: isHandleManuallyEditedRef.current ? value.handle : generateHandle(suggestion),
    });
    if (!isHandleManuallyEditedRef.current) {
      setValue('handle', generateHandle(suggestion), {shouldValidate: true});
    }
  };

  return (
    <Stack direction="column" spacing={4} data-testid="configure-flow-name">
      <Typography variant="h1" gutterBottom>
        {t('flows:create.configure.title', "Let's collect some details about your flow")}
      </Typography>

      <FormControl fullWidth required>
        <FormLabel htmlFor="flow-name-input">{t('flows:create.configure.name.label', 'Flow name')}</FormLabel>
        <Controller
          name="name"
          control={control}
          render={({field, fieldState}) => (
            <TextField
              {...field}
              fullWidth
              id="flow-name-input"
              onChange={(e) => handleNameChange(e.target.value)}
              placeholder={t('flows:create.configure.name.placeholder', 'e.g. Customer Sign-in')}
              error={!!fieldState.error}
            />
          )}
        />

        <NameSuggestion onSelect={handleSuggestionSelect} />
      </FormControl>

      <FormControl fullWidth required>
        <FormLabel htmlFor="flow-handle-input">{t('flows:create.configure.handle.label', 'Handle')}</FormLabel>
        <Controller
          name="handle"
          control={control}
          render={({field, fieldState}) => (
            <TextField
              {...field}
              fullWidth
              id="flow-handle-input"
              onChange={(e) => handleHandleChange(e.target.value)}
              placeholder={t('flows:create.configure.handle.placeholder', 'e.g. customer-sign-in')}
              error={!!fieldState.error}
              helperText={
                fieldState.error?.message ??
                t('flows:create.configure.handle.hint', 'Lowercase letters, numbers, and hyphens only')
              }
            />
          )}
        />
      </FormControl>
    </Stack>
  );
}
