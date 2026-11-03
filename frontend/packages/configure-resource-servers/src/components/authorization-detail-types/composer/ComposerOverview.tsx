// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box, Button, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import ComposerPanel from './ComposerPanel';
import KindIcon from './KindIcon';
import PanelSection from './PanelSection';
import PolicyMap from './PolicyMap';
import SummaryEditor from './SummaryEditor';
import useComposerLabels from './useComposerLabels';
import type {ConsentSettings} from '../../../hooks/useAuthorizationDetailTypeForm';
import type {SchemaField} from '../../../models/authorization-detail-type';
import {effectiveModification} from '../../../utils/authorization-detail-schema';
import {
  duplicateLabels,
  exampleDetail,
  flattenFields,
  PALETTE_ITEMS,
  type PaletteItem,
} from '../../../utils/schema-composer';

export interface ComposerOverviewProps {
  /** The fields of the type, or null when they are not known. */
  fields: SchemaField[] | null;
  /** The type identifier, which the example detail carries. */
  typeValue: string;
  consent: ConsentSettings;
  readOnly?: boolean;
  /**
   * Edits the summary here, where there is no card to select it on; otherwise the summary is edited
   * once selected on the card.
   */
  editSummary?: boolean;
  /** Selects a field for editing; left out where fields cannot be selected. */
  onSelect?: (key: string) => void;
  /** Adds a field of a kind at the end of the card; left out where fields cannot be added here. */
  onAdd?: (item: PaletteItem) => void;
  /** A notice about the summary, such as one that names an old path. */
  notice?: ReactNode;
}

/**
 * The side panel while nothing on the card is selected: the consent summary and the notices about it, the
 * kinds of field to add, who can change which field, and an example detail a client sends.
 */
export default function ComposerOverview({
  fields,
  typeValue,
  consent,
  readOnly = false,
  editSummary = false,
  onSelect = undefined,
  onAdd = undefined,
  notice = undefined,
}: ComposerOverviewProps): JSX.Element {
  const {t} = useTranslation();
  const labels = useComposerLabels();
  const rows = fields
    ? flattenFields(fields).filter((row) => row.field.name.trim() && row.field.kind !== 'object')
    : [];
  const changeable = rows.filter((row) => !row.processorSupplied && effectiveModification(row.field) !== 'none');
  const repeatedLabels = fields ? duplicateLabels(fields) : [];

  return (
    <ComposerPanel
      title={t('authorizationDetailTypes:composer.overview.title', 'Overview')}
      subtitle={
        editSummary
          ? undefined
          : readOnly
            ? t('authorizationDetailTypes:composer.overview.hintReadOnly', 'Select an element on the card to see it.')
            : t('authorizationDetailTypes:composer.overview.hint', 'Select an element on the card to edit it.')
      }
      aria-label={t('authorizationDetailTypes:composer.overview.title', 'Overview')}
      flush
    >
      {(editSummary || Boolean(notice) || repeatedLabels.length > 0) && (
        <PanelSection title={t('authorizationDetailTypes:composer.overview.consent', 'Consent')} defaultExpanded>
          <Stack spacing={2.5}>
            {notice}
            {repeatedLabels.length > 0 && (
              <Alert severity="warning" data-testid="composer-repeated-labels">
                {t(
                  'authorizationDetailTypes:composer.overview.repeatedLabels',
                  'The consent screen shows {{labels}} on more than one row, and the user cannot tell them apart. Give those fields different display names.',
                  {labels: repeatedLabels.join(', '), interpolation: {escapeValue: false}},
                )}
              </Alert>
            )}
            {editSummary && (
              <SummaryEditor consent={consent} fields={fields} typeValue={typeValue} readOnly={readOnly} />
            )}
          </Stack>
        </PanelSection>
      )}
      {onAdd && !readOnly && (
        <PanelSection
          title={t('authorizationDetailTypes:composer.palette.title', 'Add a field')}
          summary={t('authorizationDetailTypes:composer.palette.summary', 'Adds to the end of the card')}
          defaultExpanded={fields?.length === 0}
        >
          <Box
            role="group"
            aria-label={t('authorizationDetailTypes:composer.palette.title', 'Add a field')}
            sx={{display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 1}}
          >
            {PALETTE_ITEMS.map((item) => (
              <Button
                key={item}
                variant="outlined"
                color="inherit"
                size="small"
                startIcon={<KindIcon kind={item} />}
                onClick={() => onAdd(item)}
                sx={{justifyContent: 'flex-start', textTransform: 'none', borderColor: 'divider'}}
              >
                {labels.palette[item]}
              </Button>
            ))}
          </Box>
        </PanelSection>
      )}
      {rows.length > 0 && fields && (
        <PanelSection
          title={t('authorizationDetailTypes:composer.overview.policy', 'Who can change what')}
          summary={t(
            'authorizationDetailTypes:composer.overview.policySummary',
            '{{changeable}} of {{count}} fields can change after the request',
            {
              changeable: changeable.length,
              count: rows.length,
              defaultValue_one: '{{changeable}} of {{count}} field can change after the request',
            },
          )}
        >
          <PolicyMap fields={fields} onSelect={onSelect} />
        </PanelSection>
      )}
      {fields && (
        <PanelSection
          title={t('authorizationDetailTypes:composer.overview.example', 'Example request')}
          summary={t('authorizationDetailTypes:composer.overview.exampleSummary', 'What a client sends')}
        >
          <Box
            component="pre"
            data-testid="composer-example-detail"
            aria-label={t('authorizationDetailTypes:composer.overview.example', 'Example request')}
            sx={{
              m: 0,
              p: 2,
              borderRadius: 1.5,
              bgcolor: 'action.hover',
              fontFamily: 'monospace',
              fontSize: 12,
              overflow: 'auto',
              maxHeight: 320,
            }}
          >
            {JSON.stringify(exampleDetail(fields, typeValue), null, 2)}
          </Box>
          {rows.some((row) => row.processorSupplied) && (
            <Typography variant="caption" color="text.secondary" component="p" sx={{mt: 1}}>
              {t(
                'authorizationDetailTypes:composer.overview.processorNote',
                'Fields the external processor supplies are filled in after the client sends the request.',
              )}
            </Typography>
          )}
        </PanelSection>
      )}
    </ComposerPanel>
  );
}
