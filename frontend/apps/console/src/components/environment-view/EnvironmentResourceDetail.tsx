// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ResourceAvatar} from '@thunderid/components';
import {Chip, PageContent, PageTitle, Paper, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link} from 'react-router';
import {useEnvironmentView} from './EnvironmentViewContext';
import {descriptionOf, nameOf, type ResourceLayout} from './layouts';
import type {AppliedResource} from './models';
import {valueAt} from './paths';
import ResourceSections from './ResourceSections';

/** The avatar a resource without a logo of its own shows, as the configuration's pages show it. */
const DEFAULT_AVATAR = 'avatar:shape=rounded,variant=anonymous_entity,content=cube,colors=0';

/** The fields a resource's page names its kind by, beside its name. */
const KIND_FIELDS: readonly string[] = ['template', 'type', 'flowType'];

export interface EnvironmentResourceDetailProps {
  layout: ResourceLayout;
  /** The resource, absent when the environment does not run one by the identifier asked for. */
  applied?: AppliedResource;
}

/**
 * One resource as an environment runs it, with the header and tabs its page in the configuration
 * has, and nothing that changes it.
 */
export default function EnvironmentResourceDetail({
  layout,
  applied = undefined,
}: EnvironmentResourceDetailProps): JSX.Element {
  const {t} = useTranslation();
  const {environment} = useEnvironmentView();
  const back = (
    <PageTitle.BackButton component={<Link to={`/${layout.segment}`} />}>
      {t('common:environment.detail.back', 'Back to {{section}}', {section: t(...layout.title)})}
    </PageTitle.BackButton>
  );

  if (!applied) {
    return (
      <PageContent>
        <PageTitle>{back}</PageTitle>
        <Paper variant="outlined" sx={{p: 4, textAlign: 'center'}}>
          <Typography variant="body2" color="text.secondary">
            {t('common:environment.detail.missing', '{{environment}} does not run this resource.', {
              environment: environment.name,
            })}
          </Typography>
        </Paper>
      </PageContent>
    );
  }

  const logo = valueAt(applied.resource, 'logoUrl');
  const kinds = [
    ...(layout.types.length > 1 ? [applied.resourceType] : []),
    ...KIND_FIELDS.map((field: string) => valueAt(applied.resource, field)).filter(
      (value: unknown): value is string => typeof value === 'string' && value !== '',
    ),
  ];
  return (
    <PageContent>
      <PageTitle>
        {back}
        <PageTitle.Header>
          <Stack direction="row" alignItems="center" spacing={2} mb={1}>
            {layout.logo && (
              <ResourceAvatar
                variant="rounded"
                value={typeof logo === 'string' ? logo : undefined}
                size={48}
                fallback={DEFAULT_AVATAR}
              />
            )}
            <Typography variant="h3">{nameOf(applied.resource, applied.id, layout)}</Typography>
            {kinds.map((kind: string) => (
              <Chip key={kind} label={kind} size="small" color="primary" variant="outlined" />
            ))}
          </Stack>
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <Typography variant="body2" color="text.secondary">
            {descriptionOf(applied.resource) ?? t('common:environment.detail.noDescription', 'No description')}
          </Typography>
        </PageTitle.SubHeader>
      </PageTitle>
      <ResourceSections
        key={`${applied.resourceType}/${applied.id}`}
        resource={applied.resource}
        parts={applied.parts}
        layout={layout}
      />
    </PageContent>
  );
}
