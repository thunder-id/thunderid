// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, Chip, IconButton, List, ListItem, ListItemText, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Plus, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AddTrustedIssuerCertificateDialog from './AddTrustedIssuerCertificateDialog';
import type {TrustedAuthorityInput} from '../models/presentation-requests';

export interface TrustedIssuerCertificatesProps {
  certificates: TrustedAuthorityInput[];
  onAdd: (entry: TrustedAuthorityInput) => void;
  onRemove: (certificateName: string) => void;
  /** Marks certificates that are not stored yet. Off in the create wizard, where none are. */
  markUnsaved?: boolean;
}

/**
 * The trusted issuer certificates of a presentation definition, with actions to add and remove them.
 * A definition needs at least one, so an empty list shows that requirement.
 */
export default function TrustedIssuerCertificates({
  certificates,
  onAdd,
  onRemove,
  markUnsaved = false,
}: TrustedIssuerCertificatesProps): JSX.Element {
  const {t} = useTranslation('verifiable-presentations');
  const [addOpen, setAddOpen] = useState<boolean>(false);

  return (
    <>
      <Stack spacing={2}>
        {certificates.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            {t('form.issuerTrust.certificates.required', 'Add at least one trusted issuer certificate.')}
          </Typography>
        ) : (
          <List aria-label={t('form.issuerTrust.certificates.title', 'Trusted issuer certificates')}>
            {certificates.map((entry: TrustedAuthorityInput) => (
              <ListItem
                key={entry.name}
                divider
                secondaryAction={
                  <Tooltip title={t('common:actions.remove')}>
                    <IconButton
                      edge="end"
                      color="error"
                      aria-label={t('form.issuerTrust.certificates.remove', {
                        name: entry.name,
                        defaultValue: 'Remove certificate {{name}}',
                      })}
                      onClick={(): void => onRemove(entry.name)}
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  </Tooltip>
                }
              >
                <ListItemText primary={entry.name} />
                {markUnsaved && entry.certificate !== undefined && (
                  <Chip size="small" label={t('form.issuerTrust.certificates.unsaved', 'Unsaved')} sx={{mr: 2}} />
                )}
              </ListItem>
            ))}
          </List>
        )}
        <Box>
          <Button variant="outlined" startIcon={<Plus size={16} />} onClick={(): void => setAddOpen(true)}>
            {t('form.issuerTrust.certificates.add', 'Add certificate')}
          </Button>
        </Box>
      </Stack>
      <AddTrustedIssuerCertificateDialog
        open={addOpen}
        existingNames={certificates.map((entry: TrustedAuthorityInput): string => entry.name)}
        onClose={(): void => setAddOpen(false)}
        onAdd={onAdd}
      />
    </>
  );
}
