// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Box, Button, Divider, IconButton, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {Plus, Trash2} from '@wso2/oxygen-ui-icons-react';
import {type JSX, useEffect, useRef, useState} from 'react';
import {useTranslation} from 'react-i18next';
import type {AccountLinking} from '../models/connection';
import {type KeyedLink, fromAccountLinking, newLinkRow, toAccountLinking} from '../utils/accountLinking';

export interface AccountLinkingSectionProps {
  initialConfig?: AccountLinking;
  onChange: (linking: AccountLinking | undefined) => void;
}

/**
 * Resolves a returning federated identity to an existing local account when its subject identifier
 * does not match an existing local subject, by the external attributes listed here (matched together,
 * AND).
 */
export default function AccountLinkingSection({
  initialConfig = undefined,
  onChange,
}: AccountLinkingSectionProps): JSX.Element {
  const {t} = useTranslation('connections');
  const [linking, setLinking] = useState<KeyedLink[]>(() => fromAccountLinking(initialConfig));

  // Keep the latest onChange without making the report-up effect depend on its identity: a caller
  // passing an inline handler (e.g. ConnectionDetailPage) would otherwise re-trigger the effect on
  // every parent render, and since the effect derives a fresh config object each time, that loops forever.
  const onChangeRef = useRef(onChange);
  useEffect(() => {
    onChangeRef.current = onChange;
  });

  useEffect(() => {
    onChangeRef.current(toAccountLinking(linking));
  }, [linking]);

  const lastLinkIsEmpty: boolean = linking.length > 0 && linking[linking.length - 1].value.trim() === '';

  const addLink = (): void => setLinking((prev) => [...prev, newLinkRow()]);
  const removeLink = (key: number): void => setLinking((prev) => prev.filter((entry) => entry.key !== key));
  const updateLink = (key: number, value: string): void =>
    setLinking((prev) => prev.map((entry) => (entry.key === key ? {...entry, value} : entry)));

  return (
    <SettingsCard title={t('attributeMapping.linking.title')} description={t('attributeMapping.linking.description')}>
      <Stack direction="column" spacing={1.5}>
        <Typography variant="body2" color="text.secondary" fontWeight={600}>
          {linking.length > 1 ? t('attributeMapping.linking.labelCombo') : t('attributeMapping.linking.label')}
        </Typography>
        {linking.map((entry, index) => {
          const canDeleteLink = entry.value.trim() !== '' || linking.length > 1;
          return (
            <Stack key={entry.key} direction="column" spacing={1.5}>
              {index > 0 && (
                <Stack direction="row" spacing={1} alignItems="center">
                  <Typography variant="caption" color="primary.main" fontWeight={700}>
                    {t('attributeMapping.linking.and')}
                  </Typography>
                  <Divider sx={{flex: 1}} />
                </Stack>
              )}
              <Stack direction="row" spacing={1.5} alignItems="center">
                <TextField
                  fullWidth
                  placeholder={t('attributeMapping.linking.placeholder')}
                  value={entry.value}
                  onChange={(e) => updateLink(entry.key, e.target.value)}
                  inputProps={{'aria-label': t('attributeMapping.linking.label')}}
                />
                {canDeleteLink ? (
                  <IconButton
                    onClick={() => removeLink(entry.key)}
                    aria-label="remove account linking attribute"
                    data-testid={`attribute-mapping-link-remove-${entry.key}`}
                  >
                    <Trash2 size={16} />
                  </IconButton>
                ) : (
                  <Box sx={{width: 40}} />
                )}
              </Stack>
            </Stack>
          );
        })}
        <Box>
          <Button
            variant="text"
            color="primary"
            size="small"
            startIcon={<Plus size={16} />}
            onClick={addLink}
            disabled={lastLinkIsEmpty}
            data-testid="attribute-mapping-link-add"
          >
            {t('attributeMapping.linking.addAttribute')}
          </Button>
        </Box>
      </Stack>
    </SettingsCard>
  );
}
