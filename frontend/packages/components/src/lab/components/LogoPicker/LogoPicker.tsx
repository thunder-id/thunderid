/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {
  AVATAR_URI_SCHEME as AVATAR_SCHEME,
  EMOJI_URI_SCHEME as EMOJI_SCHEME,
  extractAvatarParamsFromUri as parseAvatarSpec,
  AVATAR_GRADIENT_COUNT,
  ANONYMOUS_ANIMAL_ICONS,
  ANONYMOUS_ENTITY_ICONS,
  buildAvatarSpec,
  deriveAvatarContent,
  generateAvatarDataUri,
  pickAnonymousAvatarName,
  pickAnonymousEntityName,
  type AvatarParams,
  type AvatarShape,
} from '@thunderid/react';
import {isAbsoluteUrl as isUrl} from '@thunderid/utils';
import {
  Box,
  Button,
  Dialog,
  DialogContent,
  DialogTitle,
  FormHelperText,
  IconButton,
  Popover,
  Stack,
  Tab,
  Tabs,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {Link2, Plus, Shuffle, X} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useEffect, useMemo, useRef, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import AvatarSwatchGrid from '../AvatarPicker/AvatarSwatchGrid';
import EmojiPicker from '../EmojiPicker/EmojiPicker';
import generateIconSuggestions from '../EmojiPicker/utils/generateIconSuggestions';
import IconGridPicker from '../IconGridPicker/IconGridPicker';
import {resolveResourceIcon} from '../resourceIconSchemes';

const VARIANT_COUNT = 4;
const URL_COMMIT_DEBOUNCE_MS = 500;

function schemeOf(value: string): 'avatar' | 'emoji' | 'url' {
  if (value.startsWith(AVATAR_SCHEME)) return 'avatar';
  if (isUrl(value)) return 'url';
  return 'emoji';
}

function isCompleteUrl(value: string): boolean {
  if (!isUrl(value)) return false;
  try {
    const parsed = new URL(value);
    return Boolean(parsed);
  } catch {
    return false;
  }
}

function defaultAvatarParams(seedText: string, shape: AvatarShape): AvatarParams {
  return {colors: 0, content: deriveAvatarContent('two_letter', seedText), shape, variant: 'two_letter'};
}

function randomGradientIndex(): number {
  return Math.floor(Math.random() * AVATAR_GRADIENT_COUNT);
}

type GroupKey = 'emoji' | 'rounded_blank' | 'rounded_text' | 'circle_blank' | 'circle_text' | 'animal' | 'entity';
type TextLetterVariant = 'one_letter' | 'two_letter';

const TEXT_LETTER_VARIANTS: {label: string; value: TextLetterVariant}[] = [
  {label: 'Two letters', value: 'two_letter'},
  {label: 'One letter', value: 'one_letter'},
];

const ANIMAL_SHAPES: {label: string; value: AvatarShape}[] = [
  {label: 'Rounded', value: 'rounded'},
  {label: 'Circle', value: 'circle'},
];

const compactTabsSx = {
  minHeight: 36,
  px: 0.5,
  borderBottom: '1px solid',
  borderColor: 'divider',
  '& .MuiTabs-indicator': {height: 2},
  // Extra specificity (vs. plain `.MuiTab-root`) so this reliably outranks MUI's own
  // `.MuiButtonBase-root.MuiTab-root` base rule regardless of stylesheet insertion order.
  '& .MuiButtonBase-root.MuiTab-root': {minHeight: 36, minWidth: 0, flex: 1, p: 0.5, fontSize: '0.6875rem'},
};

/**
 * Props for the {@link LogoPicker} component.
 *
 * @public
 */
export interface LogoPickerProps {
  /**
   * The current logo spec — `emoji:<char>`, `avatar:shape=...,variant=...,content=...,colors=...,bg=...`,
   * or a raw image URL.
   */
  value: string;

  /**
   * Fired whenever the user picks a logo.
   */
  onChange: (value: string) => void;

  /**
   * Seed text used to default the generated avatar's initials (typically the app name).
   */
  seedText?: string;

  /**
   * Number of options to show at once in each avatar flyout's grid (a random sample re-drawn
   * on shuffle).
   *
   * @defaultValue 5
   */
  optionCount?: number;

  /**
   * Avatar shapes the consuming feature accepts. Groups for an unsupported shape (e.g. Circle,
   * Circle text) are hidden entirely, and the shape switcher inside the Animal/Entity flyouts
   * only offers supported shapes.
   *
   * @defaultValue ['rounded']
   */
  supportedShapes?: AvatarShape[];
}

/**
 * An inline logo picker: a preview tile + custom image URL field, plus grouped tiles that
 * expand into a variant flyout — Emoji, Rounded/Circular blank avatars, Rounded/Circular
 * lettered avatars, a curated Anonymous animal, and a curated Entity icon set (applications,
 * organizations, resource servers). Each avatar flyout shows a shuffleable grid of the same
 * avatar rendered with different backgrounds ({@link AvatarSwatchGrid} for the gradient
 * variants, {@link IconGridPicker} for the animal/entity icons), matching the Emoji group's
 * pick-a-tile interaction. The Emoji group's flyout also opens the full {@link EmojiPicker}
 * via a "+" tile.
 *
 * @public
 */
export default function LogoPicker({
  value,
  onChange,
  seedText = '',
  optionCount = 5,
  supportedShapes = ['rounded'],
}: LogoPickerProps): JSX.Element {
  const {t} = useTranslation('elements');
  const scheme = schemeOf(value);
  const supportsRounded: boolean = supportedShapes.includes('rounded');
  const supportsCircle: boolean = supportedShapes.includes('circle');
  const defaultShape: AvatarShape = supportsRounded ? 'rounded' : 'circle';
  const availableShapes = useMemo(
    (): {label: string; value: AvatarShape}[] =>
      ANIMAL_SHAPES.filter(({value: shapeValue}) => supportedShapes.includes(shapeValue)),
    [supportedShapes],
  );

  const [openGroup, setOpenGroup] = useState<GroupKey | null>(null);
  const [emojiOptions, setEmojiOptions] = useState<string[]>((): string[] => {
    const suggestions = generateIconSuggestions(VARIANT_COUNT);
    if (scheme !== 'emoji') return suggestions;
    const current = value.slice(EMOJI_SCHEME.length);
    return current && !suggestions.includes(current)
      ? [current, ...suggestions.slice(0, VARIANT_COUNT - 1)]
      : suggestions;
  });
  const [urlInput, setUrlInput] = useState<string>(() => (scheme === 'url' ? value : ''));
  const [emojiDialogOpen, setEmojiDialogOpen] = useState<boolean>(false);

  const [roundedBlankShuffle, setRoundedBlankShuffle] = useState<number>(0);
  const [roundedTextShuffle, setRoundedTextShuffle] = useState<number>(0);
  const [circleBlankShuffle, setCircleBlankShuffle] = useState<number>(0);
  const [circleTextShuffle, setCircleTextShuffle] = useState<number>(0);
  const [animalShuffle, setAnimalShuffle] = useState<number>(0);
  const [entityShuffle, setEntityShuffle] = useState<number>(0);

  const currentAvatarParams: AvatarParams = useMemo(
    (): AvatarParams => (scheme === 'avatar' ? parseAvatarSpec(value) : defaultAvatarParams(seedText, defaultShape)),
    [scheme, value, seedText, defaultShape],
  );

  const isRoundedBlank: boolean =
    scheme === 'avatar' && currentAvatarParams.shape === 'rounded' && currentAvatarParams.variant === 'blank';
  const isRoundedText: boolean =
    scheme === 'avatar' &&
    currentAvatarParams.shape === 'rounded' &&
    (currentAvatarParams.variant === 'one_letter' || currentAvatarParams.variant === 'two_letter');
  const isCircleBlank: boolean =
    scheme === 'avatar' && currentAvatarParams.shape === 'circle' && currentAvatarParams.variant === 'blank';
  const isCircleText: boolean =
    scheme === 'avatar' &&
    currentAvatarParams.shape === 'circle' &&
    (currentAvatarParams.variant === 'one_letter' || currentAvatarParams.variant === 'two_letter');
  const isAnimal: boolean = scheme === 'avatar' && currentAvatarParams.variant === 'anonymous_animal';
  const isEntity: boolean = scheme === 'avatar' && currentAvatarParams.variant === 'anonymous_entity';

  // Sub-choices not captured by the committed value alone (which letter count, which animal/entity
  // shape/icon to preview) — seeded once with a random background/icon so the group tiles show
  // a varied set at a glance, and kept in sync whenever the committed value already matches.
  const [textVariant, setTextVariant] = useState<TextLetterVariant>(
    currentAvatarParams.variant === 'one_letter' ? 'one_letter' : 'two_letter',
  );
  const [animalShape, setAnimalShape] = useState<AvatarShape>(isAnimal ? currentAvatarParams.shape : defaultShape);
  const [animalName, setAnimalName] = useState<string>(
    isAnimal ? currentAvatarParams.content : pickAnonymousAvatarName(),
  );
  const [entityShape, setEntityShape] = useState<AvatarShape>(isEntity ? currentAvatarParams.shape : defaultShape);
  const [entityName, setEntityName] = useState<string>(
    isEntity ? currentAvatarParams.content : pickAnonymousEntityName(),
  );
  const [tilePreviewColors] = useState<
    Record<'rounded_blank' | 'rounded_text' | 'circle_blank' | 'circle_text', number>
  >(() => ({
    circle_blank: randomGradientIndex(),
    circle_text: randomGradientIndex(),
    rounded_blank: randomGradientIndex(),
    rounded_text: randomGradientIndex(),
  }));

  const preview = resolveResourceIcon(value, seedText);
  const textContent: string = deriveAvatarContent(textVariant, seedText);

  const showShapeQualifier: boolean = availableShapes.length > 1;
  const avatarLabel: string = t('logo_picker.content_type.avatar', 'Avatar');
  const textAvatarLabel: string = t('logo_picker.content_type.text_avatar', 'Text Avatar');
  const roundedShapeName: string = t('logo_picker.shapes.rounded', 'Rounded');
  const circleShapeName: string = t('logo_picker.shapes.circle', 'Circle');
  const roundedBlankLabel: string = showShapeQualifier ? `${roundedShapeName} ${avatarLabel}` : avatarLabel;
  const roundedTextLabel: string = showShapeQualifier ? `${roundedShapeName} ${textAvatarLabel}` : textAvatarLabel;
  const circleBlankLabel: string = showShapeQualifier ? `${circleShapeName} ${avatarLabel}` : avatarLabel;
  const circleTextLabel: string = showShapeQualifier ? `${circleShapeName} ${textAvatarLabel}` : textAvatarLabel;

  const toggleGroup = useCallback((key: GroupKey): void => {
    setOpenGroup((prev) => (prev === key ? null : key));
  }, []);

  const urlCommitTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    (): (() => void) => (): void => {
      if (urlCommitTimeoutRef.current) clearTimeout(urlCommitTimeoutRef.current);
    },
    [],
  );

  const commitUrl = useCallback(
    (url: string): void => {
      if (isCompleteUrl(url)) onChange(url);
    },
    [onChange],
  );

  const handleUrlChange = useCallback(
    (url: string): void => {
      setUrlInput(url);
      if (urlCommitTimeoutRef.current) clearTimeout(urlCommitTimeoutRef.current);
      urlCommitTimeoutRef.current = setTimeout(() => commitUrl(url), URL_COMMIT_DEBOUNCE_MS);
    },
    [commitUrl],
  );

  const handleUrlBlur = useCallback((): void => {
    if (urlCommitTimeoutRef.current) {
      clearTimeout(urlCommitTimeoutRef.current);
      urlCommitTimeoutRef.current = null;
    }
    commitUrl(urlInput);
  }, [commitUrl, urlInput]);

  const handlePickEmoji = useCallback(
    (char: string): void => {
      onChange(EMOJI_SCHEME + char);
      setEmojiDialogOpen(false);
    },
    [onChange],
  );

  const handleShuffleEmoji = useCallback((): void => {
    const next: string[] = generateIconSuggestions(VARIANT_COUNT);
    setEmojiOptions(next);
    onChange(EMOJI_SCHEME + next[0]);
  }, [onChange]);

  const handlePickGradientSwatch = useCallback(
    (shape: AvatarShape, variant: 'blank' | TextLetterVariant, content: string, colors: number): void => {
      onChange(buildAvatarSpec({colors, content, shape, variant}));
    },
    [onChange],
  );

  const handlePickAnimal = useCallback(
    (name: string): void => {
      setAnimalName(name);
      onChange(buildAvatarSpec({colors: 0, content: name, shape: animalShape, variant: 'anonymous_animal'}));
    },
    [animalShape, onChange],
  );

  const handlePickEntity = useCallback(
    (name: string): void => {
      setEntityName(name);
      onChange(buildAvatarSpec({colors: 0, content: name, shape: entityShape, variant: 'anonymous_entity'}));
    },
    [entityShape, onChange],
  );

  const emojiTileGlyph: string = scheme === 'emoji' ? value.slice(EMOJI_SCHEME.length) : emojiOptions[0];

  const roundedBlankTileSrc: string = generateAvatarDataUri({
    colors: isRoundedBlank ? currentAvatarParams.colors : tilePreviewColors.rounded_blank,
    content: '',
    shape: 'rounded',
    variant: 'blank',
  });
  const roundedTextTileSrc: string = generateAvatarDataUri({
    colors: isRoundedText ? currentAvatarParams.colors : tilePreviewColors.rounded_text,
    content: isRoundedText ? currentAvatarParams.content : textContent,
    shape: 'rounded',
    variant: isRoundedText ? currentAvatarParams.variant : textVariant,
  });
  const circleBlankTileSrc: string = generateAvatarDataUri({
    colors: isCircleBlank ? currentAvatarParams.colors : tilePreviewColors.circle_blank,
    content: '',
    shape: 'circle',
    variant: 'blank',
  });
  const circleTextTileSrc: string = generateAvatarDataUri({
    colors: isCircleText ? currentAvatarParams.colors : tilePreviewColors.circle_text,
    content: isCircleText ? currentAvatarParams.content : textContent,
    shape: 'circle',
    variant: isCircleText ? currentAvatarParams.variant : textVariant,
  });
  const animalTileSrc: string = generateAvatarDataUri({
    colors: 0,
    content: isAnimal ? currentAvatarParams.content : animalName,
    shape: isAnimal ? currentAvatarParams.shape : animalShape,
    variant: 'anonymous_animal',
  });
  const entityTileSrc: string = generateAvatarDataUri({
    colors: 0,
    content: isEntity ? currentAvatarParams.content : entityName,
    shape: isEntity ? currentAvatarParams.shape : entityShape,
    variant: 'anonymous_entity',
  });

  const animalIcons: Record<string, string> = useMemo(
    (): Record<string, string> =>
      Object.fromEntries(
        Object.keys(ANONYMOUS_ANIMAL_ICONS).map((name) => [
          name,
          generateAvatarDataUri({
            colors: 0,
            content: name,
            shape: isAnimal ? currentAvatarParams.shape : animalShape,
            variant: 'anonymous_animal',
          }),
        ]),
      ),
    [isAnimal, currentAvatarParams.shape, animalShape],
  );

  const entityIcons: Record<string, string> = useMemo(
    (): Record<string, string> =>
      Object.fromEntries(
        Object.keys(ANONYMOUS_ENTITY_ICONS).map((name) => [
          name,
          generateAvatarDataUri({
            colors: 0,
            content: name,
            shape: isEntity ? currentAvatarParams.shape : entityShape,
            variant: 'anonymous_entity',
          }),
        ]),
      ),
    [isEntity, currentAvatarParams.shape, entityShape],
  );

  return (
    <Stack spacing={2.5}>
      {/* Preview tile + URL */}
      <Stack direction="row" spacing={1.75} alignItems="stretch">
        <Box
          sx={{
            width: 76,
            height: 76,
            borderRadius: scheme === 'avatar' ? (currentAvatarParams.shape === 'circle' ? '50%' : '22%') : 2.25,
            flexShrink: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            overflow: 'hidden',
            border: '1px solid',
            borderColor: 'divider',
            bgcolor: 'action.hover',
            fontSize: 34,
          }}
        >
          {preview.type === 'emoji' ? (
            preview.char
          ) : (
            <img src={preview.src} alt="" style={{width: '100%', height: '100%', objectFit: 'cover'}} />
          )}
        </Box>
        <Stack spacing={0.75} flex={1} justifyContent="center">
          <TextField
            fullWidth
            size="small"
            value={urlInput}
            onChange={(e) => handleUrlChange(e.target.value)}
            onBlur={handleUrlBlur}
            placeholder={t('logo_picker.url.placeholder', 'Paste an image URL, e.g. https://example.com/logo.png')}
            slotProps={{input: {startAdornment: <Link2 size={16} style={{marginRight: 8, opacity: 0.5}} />}}}
          />
          <FormHelperText>
            {t(
              'logo_picker.url.helper_text',
              'Direct link to a PNG, SVG or JPG. For best results, use a square image less than 1MB in size.',
            )}
          </FormHelperText>
        </Stack>
      </Stack>

      <Stack direction="row" alignItems="center" spacing={1.5}>
        <Box sx={{flex: 1, height: '1px', bgcolor: 'divider'}} />
        <Typography variant="caption" color="text.secondary" sx={{letterSpacing: '0.04em'}}>
          {t('logo_picker.divider', 'OR PICK ONE')}
        </Typography>
        <Box sx={{flex: 1, height: '1px', bgcolor: 'divider'}} />
      </Stack>

      {/* Group tiles */}
      <Stack direction="row" spacing={1.75} sx={{flexWrap: 'wrap', rowGap: 2}}>
        <GroupTile
          groupKey="entity"
          openGroup={openGroup}
          onToggle={toggleGroup}
          selected={isEntity}
          shape={isEntity ? currentAvatarParams.shape : entityShape}
          label={t('logo_picker.groups.entity', 'Entity')}
          flyoutLabel={t('logo_picker.flyouts.entity', 'Choose an icon')}
          flyoutLayout="stack"
          onShuffle={() => setEntityShuffle((n) => n + 1)}
          tile={<img src={entityTileSrc} alt="" style={{width: '100%', height: '100%', objectFit: 'cover'}} />}
        >
          {availableShapes.length > 1 && (
            <Tabs
              value={entityShape}
              onChange={(_, shapeValue: AvatarShape) => setEntityShape(shapeValue)}
              variant="fullWidth"
              sx={compactTabsSx}
            >
              {availableShapes.map(({label, value: shapeValue}) => (
                <Tab key={shapeValue} value={shapeValue} label={t(`logo_picker.entity_shapes.${shapeValue}`, label)} />
              ))}
            </Tabs>
          )}
          <IconGridPicker
            shuffleSignal={entityShuffle}
            icons={entityIcons}
            value={isEntity ? currentAvatarParams.content : entityName}
            shape={isEntity ? currentAvatarParams.shape : entityShape}
            optionCount={optionCount}
            showShuffle={false}
            onChange={handlePickEntity}
          />
        </GroupTile>

        {supportsRounded && (
          <GroupTile
            groupKey="rounded_blank"
            openGroup={openGroup}
            onToggle={toggleGroup}
            selected={isRoundedBlank}
            label={roundedBlankLabel}
            flyoutLabel={t('logo_picker.flyouts.rounded_blank', 'Pick a background')}
            flyoutLayout="stack"
            onShuffle={() => setRoundedBlankShuffle((n) => n + 1)}
            tile={<img src={roundedBlankTileSrc} alt="" style={{width: '100%', height: '100%', objectFit: 'cover'}} />}
          >
            <AvatarSwatchGrid
              shuffleSignal={roundedBlankShuffle}
              base={{content: '', shape: 'rounded', variant: 'blank'}}
              value={isRoundedBlank ? currentAvatarParams.colors : -1}
              gradientCount={AVATAR_GRADIENT_COUNT}
              optionCount={optionCount}
              showShuffle={false}
              onChange={(colors) => handlePickGradientSwatch('rounded', 'blank', '', colors)}
            />
          </GroupTile>
        )}

        {supportsRounded && (
          <GroupTile
            groupKey="rounded_text"
            openGroup={openGroup}
            onToggle={toggleGroup}
            selected={isRoundedText}
            label={roundedTextLabel}
            flyoutLabel={t('logo_picker.flyouts.rounded_text', 'Rounded avatar')}
            flyoutLayout="stack"
            onShuffle={() => setRoundedTextShuffle((n) => n + 1)}
            tile={<img src={roundedTextTileSrc} alt="" style={{width: '100%', height: '100%', objectFit: 'cover'}} />}
          >
            <Tabs
              value={textVariant}
              onChange={(_, letterValue: TextLetterVariant) => setTextVariant(letterValue)}
              variant="fullWidth"
              sx={compactTabsSx}
            >
              {TEXT_LETTER_VARIANTS.map(({label, value: letterValue}) => (
                <Tab
                  key={letterValue}
                  value={letterValue}
                  label={t(`logo_picker.letter_variants.${letterValue}`, label)}
                />
              ))}
            </Tabs>
            <AvatarSwatchGrid
              shuffleSignal={roundedTextShuffle}
              base={{content: textContent, shape: 'rounded', variant: textVariant}}
              value={isRoundedText ? currentAvatarParams.colors : -1}
              gradientCount={AVATAR_GRADIENT_COUNT}
              optionCount={optionCount}
              showShuffle={false}
              onChange={(colors) => handlePickGradientSwatch('rounded', textVariant, textContent, colors)}
            />
          </GroupTile>
        )}

        {supportsCircle && (
          <GroupTile
            groupKey="circle_blank"
            openGroup={openGroup}
            onToggle={toggleGroup}
            selected={isCircleBlank}
            shape="circle"
            label={circleBlankLabel}
            flyoutLabel={t('logo_picker.flyouts.circle_blank', 'Pick a background')}
            flyoutLayout="stack"
            onShuffle={() => setCircleBlankShuffle((n) => n + 1)}
            tile={<img src={circleBlankTileSrc} alt="" style={{width: '100%', height: '100%', objectFit: 'cover'}} />}
          >
            <AvatarSwatchGrid
              shuffleSignal={circleBlankShuffle}
              base={{content: '', shape: 'circle', variant: 'blank'}}
              value={isCircleBlank ? currentAvatarParams.colors : -1}
              gradientCount={AVATAR_GRADIENT_COUNT}
              optionCount={optionCount}
              showShuffle={false}
              onChange={(colors) => handlePickGradientSwatch('circle', 'blank', '', colors)}
            />
          </GroupTile>
        )}

        {supportsCircle && (
          <GroupTile
            groupKey="circle_text"
            openGroup={openGroup}
            onToggle={toggleGroup}
            selected={isCircleText}
            shape="circle"
            label={circleTextLabel}
            flyoutLabel={t('logo_picker.flyouts.circle_text', 'Circular avatar')}
            flyoutLayout="stack"
            onShuffle={() => setCircleTextShuffle((n) => n + 1)}
            tile={<img src={circleTextTileSrc} alt="" style={{width: '100%', height: '100%', objectFit: 'cover'}} />}
          >
            <Tabs
              value={textVariant}
              onChange={(_, letterValue: TextLetterVariant) => setTextVariant(letterValue)}
              variant="fullWidth"
              sx={compactTabsSx}
            >
              {TEXT_LETTER_VARIANTS.map(({label, value: letterValue}) => (
                <Tab
                  key={letterValue}
                  value={letterValue}
                  label={t(`logo_picker.letter_variants.${letterValue}`, label)}
                />
              ))}
            </Tabs>
            <AvatarSwatchGrid
              shuffleSignal={circleTextShuffle}
              base={{content: textContent, shape: 'circle', variant: textVariant}}
              value={isCircleText ? currentAvatarParams.colors : -1}
              gradientCount={AVATAR_GRADIENT_COUNT}
              optionCount={optionCount}
              showShuffle={false}
              onChange={(colors) => handlePickGradientSwatch('circle', textVariant, textContent, colors)}
            />
          </GroupTile>
        )}

        <GroupTile
          groupKey="animal"
          openGroup={openGroup}
          onToggle={toggleGroup}
          selected={isAnimal}
          shape={isAnimal ? currentAvatarParams.shape : animalShape}
          label={t('logo_picker.groups.animal', 'Animal')}
          flyoutLabel={t('logo_picker.flyouts.animal', 'Choose an animal')}
          flyoutLayout="stack"
          onShuffle={() => setAnimalShuffle((n) => n + 1)}
          tile={<img src={animalTileSrc} alt="" style={{width: '100%', height: '100%', objectFit: 'cover'}} />}
        >
          {availableShapes.length > 1 && (
            <Tabs
              value={animalShape}
              onChange={(_, shapeValue: AvatarShape) => setAnimalShape(shapeValue)}
              variant="fullWidth"
              sx={compactTabsSx}
            >
              {availableShapes.map(({label, value: shapeValue}) => (
                <Tab key={shapeValue} value={shapeValue} label={t(`logo_picker.animal_shapes.${shapeValue}`, label)} />
              ))}
            </Tabs>
          )}
          <IconGridPicker
            shuffleSignal={animalShuffle}
            icons={animalIcons}
            value={isAnimal ? currentAvatarParams.content : animalName}
            shape={isAnimal ? currentAvatarParams.shape : animalShape}
            optionCount={optionCount}
            showShuffle={false}
            onChange={handlePickAnimal}
          />
        </GroupTile>

        <GroupTile
          groupKey="emoji"
          openGroup={openGroup}
          onToggle={toggleGroup}
          selected={scheme === 'emoji'}
          label={t('logo_picker.groups.emoji', 'Emoji')}
          flyoutLabel={t('logo_picker.flyouts.emoji', 'Choose an emoji')}
          onShuffle={handleShuffleEmoji}
          tile={<Typography sx={{fontSize: 28}}>{emojiTileGlyph}</Typography>}
        >
          {emojiOptions.map((char) => (
            <VariantTile
              key={char}
              selected={scheme === 'emoji' && value === EMOJI_SCHEME + char}
              onClick={() => handlePickEmoji(char)}
            >
              <Typography sx={{fontSize: 19}}>{char}</Typography>
            </VariantTile>
          ))}
          <VariantTile
            selected={false}
            onClick={() => setEmojiDialogOpen(true)}
            aria-label={t('logo_picker.groups.more_emojis', 'More emojis')}
          >
            <Plus size={16} />
          </VariantTile>
        </GroupTile>
      </Stack>

      {/* Full emoji picker, opened via the "+" tile in the Emoji flyout */}
      <Dialog open={emojiDialogOpen} onClose={() => setEmojiDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>
          <Stack direction="row" alignItems="center" justifyContent="space-between">
            <Typography variant="h5">{t('logo_picker.emoji_dialog.title', 'Choose an emoji')}</Typography>
            <IconButton
              aria-label={t('resource_logo_dialog.actions.close', 'Close')}
              onClick={() => setEmojiDialogOpen(false)}
              size="small"
            >
              <X size={20} />
            </IconButton>
          </Stack>
        </DialogTitle>
        <DialogContent dividers sx={{p: 0}}>
          <EmojiPicker value={scheme === 'emoji' ? value.slice(EMOJI_SCHEME.length) : ''} onChange={handlePickEmoji} />
        </DialogContent>
      </Dialog>
    </Stack>
  );
}

function groupTileSx(selected: boolean, shape: AvatarShape = 'rounded') {
  return {
    width: 64,
    height: 64,
    borderRadius: shape === 'circle' ? '50%' : '22%',
    overflow: 'hidden',
    cursor: 'pointer',
    p: 0,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    border: '1.5px solid',
    borderColor: selected ? 'primary.main' : 'divider',
    bgcolor: 'action.hover',
  };
}

function GroupTile({
  groupKey,
  openGroup,
  onToggle,
  selected,
  shape = 'rounded',
  label,
  flyoutLabel,
  onShuffle = undefined,
  flyoutLayout = 'wrap',
  tile,
  children,
}: {
  groupKey: GroupKey;
  openGroup: GroupKey | null;
  onToggle: (key: GroupKey) => void;
  selected: boolean;
  shape?: AvatarShape;
  label: string;
  flyoutLabel: string;
  onShuffle?: () => void;
  flyoutLayout?: 'stack' | 'wrap';
  tile: ReactNode;
  children: ReactNode;
}): JSX.Element {
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const open = openGroup === groupKey;

  return (
    <Box sx={{width: 64}}>
      <Box
        ref={setAnchorEl}
        component="button"
        type="button"
        aria-label={label}
        onClick={() => onToggle(groupKey)}
        sx={groupTileSx(selected, shape)}
      >
        {tile}
      </Box>
      <Typography variant="caption" color="text.secondary" sx={{display: 'block', textAlign: 'center', mt: 0.5}}>
        {label}
      </Typography>
      <Popover
        open={open}
        anchorEl={anchorEl}
        onClose={() => onToggle(groupKey)}
        anchorOrigin={{vertical: 'bottom', horizontal: 'left'}}
        transformOrigin={{vertical: 'top', horizontal: 'left'}}
        slotProps={{
          paper: {
            sx: {
              mt: 1,
              bgcolor: 'background.paper',
              backdropFilter: 'blur(10px)',
              WebkitBackdropFilter: 'blur(10px)',
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 2,
              overflow: 'hidden',
              boxShadow: 8,
              minWidth: 'initial',
            },
          },
        }}
      >
        <FlyoutContent label={flyoutLabel} onShuffle={onShuffle} layout={flyoutLayout}>
          {children}
        </FlyoutContent>
      </Popover>
    </Box>
  );
}

function FlyoutContent({
  label,
  onShuffle = undefined,
  layout = 'wrap',
  children,
}: {
  label: string;
  onShuffle?: () => void;
  layout?: 'stack' | 'wrap';
  children: ReactNode;
}): JSX.Element {
  const {t} = useTranslation('elements');
  return (
    <>
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        sx={{px: 1.5, py: 1.25, borderBottom: '1px solid', borderColor: 'divider'}}
      >
        <Typography variant="subtitle2" sx={{fontWeight: 600}}>
          {label}
        </Typography>
        {onShuffle && (
          <Button size="small" startIcon={<Shuffle size={12} />} onClick={onShuffle} sx={{minWidth: 'auto', p: 0.25}}>
            {t('logo_picker.shuffle', 'Shuffle')}
          </Button>
        )}
      </Stack>
      <Box sx={layout === 'stack' ? {} : {display: 'flex', flexWrap: 'wrap', gap: 0.875, p: 1.5}}>{children}</Box>
    </>
  );
}

function VariantTile({
  selected,
  onClick,
  children,
  ...rest
}: {
  selected: boolean;
  onClick: () => void;
  children: ReactNode;
} & Record<string, unknown>): JSX.Element {
  return (
    <Box
      component="button"
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      sx={{
        position: 'relative',
        width: 42,
        height: 42,
        borderRadius: 1.25,
        overflow: 'hidden',
        cursor: 'pointer',
        p: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        border: '1px solid',
        borderColor: selected ? 'primary.main' : 'divider',
        bgcolor: 'action.hover',
      }}
      {...rest}
    >
      {children}
    </Box>
  );
}
