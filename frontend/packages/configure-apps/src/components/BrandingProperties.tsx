// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AppImage, AppLink} from '@thunderid/system-apps';
import {
  Box,
  Button,
  Chip,
  Divider,
  FormControlLabel,
  IconButton,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {AppWindow, Image, PanelBottom, Plus, Type, X} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import ConsoleAttribution from './ConsoleAttribution';
import type {BuilderAppConfig} from '../models/builder';
import type {AppConfigAction} from '../utils/appConfigHistory';

export interface BrandingPropertiesProps {
  /** The logo of the application the app signs in with, which the app shows by default. */
  applicationLogoUrl?: string;
  config: BuilderAppConfig;
  dispatch: (action: AppConfigAction) => void;
}

type ImageKey = 'favicon' | 'logo';
type Variant = keyof AppImage;

/** Whether a URL is one the server accepts for a branding image: absolute http or https. */
const isImageUrl = (value: string): boolean => {
  try {
    const url = new URL(value);

    return url.protocol === 'https:' || url.protocol === 'http:';
  } catch {
    return false;
  }
};

/** The heading of a branding section: its icon and name. */
function SectionHeading({icon, title}: {icon: ReactNode; title: string}): JSX.Element {
  return (
    <Stack direction="row" alignItems="center" spacing={1} sx={{color: 'text.secondary'}}>
      {icon}
      <Typography variant="subtitle2" fontWeight={600} color="text.primary">
        {title}
      </Typography>
    </Stack>
  );
}

/** An image drawn on the background of the color scheme it is for. */
function ImageTile({size, src, variant}: {size: number; src: string | undefined; variant: Variant}): JSX.Element {
  return (
    <Box
      aria-hidden
      sx={{
        width: 72,
        height: 48,
        flexShrink: 0,
        display: 'grid',
        placeItems: 'center',
        borderRadius: 1,
        border: 1,
        borderColor: 'divider',
        bgcolor: variant === 'dark' ? '#11151c' : '#ffffff',
      }}
    >
      {src && <Box component="img" src={src} alt="" sx={{maxWidth: 60, maxHeight: size, objectFit: 'contain'}} />}
    </Box>
  );
}

/**
 * A URL for one color scheme's image, with a preview. Typed text applies once it is a URL the
 * server accepts; clearing it leaves that scheme to its fallback.
 */
function ImageField({
  fallbackSrc,
  help,
  label,
  onChange,
  size,
  value,
  variant,
}: {
  /** What the tile shows while the field is empty. */
  fallbackSrc: string | undefined;
  help: string;
  label: string;
  onChange: (value: string | undefined) => void;
  size: number;
  value: string | undefined;
  variant: Variant;
}): JSX.Element {
  const {t} = useTranslation('apps');
  const [draft, setDraft] = useState<string>(value ?? '');
  const [invalid, setInvalid] = useState<boolean>(false);

  const commit = (text: string): void => {
    const trimmed: string = text.trim();
    if (!trimmed) {
      setInvalid(false);
      onChange(undefined);
    } else if (isImageUrl(trimmed)) {
      setInvalid(false);
      onChange(trimmed);
    } else {
      setInvalid(true);
    }
  };

  return (
    <Stack direction="row" spacing={1.5} alignItems="flex-start">
      <ImageTile src={value ?? fallbackSrc} size={size} variant={variant} />
      <TextField
        size="small"
        fullWidth
        label={label}
        value={draft}
        placeholder="https://"
        error={invalid}
        helperText={invalid ? t('builder.branding.invalidUrl', 'Use an http or https URL.') : help}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={(event) => commit(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter') commit((event.target as HTMLInputElement).value);
        }}
        slotProps={{htmlInput: {spellCheck: false, inputMode: 'url'}}}
      />
    </Stack>
  );
}

/**
 * One branding image: the default the app takes, or a light and a dark image of the app's own.
 */
function ImageOverride({
  defaultCard,
  dispatch,
  image,
  labels,
  size,
  value,
}: {
  defaultCard: ReactNode;
  dispatch: BrandingPropertiesProps['dispatch'];
  image: ImageKey;
  labels: {dark: string; darkHelp: string; light: string; lightHelp: string; same: string};
  size: number;
  value: AppImage | undefined;
}): JSX.Element {
  // Turned off with nothing typed yet: the fields show, and the default stays until a URL is set.
  const [ownPending, setOwnPending] = useState<boolean>(false);
  const isOwn: boolean = Boolean(value) || ownPending;

  const setVariant = (variant: Variant, url: string | undefined): void => {
    const next: AppImage = {...value, [variant]: url};
    const isEmpty: boolean = !next.light && !next.dark;
    setOwnPending(isEmpty);
    dispatch({image, type: 'setBrandingImage', value: isEmpty ? undefined : next});
  };

  return (
    <Stack spacing={1.5}>
      <FormControlLabel
        labelPlacement="start"
        sx={{justifyContent: 'space-between', ml: 0}}
        label={labels.same}
        control={
          <Switch
            checked={!isOwn}
            onChange={() => {
              if (isOwn) {
                setOwnPending(false);
                dispatch({image, type: 'setBrandingImage', value: undefined});
              } else {
                setOwnPending(true);
              }
            }}
          />
        }
      />
      {isOwn ? (
        <Stack spacing={1.5}>
          <ImageField
            // Starts the typed text over whenever the value changes from elsewhere (undo, redo).
            key={`light-${value?.light ?? ''}`}
            variant="light"
            label={labels.light}
            help={labels.lightHelp}
            value={value?.light}
            fallbackSrc={undefined}
            size={size}
            onChange={(url: string | undefined) => setVariant('light', url)}
          />
          <ImageField
            key={`dark-${value?.dark ?? ''}`}
            variant="dark"
            label={labels.dark}
            help={labels.darkHelp}
            value={value?.dark}
            fallbackSrc={value?.light}
            size={size}
            onChange={(url: string | undefined) => setVariant('dark', url)}
          />
        </Stack>
      ) : (
        defaultCard
      )}
    </Stack>
  );
}

/** What the app shows by default, linked to where it comes from. */
function DefaultCard({caption, preview, title}: {caption: string; preview: ReactNode; title: string}): JSX.Element {
  const {t} = useTranslation('apps');

  return (
    <Stack
      direction="row"
      alignItems="center"
      spacing={1.5}
      sx={{p: 1, border: 1, borderStyle: 'dashed', borderColor: 'divider', borderRadius: 1.5, bgcolor: 'action.hover'}}
    >
      {preview}
      <Box sx={{flex: 1, minWidth: 0}}>
        <Typography variant="body2" fontWeight={500} noWrap>
          {title}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {caption}
        </Typography>
      </Box>
      <Chip size="small" variant="outlined" color="primary" label={t('builder.appearance.theme.linked', 'Linked')} />
    </Stack>
  );
}

const MAX_LINKS = 5;

/** The app's name, applied when the field is left. Cleared, the app shows its own name. */
function AppName({
  dispatch,
  title,
}: {
  dispatch: BrandingPropertiesProps['dispatch'];
  title: string | undefined;
}): JSX.Element {
  const {t} = useTranslation('apps');
  const [draft, setDraft] = useState<string>(title ?? '');

  const commit = (): void => {
    const trimmed: string = draft.trim();
    if (trimmed !== (title ?? '')) {
      dispatch({title: trimmed || undefined, type: 'setTitle'});
    }
  };

  return (
    <TextField
      size="small"
      fullWidth
      label={t('builder.branding.name', 'App name')}
      value={draft}
      placeholder={t('manage:title', 'Admin')}
      helperText={t('builder.branding.name.help', 'Shown in the top bar and the browser tab.')}
      onChange={(event) => setDraft(event.target.value)}
      onBlur={commit}
      onKeyDown={(event) => {
        if (event.key === 'Enter') commit();
      }}
      slotProps={{htmlInput: {maxLength: 60}}}
    />
  );
}

/**
 * The links at the foot of the app's pages. A link applies once it has a label and a URL the server
 * accepts; one still being typed stays here until then.
 */
function FooterLinks({
  dispatch,
  links,
}: {
  dispatch: BrandingPropertiesProps['dispatch'];
  links: AppLink[];
}): JSX.Element {
  const {t} = useTranslation('apps');
  const [drafts, setDrafts] = useState<AppLink[]>(links);
  const [touched, setTouched] = useState<boolean[]>([]);
  const linksKey: string = JSON.stringify(links);
  // The links as last seen, and as last applied from here. Links that change from elsewhere (undo,
  // redo) start the drafts over; the ones applied from here keep the links still being typed.
  const [seenKey, setSeenKey] = useState<string>(linksKey);
  const [appliedKey, setAppliedKey] = useState<string>(linksKey);
  if (linksKey !== seenKey) {
    setSeenKey(linksKey);
    if (linksKey !== appliedKey) {
      setDrafts(links);
      setTouched([]);
      setAppliedKey(linksKey);
    }
  }

  const apply = (next: AppLink[]): void => {
    setDrafts(next);
    const complete: AppLink[] = next
      .map((link: AppLink) => ({label: link.label.trim(), url: link.url.trim()}))
      .filter((link: AppLink) => link.label && isImageUrl(link.url));
    const completeKey: string = JSON.stringify(complete);
    if (completeKey !== linksKey) {
      setAppliedKey(completeKey);
      dispatch({links: complete, type: 'setLinks'});
    }
  };

  const update = (index: number, patch: Partial<AppLink>): void =>
    setDrafts((current: AppLink[]) => current.map((link, i) => (i === index ? {...link, ...patch} : link)));

  return (
    <Stack spacing={1}>
      <Typography variant="body2" fontWeight={500}>
        {t('builder.branding.links', 'Links')}
      </Typography>
      {drafts.length === 0 && (
        <Typography variant="caption" color="text.secondary">
          {t('builder.branding.links.empty', 'No links yet. Add privacy, terms or support pages.')}
        </Typography>
      )}
      {drafts.map((link: AppLink, index: number) => {
        const urlInvalid: boolean = Boolean(touched[index]) && Boolean(link.url.trim()) && !isImageUrl(link.url.trim());

        return (
          // eslint-disable-next-line react/no-array-index-key -- links have no identity of their own
          <Stack key={index} direction="row" spacing={1} alignItems="flex-start">
            <TextField
              size="small"
              label={t('builder.branding.links.label', 'Label')}
              value={link.label}
              onChange={(event) => update(index, {label: event.target.value})}
              onBlur={() => apply(drafts)}
              slotProps={{htmlInput: {maxLength: 60}}}
              sx={{width: 110, flexShrink: 0}}
            />
            <TextField
              size="small"
              fullWidth
              label={t('builder.branding.links.url', 'URL')}
              value={link.url}
              placeholder="https://"
              error={urlInvalid}
              helperText={urlInvalid ? t('builder.branding.invalidUrl', 'Use an http or https URL.') : undefined}
              onChange={(event) => update(index, {url: event.target.value})}
              onBlur={() => {
                setTouched((current: boolean[]) => Object.assign([...current], {[index]: true}));
                apply(drafts);
              }}
              slotProps={{htmlInput: {spellCheck: false, inputMode: 'url'}}}
            />
            <IconButton
              size="small"
              aria-label={t('builder.branding.links.remove', 'Remove {{link}}', {
                link: link.label || t('builder.branding.links.link', 'link'),
              })}
              onClick={() => {
                setTouched((current: boolean[]) => current.filter((_value, i) => i !== index));
                apply(drafts.filter((_link, i) => i !== index));
              }}
              sx={{mt: 0.5}}
            >
              <X size={16} />
            </IconButton>
          </Stack>
        );
      })}
      {drafts.length < MAX_LINKS && (
        <Button
          size="small"
          variant="outlined"
          startIcon={<Plus size={14} />}
          onClick={() => setDrafts([...drafts, {label: '', url: ''}])}
          sx={{alignSelf: 'flex-start'}}
        >
          {t('builder.branding.links.add', 'Add link')}
        </Button>
      )}
    </Stack>
  );
}

/**
 * The app's logo and favicon. Each defaults to what the app already has, the application's
 * logo and the app's own icon, and can be replaced with a light and a dark image of its own.
 */
export default function BrandingProperties({
  applicationLogoUrl = undefined,
  config,
  dispatch,
}: BrandingPropertiesProps): JSX.Element {
  const {t} = useTranslation('apps');
  const darkHelp: string = t('builder.branding.darkHelp', 'Optional. The light one shows when empty.');

  return (
    <Stack spacing={3}>
      <Stack spacing={1.5} component="section" aria-label={t('builder.branding.logo', 'Logo')}>
        <SectionHeading icon={<Image size={16} />} title={t('builder.branding.logo', 'Logo')} />
        <ImageOverride
          image="logo"
          value={config.branding.logo}
          dispatch={dispatch}
          size={32}
          labels={{
            same: t('builder.branding.logo.same', 'Same logo as the application'),
            light: t('builder.branding.light', 'Light'),
            lightHelp: t('builder.branding.logo.lightHelp', 'Shown on light backgrounds.'),
            dark: t('builder.branding.dark', 'Dark'),
            darkHelp,
          }}
          defaultCard={
            <DefaultCard
              preview={<ImageTile src={applicationLogoUrl} size={32} variant="light" />}
              title={t('builder.branding.logo.default', 'Application logo')}
              caption={
                applicationLogoUrl
                  ? t('builder.branding.logo.defaultHelp', 'The logo the Gate shows, set on the application.')
                  : t('builder.branding.logo.none', 'The application has no logo, so the app shows its name only.')
              }
            />
          }
        />
      </Stack>
      <Divider />
      <Stack spacing={1.5} component="section" aria-label={t('builder.branding.nameSection', 'Name')}>
        <SectionHeading icon={<Type size={16} />} title={t('builder.branding.nameSection', 'Name')} />
        <AppName
          // Starts the typed text over whenever the name changes from elsewhere (undo, redo).
          key={config.branding.title ?? ''}
          title={config.branding.title}
          dispatch={dispatch}
        />
      </Stack>
      <Divider />
      <Stack spacing={1.5} component="section" aria-label={t('builder.branding.favicon', 'Favicon')}>
        <SectionHeading icon={<AppWindow size={16} />} title={t('builder.branding.favicon', 'Favicon')} />
        <ImageOverride
          image="favicon"
          value={config.branding.favicon}
          dispatch={dispatch}
          size={24}
          labels={{
            same: t('builder.branding.favicon.same', "Same icon as the app's own"),
            light: t('builder.branding.light', 'Light'),
            lightHelp: t('builder.branding.favicon.lightHelp', 'Shown when the browser uses a light theme.'),
            dark: t('builder.branding.dark', 'Dark'),
            darkHelp,
          }}
          defaultCard={
            <DefaultCard
              preview={<ImageTile src={undefined} size={24} variant="light" />}
              title={t('builder.branding.favicon.default', "The app's own icon")}
              caption={t('builder.branding.favicon.defaultHelp', 'Set where the app is deployed, in its config.js.')}
            />
          }
        />
      </Stack>
      <Divider />
      <Stack spacing={1.5} component="section" aria-label={t('builder.branding.footer', 'Footer')}>
        <SectionHeading icon={<PanelBottom size={16} />} title={t('builder.branding.footer', 'Footer')} />
        <FormControlLabel
          labelPlacement="start"
          sx={{justifyContent: 'space-between', ml: 0}}
          label={
            <Stack direction="row" alignItems="center" spacing={1}>
              <Typography variant="body2">{t('builder.appearance.attribution.show', 'Show')}</Typography>
              <ConsoleAttribution />
            </Stack>
          }
          control={
            <Switch
              checked={config.branding.showSecuredBy}
              onChange={() => dispatch({type: 'toggleSecuredBy'})}
              slotProps={{input: {'aria-label': t('builder.appearance.securedBy', 'Show "Secured by ThunderID"')}}}
            />
          }
        />
        <FooterLinks links={config.branding.links ?? []} dispatch={dispatch} />
      </Stack>
    </Stack>
  );
}
