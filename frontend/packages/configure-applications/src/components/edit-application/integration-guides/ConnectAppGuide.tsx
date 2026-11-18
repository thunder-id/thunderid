// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {GuideOptionMenu, QueryErrorNotice, SEGMENTED_CONTROL_SX} from '@thunderid/components';
import {useCopyToClipboard} from '@thunderid/hooks';
import {
  alpha,
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  CircularProgress,
  Paper,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ArrowRight,
  ArrowUpRight,
  Bot,
  BookOpen,
  Check,
  ChevronDown,
  ChevronUp,
  Copy,
  Download,
  ListOrdered,
  LogIn,
  UserPlus,
} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type ReactNode} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {Link as RouterLink} from 'react-router';
import GuideStepList from './GuideStepList';
import QuickstartGuide, {type QuickstartGuideSource} from './QuickstartGuide';
import useGetCodingAgentPrompt from '../../../api/useGetCodingAgentPrompt';
import useGetQuickstartGuide from '../../../api/useGetQuickstartGuide';
import useApplicationRoutes from '../../../hooks/useApplicationRoutes';
import {fillQuickstartGuideCode} from '../../../utils/parseQuickstartGuide';

/**
 * Props for the {@link ConnectAppGuide} component.
 *
 * @public
 */
export interface ConnectAppGuideProps {
  /**
   * The docs quickstarts for the application's template, one per platform it covers
   */
  guides: QuickstartGuideSource[];
  /**
   * The URL of the template's coding agent prompt on the docs site, when it has one. A guide with a
   * prompt of its own uses that instead.
   */
  promptUrl?: string;
  /**
   * Values for the prompt's `{{placeholder}}` slots, by placeholder name
   */
  promptValues: Record<string, string | undefined>;
  /**
   * Placeholder text in the docs code samples, mapped to this application's value for it
   */
  codeValues: Record<string, string | undefined>;
  /**
   * Where the app runs during development (e.g. "http://localhost:5173"), when the template knows
   */
  devServerUrl?: string;
  /**
   * The application's allowed redirect URIs
   */
  redirectUris?: string[];
  /**
   * Whether people can sign up for an account themselves
   */
  isSelfSignUpEnabled: boolean;
  /**
   * Whether the application has a sign-in page, so a person needs an account to try it
   */
  isSignInFacing: boolean;
  /**
   * Navigates to the application's Flows tab
   */
  onGoToFlows?: () => void;
  /**
   * Opens an external URL, after whatever confirmation the host page wants
   */
  onOpenLink: (url: string) => void;
}

/** Fills a prompt's `{{placeholder}}` slots, leaving any without a value as written. */
function fillPrompt(text: string, values: Record<string, string | undefined>): string {
  return Object.entries(values).reduce((result, [key, value]) => {
    if (!value || value.trim() === '') {
      return result;
    }
    // A callback, so the value goes in as written: a replacement string reads `$&`, `$$`, and the like.
    return result.replace(new RegExp(`\\{\\{${key}\\}\\}`, 'g'), () => value);
  }, text);
}

/** One of a pair of choices, as a card with an icon, a title, and a line on what it means. */
function ChoiceCard({
  icon,
  title,
  description,
  isSelected,
  onSelect,
}: {
  icon: ReactNode;
  title: string;
  description: string;
  isSelected: boolean;
  onSelect: () => void;
}): JSX.Element {
  return (
    <Card variant="outlined" sx={{borderColor: isSelected ? 'primary.main' : 'divider'}}>
      <CardActionArea role="radio" aria-checked={isSelected} onClick={onSelect} sx={{height: '100%'}}>
        <CardContent sx={{display: 'flex', alignItems: 'flex-start', gap: 1.5}}>
          <Box sx={{display: 'flex', color: 'text.secondary', pt: 0.25}}>{icon}</Box>
          <Box sx={{minWidth: 0}}>
            <Typography variant="subtitle2">{title}</Typography>
            <Typography variant="body2" color="text.secondary">
              {description}
            </Typography>
          </Box>
        </CardContent>
      </CardActionArea>
    </Card>
  );
}

/** A card that leads somewhere, with an icon, a title, and a line on where it goes. */
function ActionCard({
  icon,
  title,
  description,
  to = undefined,
  onClick = undefined,
}: {
  icon: ReactNode;
  title: string;
  description: string;
  to?: string;
  onClick?: () => void;
}): JSX.Element {
  const content = (
    <CardContent sx={{display: 'flex', alignItems: 'center', gap: 1.5, py: 1.5}}>
      <Box sx={{display: 'flex', color: 'text.secondary'}}>{icon}</Box>
      <Box sx={{flex: 1, minWidth: 0}}>
        <Typography variant="subtitle2">{title}</Typography>
        <Typography variant="caption" color="text.secondary">
          {description}
        </Typography>
      </Box>
      <Box sx={{display: 'flex', color: 'primary.main'}}>
        <ArrowRight size={16} aria-hidden />
      </Box>
    </CardContent>
  );

  return (
    <Card variant="outlined">
      {to ? (
        <CardActionArea component={RouterLink} to={to}>
          {content}
        </CardActionArea>
      ) : (
        <CardActionArea onClick={onClick}>{content}</CardActionArea>
      )}
    </Card>
  );
}

/** Where to get someone to sign in with: a test user, or self sign-up when it is on or can be turned on. */
function SignInHelp({
  isSelfSignUpEnabled,
  onGoToFlows = undefined,
}: {
  isSelfSignUpEnabled: boolean;
  onGoToFlows?: () => void;
}): JSX.Element {
  const {t} = useTranslation();
  const routes = useApplicationRoutes();
  const addUserCard = (
    <ActionCard
      icon={<UserPlus size={18} aria-hidden />}
      title={t('applications:edit.overview.connect.signInHelp.addUser', 'Add a test user')}
      description={t('applications:edit.overview.connect.signInHelp.addUserHint', 'Opens Users')}
      to={routes.users.add()}
    />
  );

  return (
    <Stack spacing={1}>
      <Typography variant="body2" color="text.secondary">
        {isSelfSignUpEnabled
          ? t(
              'applications:edit.overview.connect.signInHelp.leadWithSignUp',
              'Need someone to sign in with? Sign up from your app, or add a user.',
            )
          : t('applications:edit.overview.connect.signInHelp.lead', 'Need someone to sign in with? Do either:')}
      </Typography>
      <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', sm: 'repeat(2, minmax(0, 1fr))'}, gap: 1.5}}>
        {isSelfSignUpEnabled ? (
          <Paper
            variant="outlined"
            sx={(theme) => ({
              display: 'flex',
              alignItems: 'center',
              gap: 1.5,
              px: 2,
              py: 1.5,
              borderColor: alpha(theme.palette.success.main, 0.4),
              bgcolor: alpha(theme.palette.success.main, 0.06),
            })}
          >
            <Box sx={{display: 'flex', color: 'success.main'}}>
              <Check size={18} aria-hidden />
            </Box>
            <Box sx={{minWidth: 0}}>
              <Typography variant="subtitle2">
                {t('applications:edit.overview.connect.signInHelp.signUpOn', 'Self sign-up is on')}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {t(
                  'applications:edit.overview.connect.signInHelp.signUpOnHint',
                  'Select Sign up on the sign-in page to create your user.',
                )}
              </Typography>
            </Box>
          </Paper>
        ) : (
          addUserCard
        )}
        {isSelfSignUpEnabled
          ? addUserCard
          : onGoToFlows && (
              <ActionCard
                icon={<LogIn size={18} aria-hidden />}
                title={t('applications:edit.overview.connect.signInHelp.turnOnSignUp', 'Turn on self sign-up')}
                description={t('applications:edit.overview.connect.signInHelp.turnOnSignUpHint', 'Opens the Flows tab')}
                onClick={onGoToFlows}
              />
            )}
      </Box>
    </Stack>
  );
}

/** The coding agent prompt, filled in for this application, with a copy button. */
function CodingAgentPrompt({
  label = undefined,
  promptUrl,
  promptValues,
  codeValues,
}: {
  label?: string;
  promptUrl: string;
  promptValues: Record<string, string | undefined>;
  codeValues: Record<string, string | undefined>;
}): JSX.Element {
  const {t} = useTranslation();
  const {copied, copy} = useCopyToClipboard();
  const [isExpanded, setIsExpanded] = useState(false);
  const {data: promptText, error, refetch} = useGetCodingAgentPrompt(promptUrl);
  // Beyond its `{{placeholder}}` slots, a prompt can write the same placeholders the docs code samples
  // do (a local instance's URL, a client ID to replace), so those are filled in the same way. They are
  // filled first, so a value filled into a slot is never scanned for them again.
  const prompt =
    promptText === undefined
      ? undefined
      : fillPrompt(
          fillQuickstartGuideCode(promptText, codeValues)
            .map((segment) => segment.text)
            .join(''),
          promptValues,
        );
  const promptLabel = label
    ? t('applications:edit.overview.connect.agent.promptLabel', '{{label}} prompt', {label})
    : t('applications:edit.overview.connect.agent.promptLabelGeneric', 'Coding agent prompt');

  if (error) {
    return (
      <QueryErrorNotice
        variant="inline"
        error={error}
        t={t}
        fallbackKey="applications:edit.overview.connect.agent.error"
        fallbackDefaultValue="The prompt could not be loaded."
        onRetry={() => {
          void refetch();
        }}
      />
    );
  }

  if (prompt === undefined) {
    return (
      <Box sx={{display: 'flex', justifyContent: 'center', py: 3}}>
        <CircularProgress
          size={24}
          aria-label={t('applications:edit.overview.connect.agent.loading', 'Loading the prompt')}
        />
      </Box>
    );
  }

  return (
    <Box sx={{border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'hidden'}}>
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        spacing={1}
        sx={{px: 2, py: 1, borderBottom: 1, borderColor: 'divider', bgcolor: 'action.hover'}}
      >
        <Typography variant="caption" color="text.secondary" sx={{fontFamily: 'monospace'}} noWrap>
          {promptLabel}
        </Typography>
        <Button
          variant="contained"
          size="small"
          startIcon={copied ? <Check size={14} aria-hidden /> : <Copy size={14} aria-hidden />}
          onClick={() => {
            void copy(prompt);
          }}
        >
          {copied
            ? t('common:actions.copied', 'Copied!')
            : t('applications:edit.overview.agentPrompt.action', 'Copy prompt')}
        </Button>
      </Stack>
      <Box
        component="pre"
        tabIndex={0}
        aria-label={promptLabel}
        sx={{
          m: 0,
          p: 2,
          typography: 'body2',
          fontFamily: 'monospace',
          whiteSpace: 'pre-wrap',
          overflowWrap: 'anywhere',
          // Collapsed, the prompt fades out into the card, whatever its color, rather than ending mid-line.
          ...(isExpanded
            ? {}
            : {maxHeight: 220, overflow: 'hidden', maskImage: 'linear-gradient(to bottom, black 55%, transparent)'}),
        }}
      >
        {prompt}
      </Box>
      <Box sx={{display: 'flex', justifyContent: 'center', pb: 1.5}}>
        {isExpanded ? (
          <Button size="small" startIcon={<ChevronUp size={14} aria-hidden />} onClick={() => setIsExpanded(false)}>
            {t('applications:edit.overview.connect.agent.collapse', 'Collapse prompt')}
          </Button>
        ) : (
          <Button size="small" startIcon={<ChevronDown size={14} aria-hidden />} onClick={() => setIsExpanded(true)}>
            {t('applications:edit.overview.connect.agent.expand', 'Expand prompt')}
          </Button>
        )}
      </Box>
    </Box>
  );
}

/**
 * Shows, on the application's Overview, how to connect an app to this application: by handing a
 * coding agent a ready-made prompt, or by hand, following the docs quickstart or running its
 * sample app. Every value the developer needs is already filled in. The guide, the prompt, and the
 * sample instructions all come from the docs site, so they follow the docs version the Console is
 * configured with.
 *
 * @param props - The component props
 * @returns The guide card
 *
 * @public
 */
export default function ConnectAppGuide({
  guides,
  promptUrl = undefined,
  promptValues,
  codeValues,
  devServerUrl = undefined,
  redirectUris = [],
  isSelfSignUpEnabled,
  isSignInFacing,
  onGoToFlows = undefined,
  onOpenLink,
}: ConnectAppGuideProps): JSX.Element {
  const {t} = useTranslation();
  const [selectedLabel, setSelectedLabel] = useState(guides[0]?.label);
  const guide = guides.find((entry) => entry.label === selectedLabel) ?? guides[0];
  // A template covering several technologies has a prompt per technology; one built for a single
  // technology has its own.
  const activePromptUrl = guide?.promptUrl ?? promptUrl;
  // Until someone picks a mode, the prompt decides it: agent setup when there is one. It can arrive after
  // the guide first shows, such as when the application's template loads later.
  const [mode, setMode] = useState<'agent' | 'manual' | null>(null);
  const [manualView, setManualView] = useState<'steps' | 'sample'>('steps');
  // Read as the guide shows it, so a guide limited to a section offers only that section's sample.
  const {data: guideContent} = useGetQuickstartGuide(guide?.docsUrl, guide?.section);
  const hasSample = Boolean(guideContent?.sample);
  const label = guide?.label;
  const activeMode = activePromptUrl ? (guide ? (mode ?? 'agent') : 'agent') : 'manual';
  const activeManualView = hasSample ? manualView : 'steps';
  const isDevServerAllowed = Boolean(devServerUrl && redirectUris.includes(devServerUrl));
  const signInHelp = isSignInFacing && (
    <SignInHelp isSelfSignUpEnabled={isSelfSignUpEnabled} onGoToFlows={onGoToFlows} />
  );

  return (
    <Paper variant="outlined" component="section" aria-labelledby="connect-app-guide-title" sx={{p: 3}}>
      <Stack spacing={3}>
        <Stack direction="row" alignItems="center" flexWrap="wrap" gap={2}>
          <Box
            sx={{
              width: 44,
              height: 44,
              borderRadius: '50%',
              border: 1,
              borderColor: 'divider',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
            }}
          >
            {guide?.icon ?? <BookOpen size={20} aria-hidden />}
          </Box>
          <Box sx={{flex: '1 1 240px', minWidth: 0}}>
            <Typography
              id="connect-app-guide-title"
              variant="h5"
              component="h2"
              // In a row, so a technology menu in the sentence lines up with the words around it.
              sx={{
                display: 'flex',
                alignItems: 'center',
                flexWrap: 'wrap',
                whiteSpace: 'pre-wrap',
                rowGap: 0.5,
                columnGap: 1,
              }}
            >
              {/* A template covering several technologies names the one in a menu that switches it, the first
                  chosen until another is. */}
              {guide && guides.length > 1 && (
                <Trans
                  i18nKey="applications:edit.overview.connect.titleWithChoice"
                  defaults="Connect your <choice/> app"
                  components={{
                    choice: (
                      // The technology the guide is for, in a menu of the others the template covers.
                      <GuideOptionMenu
                        id="connect-app-technology-menu"
                        menuLabel={t('applications:edit.overview.connect.technology', 'Technology')}
                        options={guides.map((entry) => ({value: entry.label, label: entry.label, icon: entry.icon}))}
                        value={guide.label}
                        onSelect={setSelectedLabel}
                      />
                    ),
                  }}
                />
              )}
              {guides.length <= 1 &&
                (label
                  ? t('applications:edit.overview.connect.title', 'Connect your {{label}} app', {label})
                  : t('applications:edit.overview.connect.titleGeneric', 'Connect your app'))}
            </Typography>
          </Box>
          {guide && (
            <Button
              size="small"
              endIcon={<ArrowUpRight size={14} aria-hidden />}
              // The product's own docs, so it opens without the leaving-the-console confirmation.
              component="a"
              href={guide.docsUrl}
              target="_blank"
              rel="noopener noreferrer"
            >
              {t('applications:edit.overview.connect.readGuide', 'Read the full guide')}
            </Button>
          )}
        </Stack>

        {activePromptUrl && guide && (
          <ToggleButtonGroup
            exclusive
            size="small"
            value={activeMode}
            onChange={(_, value: 'agent' | 'manual' | null) => {
              if (value) {
                setMode(value);
              }
            }}
            aria-label={t('applications:edit.overview.connect.mode', 'Setup mode')}
            sx={SEGMENTED_CONTROL_SX}
          >
            <ToggleButton value="agent">
              <Bot size={16} aria-hidden />
              {t('applications:edit.overview.connect.agentMode', 'Agent setup')}
            </ToggleButton>
            <ToggleButton value="manual">
              <BookOpen size={16} aria-hidden />
              {t('applications:edit.overview.connect.manualMode', 'Manual setup')}
            </ToggleButton>
          </ToggleButtonGroup>
        )}

        {activeMode === 'agent' && activePromptUrl && (
          <GuideStepList
            steps={[
              {
                key: 'prompt',
                title: t('applications:edit.overview.connect.agent.promptTitle', 'Prompt your coding agent'),
                content: (
                  <>
                    <Typography variant="body2" color="text.secondary">
                      {t(
                        'applications:edit.overview.connect.agent.promptDescription',
                        'Paste this into Claude Code, Cursor, Copilot, or any coding agent. It walks the agent through installing the SDK and wiring up sign-in for this application.',
                      )}
                    </Typography>
                    <CodingAgentPrompt
                      label={label}
                      promptUrl={activePromptUrl}
                      promptValues={promptValues}
                      codeValues={codeValues}
                    />
                  </>
                ),
              },
              {
                key: 'run',
                title: t('applications:edit.overview.connect.agent.runTitle', 'Run your app and sign in'),
                content: (
                  <>
                    <Typography variant="body2" color="text.secondary">
                      {devServerUrl
                        ? t(
                            'applications:edit.overview.connect.agent.runWithUrl',
                            'When the agent finishes, start your app, open {{url}}, and sign in.',
                            {url: devServerUrl},
                          )
                        : t(
                            'applications:edit.overview.connect.agent.run',
                            'When the agent finishes, run your app and sign in.',
                          )}
                    </Typography>
                    {isDevServerAllowed && (
                      <Chip
                        size="small"
                        color="success"
                        variant="outlined"
                        icon={<Check size={14} aria-hidden />}
                        label={t(
                          'applications:edit.overview.connect.agent.redirectAllowed',
                          '{{url}} is an allowed redirect URI',
                          {url: devServerUrl},
                        )}
                        sx={{alignSelf: 'flex-start'}}
                      />
                    )}
                    {signInHelp}
                  </>
                ),
              },
            ]}
          />
        )}

        {activeMode === 'manual' && guide && (
          <Stack spacing={3}>
            {hasSample && (
              <Box
                role="radiogroup"
                aria-label={t('applications:edit.overview.connect.manualPath', 'Manual setup path')}
                sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', sm: 'repeat(2, minmax(0, 1fr))'}, gap: 1.5}}
              >
                <ChoiceCard
                  icon={<ListOrdered size={20} aria-hidden />}
                  title={t('applications:edit.overview.connect.followSteps', 'Follow the steps')}
                  description={t(
                    'applications:edit.overview.connect.followStepsHint',
                    'Add the SDK to a new or existing {{label}} app.',
                    {label},
                  )}
                  isSelected={activeManualView === 'steps'}
                  onSelect={() => setManualView('steps')}
                />
                <ChoiceCard
                  icon={<Download size={20} aria-hidden />}
                  title={t('applications:edit.overview.connect.useSample', 'Use our sample')}
                  description={t(
                    'applications:edit.overview.connect.useSampleHint',
                    'Download a working app, then give it the values for this application below.',
                  )}
                  isSelected={activeManualView === 'sample'}
                  onSelect={() => setManualView('sample')}
                />
              </Box>
            )}
            <QuickstartGuide guide={guide} codeValues={codeValues} onOpenLink={onOpenLink} view={activeManualView} />
            {signInHelp}
          </Stack>
        )}
      </Stack>
    </Paper>
  );
}
