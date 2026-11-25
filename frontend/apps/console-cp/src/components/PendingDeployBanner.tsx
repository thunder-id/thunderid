// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useAutoApply, usePipeline, useProjectsPending, type Project} from '@thunderid/configure-gateways';
import {useProjectSelection} from '@thunderid/configure-projects';
import {useCapabilitiesContext} from '@thunderid/contexts';
import {
  Box,
  Button,
  ButtonBase,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Radio,
  Stack,
  Typography,
  alpha,
} from '@wso2/oxygen-ui';
import {CircleAlert, Rocket} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link} from 'react-router';
import RouteConfig from '../configs/RouteConfig';
import useProjectCounts from '../hooks/useProjectCounts';

/** The banner's rounded button, as the design draws it. */
const PILL = {minHeight: 40, px: 2, borderRadius: '20px', fontSize: 13, fontWeight: 600, whiteSpace: 'nowrap'};

/** One project to choose in the picker, with what it holds. */
function PickerOption({
  project,
  checked,
  onChoose,
}: {
  project: Project;
  checked: boolean;
  onChoose: () => void;
}): JSX.Element {
  const counts = useProjectCounts(project.id);
  return (
    <ButtonBase
      onClick={onChoose}
      role="radio"
      aria-checked={checked}
      sx={{
        display: 'flex',
        justifyContent: 'flex-start',
        gap: 1,
        width: '100%',
        textAlign: 'left',
        px: 1,
        py: 1,
        border: 1,
        borderColor: checked ? 'primary.main' : 'divider',
        borderRadius: 1,
      }}
      data-testid={`project-picker-${project.id}`}
    >
      <Radio checked={checked} size="small" tabIndex={-1} />
      <Box sx={{flex: 1, minWidth: 0}}>
        <Typography variant="body2" fontWeight={500} noWrap>
          {project.displayName}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {counts}
        </Typography>
      </Box>
    </ButtonBase>
  );
}

export interface PendingDeployBannerProps {
  /** Offers to create a project from the picker, where the console can. */
  canCreateProject?: boolean;
}

/**
 * Tells, on every page, that changes are waiting to reach the first gateway. Inside a project it counts
 * that project's changes and the shared ones, and its deploy opens the project's pipeline; outside any project it
 * counts every project's, and its deploy asks which project to open.
 */
export default function PendingDeployBanner({canCreateProject = true}: PendingDeployBannerProps): JSX.Element | null {
  const {t} = useTranslation();
  const pipeline = usePipeline();
  const {projects: allProjects, selectedProject, selectProject: select} = useProjectSelection();
  const projects = useMemo(
    () => (selectedProject ? allProjects.filter((project: Project) => project.id === selectedProject.id) : allProjects),
    [allProjects, selectedProject],
  );
  const [picking, setPicking] = useState(false);
  const [chosen, setChosen] = useState<string | null>(null);
  const autoApply = useAutoApply();
  const canDeploy = useCapabilitiesContext().can('gateway', 'update');
  const auto = autoApply.data?.enabled ?? false;
  const first = pipeline.data?.stages[0];
  const firstTag = first?.name ?? '';
  const projectIds = useMemo(() => projects.map((project: Project) => project.id), [projects]);
  const pending = useProjectsPending(first?.gateway.id ?? '', projectIds);
  const pendingProjects = projects.filter((project: Project) => (pending.byProject[project.id] ?? 0) > 0);
  const pendingInProjects = pendingProjects.reduce(
    (sum: number, project: Project) => sum + (pending.byProject[project.id] ?? 0),
    0,
  );

  // A project opens on its pipeline, where its deploy is.
  const open = (id: string): void => {
    const handle = allProjects.find((project) => project.id === id)?.handle;
    if (handle) select(handle, '/pipeline');
  };
  const deploy = (): void => {
    if (selectedProject) {
      open(selectedProject.id);
      return;
    }
    setChosen(pendingProjects[0]?.id ?? projects[0]?.id ?? null);
    setPicking(true);
  };
  const proceed = (): void => {
    if (!chosen) return;
    setPicking(false);
    open(chosen);
  };

  // What is waiting to reach the first gateway: the projects' own changes, then the shared ones.
  const inProjects =
    pendingProjects.length === 1
      ? t('home:organization.pendingInProject', {
          count: pendingInProjects,
          project: pendingProjects[0].displayName,
          defaultValue_one: '{{count}} change in {{project}}',
          defaultValue_other: '{{count}} changes in {{project}}',
        })
      : t('home:organization.pendingInProjects', '{{count}} changes across {{projects}} projects', {
          count: pendingInProjects,
          projects: pendingProjects.length,
        });
  let pendingSummary = '';
  if (pendingProjects.length > 0 && pending.shared > 0)
    pendingSummary = t('home:organization.pendingAndShared', '{{changes}} and {{count}} in shared resources', {
      changes: inProjects,
      count: pending.shared,
    });
  else if (pendingProjects.length > 0) pendingSummary = inProjects;
  else if (pending.shared > 0)
    pendingSummary = t('home:organization.pendingShared', {
      count: pending.shared,
      defaultValue_one: '{{count}} change in shared resources',
      defaultValue_other: '{{count}} changes in shared resources',
    });
  if (!first || auto || pendingSummary === '') return null;

  return (
    <Box sx={{width: '100%', maxWidth: 1400, mx: 'auto', px: 8, pt: 8, boxSizing: 'border-box'}}>
      <Stack
        role="status"
        direction="row"
        alignItems="center"
        sx={{
          flexWrap: 'wrap',
          gap: '12px 16px',
          p: '14px 16px 14px 18px',
          borderRadius: '12px',
          bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1),
          border: '1px solid',
          borderColor: (theme) => alpha(theme.palette.primary.main, 0.4),
        }}
        data-testid="organization-pending"
      >
        <Box sx={{display: 'flex', color: 'primary.main'}}>
          <CircleAlert size={20} />
        </Box>
        <Stack spacing={0.25} sx={{flex: '1 1 360px', minWidth: 0}}>
          <Typography sx={{fontWeight: 600}}>
            {t('home:organization.pendingTitle', "You have changes that aren't in {{target}} yet", {
              target: firstTag,
            })}
          </Typography>
          <Typography color="text.secondary" sx={{fontSize: 13}}>
            {t('home:organization.pendingDetail', '{{summary}}. Deploy them to {{target}} to try them out.', {
              summary: pendingSummary,
              target: firstTag,
            })}
          </Typography>
        </Stack>
        {canDeploy && (
          <Button
            variant="contained"
            startIcon={<Rocket size={16} />}
            onClick={deploy}
            disabled={projects.length === 0}
            sx={PILL}
            data-testid="organization-deploy"
          >
            {t('home:organization.deployTo', 'Deploy to {{target}}', {target: firstTag})}
          </Button>
        )}
      </Stack>

      <Dialog open={picking} onClose={() => setPicking(false)} fullWidth maxWidth="xs">
        <DialogTitle>
          {t('home:organization.deploy', 'Deploy')}
          <Typography variant="body2" color="text.secondary" component="div">
            {t(
              'home:organization.pickDeploySubtitle',
              'Choose the project to deploy. Each project deploys through its own pipeline.',
            )}
          </Typography>
        </DialogTitle>
        <DialogContent>
          <Typography variant="overline" color="text.secondary">
            {t('home:organization.pickProject', 'Project')}
          </Typography>
          <Stack spacing={1} role="radiogroup" data-testid="project-picker">
            {projects.map((project: Project) => (
              <PickerOption
                key={project.id}
                project={project}
                checked={chosen === project.id}
                onChoose={() => setChosen(project.id)}
              />
            ))}
          </Stack>
          <Typography variant="caption" color="text.secondary" component="div" sx={{mt: 1.5}}>
            {t(
              'home:organization.pickDeployNote',
              'Its latest version enters the pipeline at the first gateway, together with shared resources.',
            )}{' '}
            {canCreateProject && (
              <Box
                component={Link}
                to={RouteConfig.projects.create()}
                sx={{color: 'primary.main'}}
                onClick={() => setPicking(false)}
              >
                {t('home:organization.needProject', 'Need a new project? Create one.')}
              </Box>
            )}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPicking(false)}>{t('home:organization.pickCancel', 'Cancel')}</Button>
          <Button variant="contained" onClick={proceed} disabled={!chosen} data-testid="project-picker-continue">
            {t('home:organization.pickContinue', 'Continue')}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
