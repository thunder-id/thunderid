// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueries, useQuery} from '@tanstack/react-query';
import {DocumentTitle} from '@thunderid/components';
import {
  gatewayColor,
  useAutoApply,
  usePipeline,
  useProjects,
  type PipelineStage,
  type Project,
} from '@thunderid/configure-gateways';
import {useProjectSelection} from '@thunderid/configure-projects';
import {useConfig} from '@thunderid/contexts';
import {User, useThunderID} from '@thunderid/react';
import {
  Box,
  Button,
  ButtonBase,
  Card,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  PageContent,
  Radio,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {ArrowRight, RefreshCw, Rocket, Send} from '@wso2/oxygen-ui-icons-react';
import {Fragment, useEffect, useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate} from 'react-router';
import RouteConfig from '../configs/RouteConfig';

/** Reads how many of a listed resource there are, from a list of one. */
function useCountReader(): (path: string) => Promise<number> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  return async (path: string): Promise<number> => {
    const separator = path.includes('?') ? '&' : '?';
    const response = (await http.request({
      url: `${getServerUrl()}${path}${separator}limit=1&offset=0`,
      method: 'GET',
    } as unknown as Parameters<typeof http.request>[0])) as {data: {totalResults?: number}};
    return response.data.totalResults ?? 0;
  };
}

/** How many of a listed resource there are. */
function useCount(path: string): number | undefined {
  const read = useCountReader();
  return useQuery<number>({
    queryKey: ['organization-home-count', path],
    queryFn: () => read(path),
    retry: false,
    staleTime: 60_000,
  }).data;
}

const applicationsOf = (project: Project): string => `/applications?projectId=${encodeURIComponent(project.id)}`;
const agentsOf = (project: Project): string => `/agents?projectId=${encodeURIComponent(project.id)}`;

/** How many applications or agents the projects hold together, and how many projects hold any. */
function useAcross(projects: Project[], pathOf: (project: Project) => string): {total?: number; projects: number} {
  const read = useCountReader();
  const results = useQueries({
    queries: projects.map((project: Project) => ({
      queryKey: ['organization-home-count', pathOf(project)],
      queryFn: () => read(pathOf(project)),
      retry: false,
      staleTime: 60_000,
    })),
  });
  if (results.some((result) => result.data === undefined)) return {projects: 0};
  const counts = results.map((result) => result.data ?? 0);
  return {
    total: counts.reduce((sum: number, count: number) => sum + count, 0),
    projects: counts.filter((count: number) => count > 0).length,
  };
}

/** A section of the overview: a heading with what it holds. */
function Section({
  title,
  subtitle = undefined,
  action = undefined,
  children = undefined,
  testId,
}: {
  title: ReactNode;
  subtitle?: string;
  action?: ReactNode;
  children?: ReactNode;
  testId: string;
}): JSX.Element {
  return (
    <Card variant="outlined" sx={{p: 3}} component="section" data-testid={testId}>
      <Stack direction="row" spacing={2} alignItems="flex-start" sx={{mb: children ? 2 : 0, flexWrap: 'wrap'}}>
        <Box sx={{flex: 1, minWidth: 240}}>
          <Typography variant="h6" component="h2">
            {title}
          </Typography>
          {subtitle && (
            <Typography variant="body2" color="text.secondary">
              {subtitle}
            </Typography>
          )}
        </Box>
        {action}
      </Stack>
      {children}
    </Card>
  );
}

/** How many applications and agents a project holds, as a person reads it. */
function useProjectCounts(project: Project): string {
  const {t} = useTranslation();
  const applications = useCount(applicationsOf(project));
  const agents = useCount(agentsOf(project));
  const countOf = (count: number | undefined, key: string, one: string, other: string, none: string): string => {
    if (count === undefined) return '';
    if (count === 0) return t(`home:organization.${key}None`, none);
    return t(`home:organization.${key}`, {count, defaultValue_one: one, defaultValue_other: other});
  };
  return [
    countOf(applications, 'applications', '{{count}} application', '{{count}} applications', 'No applications'),
    countOf(agents, 'agents', '{{count}} agent', '{{count}} agents', 'No agents'),
  ]
    .filter(Boolean)
    .join(' · ');
}

/** One project, with how many applications and agents it holds; it opens the project. */
function ProjectRow({project, onOpen}: {project: Project; onOpen: () => void}): JSX.Element {
  const counts = useProjectCounts(project);
  return (
    <ButtonBase
      onClick={onOpen}
      sx={{
        display: 'flex',
        justifyContent: 'flex-start',
        gap: 2,
        width: '100%',
        textAlign: 'left',
        px: 2,
        py: 1.5,
        border: 1,
        borderColor: 'divider',
        borderRadius: 1,
        '&:hover': {bgcolor: 'action.hover'},
      }}
      data-testid={`organization-project-${project.id}`}
    >
      <Typography variant="body1" fontWeight={500} sx={{flex: 1, minWidth: 0}} noWrap>
        {project.displayName}
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {counts}
      </Typography>
      <ArrowRight size={16} />
    </ButtonBase>
  );
}

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
  const counts = useProjectCounts(project);
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

/** What the project picker was opened for: a deploy, or a new application or agent. */
type Pick = 'deploy' | 'application' | 'agent';

/** The arrow from one step of the path to the next, named for what moves configuration along it. */
function Joint({label, icon}: {label: string; icon: ReactNode}): JSX.Element {
  return (
    <Stack alignItems="center" justifyContent="center" spacing={0.5} sx={{color: 'primary.main', px: 0.5}}>
      <Typography variant="caption" fontWeight={600} color="inherit">
        {label}
      </Typography>
      <Stack direction="row" spacing={0.5} alignItems="center" aria-hidden>
        {icon}
        <ArrowRight size={16} />
      </Stack>
    </Stack>
  );
}

/** Where a gateway is reached, for a person to tell gateways apart. */
const hostOf = (url: string | undefined): string => {
  if (!url) return '';
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
};

/** A person's first name, as their profile gives it. */
const firstNameOf = (user: unknown): string | undefined => {
  const name = (user as {name?: unknown} | null)?.name;
  return typeof name === 'string' && name ? name.split(' ')[0] : undefined;
};

export interface OrganizationHomePageProps {
  /** Says where the gateways come from, as the deployment has them. */
  gatewaysSource?: string;
  /** Offers to create a project, where the console can. */
  canCreateProject?: boolean;
}

/**
 * The organization's overview: how configuration moves from this console through the gateways, the
 * applications and agents the projects hold, and the projects themselves.
 */
export default function OrganizationHomePage({
  gatewaysSource = undefined,
  canCreateProject = true,
}: OrganizationHomePageProps): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const pipeline = usePipeline();
  const {projects, selectProject: select, clearSelection} = useProjectSelection();
  // The organization's home is the organization level: no project is selected while it is open.
  useEffect(() => clearSelection(), [clearSelection]);
  const projectList = useProjects();
  const autoApply = useAutoApply();
  const applications = useAcross(projects, applicationsOf);
  const agents = useAcross(projects, agentsOf);
  const stages: PipelineStage[] = pipeline.data?.stages ?? [];
  const [picking, setPicking] = useState<Pick | null>(null);
  const [chosen, setChosen] = useState<string | null>(null);

  const openProject = (id: string): void => {
    select(id);
    void navigate(RouteConfig.projects.home(id));
  };
  const pick = (purpose: Pick): void => {
    setChosen(projects[0]?.id ?? null);
    setPicking(purpose);
  };
  const proceed = (): void => {
    if (!chosen || !picking) return;
    select(chosen);
    setPicking(null);
    if (picking === 'application') void navigate(RouteConfig.applications.types());
    else if (picking === 'agent') void navigate(RouteConfig.agents.create());
    else void navigate(RouteConfig.projects.home(chosen));
  };

  // What a gateway is for along the path: where a deploy enters, or what it is promoted from.
  const roleOf = (index: number): string => {
    let role: string;
    if (index === 0) {
      role = autoApply.data?.enabled
        ? t('home:organization.roleEverySave', 'Every save')
        : t('home:organization.roleDeployFirst', 'Deploy here first');
    } else {
      role = t('home:organization.rolePromoteFrom', 'Promote from {{previous}}', {previous: stages[index - 1].name});
    }
    return index > 0 && index === stages.length - 1
      ? `${role} · ${t('home:organization.roleLast', 'Last stage')}`
      : role;
  };

  const pickerText: Record<Pick, {title: string; subtitle: string; note: string}> = {
    deploy: {
      title: t('home:organization.deploy', 'Deploy'),
      subtitle: t(
        'home:organization.pickDeploySubtitle',
        'Choose the project to deploy. Each project deploys through its own pipeline.',
      ),
      note: t(
        'home:organization.pickDeployNote',
        'Its latest version enters the pipeline at the first gateway, together with shared resources.',
      ),
    },
    application: {
      title: t('home:organization.newApplication', 'New application'),
      subtitle: t(
        'home:organization.pickApplicationSubtitle',
        'Applications belong to a project. Choose where this one lives.',
      ),
      note: t('home:organization.pickCreateNote', "It will be deployed through this project's pipeline."),
    },
    agent: {
      title: t('home:organization.newAgent', 'New agent'),
      subtitle: t('home:organization.pickAgentSubtitle', 'Agents belong to a project. Choose where this one lives.'),
      note: t('home:organization.pickCreateNote', "It will be deployed through this project's pipeline."),
    },
  };

  const acrossTile = (
    kind: 'applications' | 'agents',
    across: {total?: number; projects: number},
    lead: string,
    action: string,
    onAction: () => void,
  ): JSX.Element => (
    <Card variant="outlined" sx={{p: 2.5, flex: 1, minWidth: 240}} data-testid={`organization-${kind}`}>
      <Stack direction="row" spacing={1} alignItems="baseline">
        <Typography variant="h4" component="span">
          {across.total ?? '·'}
        </Typography>
        <Typography variant="subtitle1" component="span">
          {kind === 'applications'
            ? t('home:organization.applicationsLabel', 'applications')
            : t('home:organization.agentsLabel', 'agents')}
        </Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary">
        {lead}
      </Typography>
      <Typography variant="caption" color="text.secondary" component="div" sx={{mt: 0.5}}>
        {t('home:organization.acrossProjects', {
          count: across.projects,
          defaultValue_one: 'Across {{count}} project',
          defaultValue_other: 'Across {{count}} projects',
        })}
      </Typography>
      <Button variant="outlined" size="small" sx={{mt: 1.5}} onClick={onAction} disabled={projects.length === 0}>
        {action}
      </Button>
    </Card>
  );

  return (
    <PageContent>
      <DocumentTitle title={t('home:organization.title', 'Overview')} />
      <Stack spacing={4}>
        <User>
          {(user) => (
            <Box>
              <Typography variant="h1" sx={{fontSize: {xs: '2rem', md: '2.5rem'}, fontWeight: 500}} gutterBottom>
                {t('home:organization.hello', 'Hello, {{name}}', {
                  name: firstNameOf(user) ?? t('home:organization.there', 'there'),
                })}
              </Typography>
              <Typography variant="body1" color="text.secondary">
                {t(
                  'home:organization.lead',
                  'Shared settings here apply to every project. Each project builds its own applications and agents and runs them through its pipeline.',
                )}
              </Typography>
            </Box>
          )}
        </User>

        <Section
          title={t('home:organization.howItWorks', 'How it works')}
          subtitle={
            stages.length === 1
              ? t(
                  'home:organization.howItWorksLeadSingle',
                  'Configure here, then deploy to your gateway. It has its own users and the apps connected to it.',
                )
              : t(
                  'home:organization.howItWorksLead',
                  'Configure here, then deploy to your first gateway and promote through the rest. Each gateway has its own users and the apps connected to it.',
                )
          }
          action={
            projects.length > 0 ? (
              <Button
                variant="contained"
                size="small"
                startIcon={<Rocket size={16} />}
                onClick={() => pick('deploy')}
                sx={{borderRadius: 100}}
                data-testid="organization-deploy"
              >
                {t('home:organization.deploy', 'Deploy')}
              </Button>
            ) : undefined
          }
          testId="organization-how-it-works"
        >
          {pipeline.isLoading && <CircularProgress size={20} />}
          <Stack direction="row" spacing={1.5} sx={{overflowX: 'auto', pb: 1}} alignItems="stretch">
            <Card variant="outlined" sx={{p: 2, minWidth: 180, flexShrink: 0}}>
              <Typography variant="overline" color="text.secondary">
                {t('home:organization.thisConsole', 'This console')}
              </Typography>
              <Typography variant="subtitle1" fontWeight={600}>
                {t('home:organization.configureHere', 'Configure here')}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {t('home:organization.projectManaged', 'Project-managed resources')}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {t('home:organization.globalShared', 'Global shared resources')}
              </Typography>
            </Card>
            {stages.map((stage: PipelineStage, index: number) => {
              const color = gatewayColor(index);
              const tag = stage.name.toUpperCase();
              const host = hostOf(stage.gateway.baseUrl);
              return (
                <Fragment key={stage.gateway.id}>
                  {index === 0 ? (
                    <Joint label={t('home:organization.autoSync', 'Auto-sync')} icon={<RefreshCw size={14} />} />
                  ) : (
                    <Joint label={t('home:organization.promote', 'Promote')} icon={<Send size={14} />} />
                  )}
                  <Card
                    variant="outlined"
                    sx={{p: 2, minWidth: 200, flexShrink: 0, borderTop: `3px solid ${color.light}`}}
                    data-testid={`organization-gateway-${stage.gateway.id}`}
                  >
                    <Typography variant="caption" sx={{fontFamily: 'monospace', color: color.light}}>
                      {tag}
                    </Typography>
                    <Typography variant="subtitle1" fontWeight={600}>
                      {stage.name}
                    </Typography>
                    <Typography variant="body2" color="text.secondary" data-testid="organization-gateway-role">
                      {roleOf(index)}
                    </Typography>
                    {/* Who a gateway serves: the last one live users, the ones before it testers. */}
                    {(index > 0 && index === stages.length - 1
                      ? [
                          t('home:organization.liveApps', 'Live apps'),
                          t('home:organization.liveUsers', 'Customers, employees'),
                        ]
                      : [t('home:organization.testApps', 'Apps'), t('home:organization.testUsers', 'Test users')]
                    ).map((line: string) => (
                      <Typography key={line} variant="caption" color="text.secondary" component="div">
                        {line}
                      </Typography>
                    ))}
                    {host && (
                      <Typography variant="caption" color="text.secondary" component="div">
                        {host}
                      </Typography>
                    )}
                    <Button
                      component={Link}
                      to={RouteConfig.deploy.gateway(stage.gateway.id)}
                      size="small"
                      variant="text"
                      endIcon={<ArrowRight size={14} />}
                      sx={{px: 0, mt: 1}}
                    >
                      {t('home:organization.openGateway', 'Open {{tag}}', {tag})}
                    </Button>
                  </Card>
                </Fragment>
              );
            })}
          </Stack>
          <Typography variant="caption" color="text.secondary" component="div" sx={{mt: 1}}>
            {gatewaysSource ??
              t('home:organization.gatewaysSource', 'Gateways come from the deployment configuration.')}
          </Typography>
        </Section>

        <Section
          title={t('home:organization.applicationsAndAgents', 'Applications and agents')}
          subtitle={t(
            'home:organization.applicationsAndAgentsLead',
            'The core of your identity setup. Every application and agent belongs to a project, which configures, deploys and promotes it.',
          )}
          testId="organization-applications-and-agents"
        >
          <Stack direction="row" spacing={2} sx={{flexWrap: 'wrap', rowGap: 2}}>
            {acrossTile(
              'applications',
              applications,
              t('home:organization.applicationsLead', 'Web, mobile and API clients that sign users in'),
              t('home:organization.newApplication', 'New application'),
              () => pick('application'),
            )}
            {acrossTile(
              'agents',
              agents,
              t('home:organization.agentsLead', 'AI agents with their own identity and access'),
              t('home:organization.newAgent', 'New agent'),
              () => pick('agent'),
            )}
          </Stack>
        </Section>

        <Section
          title={t('home:organization.projectsCount', 'Projects · {{count}}', {count: projects.length})}
          subtitle={t('home:organization.projectsLead', 'Each project holds its own applications and agents.')}
          action={
            canCreateProject ? (
              <Button variant="outlined" size="small" onClick={() => void navigate(RouteConfig.projects.create())}>
                {t('home:organization.createProject', 'Create project')}
              </Button>
            ) : undefined
          }
          testId="organization-projects"
        >
          {projectList.isLoading && <CircularProgress size={20} />}
          <Stack spacing={1}>
            {projects.map((project: Project) => (
              <ProjectRow key={project.id} project={project} onOpen={() => openProject(project.id)} />
            ))}
          </Stack>
        </Section>
      </Stack>

      <Dialog open={picking !== null} onClose={() => setPicking(null)} fullWidth maxWidth="xs">
        {picking && (
          <>
            <DialogTitle>
              {pickerText[picking].title}
              <Typography variant="body2" color="text.secondary" component="div">
                {pickerText[picking].subtitle}
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
                {pickerText[picking].note}{' '}
                {canCreateProject && (
                  <Box
                    component={Link}
                    to={RouteConfig.projects.create()}
                    sx={{color: 'primary.main'}}
                    onClick={() => setPicking(null)}
                  >
                    {t('home:organization.needProject', 'Need a new project? Create one.')}
                  </Box>
                )}
              </Typography>
            </DialogContent>
            <DialogActions>
              <Button onClick={() => setPicking(null)}>{t('home:organization.pickCancel', 'Cancel')}</Button>
              <Button variant="contained" onClick={proceed} disabled={!chosen} data-testid="project-picker-continue">
                {t('home:organization.pickContinue', 'Continue')}
              </Button>
            </DialogActions>
          </>
        )}
      </Dialog>
    </PageContent>
  );
}
