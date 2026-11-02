// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  CONSENT_DETAIL_DISPLAY_LIMITS,
  Consent,
  ConsentAuthorizationDetail,
  ConsentCheckboxList,
  formatConsentDetailValue,
  isConsentDetailStructured,
  isConsentDetailTextClamped,
  getConsentOptionalKey,
  type ConsentDetailRow,
  type ConsentPurpose,
  type ConsentRenderProps,
  useConsentDetailField,
} from '@thunderid/react';
import {cn} from '@thunderid/utils';
import {Box, Button, Chip, Divider, FormControlLabel, Switch, TextField, Typography} from '@wso2/oxygen-ui';
import {useState, type ChangeEvent, type JSX, type KeyboardEvent, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import getAuthorizationDetailRowLayout from '../../../utils/getAuthorizationDetailRowLayout';

type ConsentElement = ConsentPurpose['optional'][number];

// RFC 9396 authorization details arrive as one consent purpose with an optional element per detail,
// named by the detail id and carrying server-rendered display fields.
function isAuthorizationDetailPurpose(purpose: ConsentPurpose): boolean {
  return purpose.type === 'authorization_details';
}

// The SDK names its authorization detail strings `consent.authorization_details.<name>`; the Gate keeps
// the same strings as `consent:authorizationDetails.<camelName>`.
const SDK_DETAIL_TEXT_PREFIX = 'consent.authorization_details.';

function toGateDetailTextKey(key: string): string | undefined {
  if (!key.startsWith(SDK_DETAIL_TEXT_PREFIX)) return undefined;
  const name = key
    .slice(SDK_DETAIL_TEXT_PREFIX.length)
    .replace(/_([a-z])/g, (_match: string, letter: string) => letter.toUpperCase());
  return `consent:authorizationDetails.${name}`;
}

type DetailText = (key: string, params?: Record<string, string | number>, defaultValue?: string) => string;

// Resolves an SDK authorization detail string. The Gate's text, including translations the server merges
// in, wins. Otherwise the key is returned unchanged, which makes the SDK fall back to its own text, or, for
// a string the Gate draws itself, defaultValue, the SDK's wording, is used.
function useDetailText(): DetailText {
  const {t, i18n} = useTranslation();
  return (key, params, defaultValue) => {
    const gateKey = toGateDetailTextKey(key);
    if (!gateKey) return key;
    if (i18n.exists(gateKey)) return t(gateKey, params);
    return defaultValue === undefined ? key : t(gateKey, {...params, defaultValue});
  };
}

function isPermissionPurpose(purpose: ConsentPurpose): boolean {
  return purpose.type === 'permissions';
}

// Groups permission elements by rollup parent. Permissions whose `parent` is empty or whose
// parent is not itself a prompted permission appear under topLevel; the rest appear under their
// parent's entry in childrenOf.
function groupPermissionsByParent(permissions: ConsentPurpose['optional']): {
  topLevel: string[];
  childrenOf: Record<string, string[]>;
} {
  const names = new Set<string>(permissions.map((p) => p.name));
  const childrenOf: Record<string, string[]> = {};
  const topLevel: string[] = [];

  permissions.forEach((p) => {
    if (p.parent && names.has(p.parent)) {
      if (!childrenOf[p.parent]) {
        childrenOf[p.parent] = [];
      }
      childrenOf[p.parent].push(p.name);
    } else {
      topLevel.push(p.name);
    }
  });

  return {topLevel, childrenOf};
}

interface ConsentAdapterProps {
  consentData?: string | ConsentPurpose[] | {purposes: ConsentPurpose[]};
  formValues: Record<string, string>;
  onInputChange: (name: string, value: string) => void;
}

// Permissions are opt-in: an absent form value means the user has not granted the permission. This
// must match how the SDK compiles consent_decisions on submit, or the screen and the token disagree.
function isPermissionChecked(formValues: Record<string, string>, purposeId: string, name: string): boolean {
  return formValues[getConsentOptionalKey(purposeId, name)] === 'true';
}

// collectDescendants returns the transitive set of descendants of `name` per the parent index.
function collectDescendants(name: string, childrenOf: Record<string, string[]>): string[] {
  const out: string[] = [];
  const direct = childrenOf[name] ?? [];
  direct.forEach((child) => {
    out.push(child);
    out.push(...collectDescendants(child, childrenOf));
  });
  return out;
}

function PermissionRow({
  purposeId,
  name,
  formValues,
  onInputChange,
  descendants,
  depth,
}: {
  purposeId: string;
  name: string;
  formValues: Record<string, string>;
  onInputChange: (key: string, value: string) => void;
  // Transitive descendants — used for parent-state rollup and cascade toggle.
  descendants: string[];
  depth: number;
}): JSX.Element {
  const selfChecked = isPermissionChecked(formValues, purposeId, name);
  const descendantsChecked = descendants.map((c) => isPermissionChecked(formValues, purposeId, c));
  const someDescendantsChecked = descendantsChecked.some(Boolean);
  const allDescendantsChecked = descendants.length > 0 && descendantsChecked.every(Boolean);
  const indeterminate = descendants.length > 0 && someDescendantsChecked && !allDescendantsChecked;
  const displayChecked = descendants.length > 0 ? allDescendantsChecked || selfChecked : selfChecked;

  const handleToggle = (checked: boolean): void => {
    onInputChange(getConsentOptionalKey(purposeId, name), checked ? 'true' : 'false');
    descendants.forEach((c) => {
      onInputChange(getConsentOptionalKey(purposeId, c), checked ? 'true' : 'false');
    });
  };

  return (
    <Box sx={{px: 1, pl: 1 + depth * 3}}>
      <FormControlLabel
        className={cn('FormControlLabel--root')}
        control={
          <Switch
            className={cn('Switch--root')}
            checked={displayChecked}
            inputProps={{'aria-checked': indeterminate ? 'mixed' : displayChecked}}
            onChange={(e): void => handleToggle((e.target as HTMLInputElement).checked)}
            size="small"
          />
        }
        label={
          <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5}}>
            <Box
              sx={{
                width: 6,
                height: 6,
                borderRadius: '50%',
                backgroundColor: 'text.disabled',
                flexShrink: 0,
              }}
            />
            <Typography className={cn('Text--body2')} variant="body2" sx={{fontWeight: 500}}>
              {name}
            </Typography>
          </Box>
        }
        labelPlacement="start"
        sx={{
          m: 0,
          width: '100%',
          justifyContent: 'space-between',
          py: 0.5,
        }}
      />
      <Divider className={cn('Divider--root')} sx={{opacity: 0.5}} />
    </Box>
  );
}

// Expands or collapses long authorization detail content. With hiddenCount the collapsed label counts
// the items left out.
function DetailToggle({
  controls = undefined,
  expanded,
  hiddenCount = undefined,
  onToggle,
}: {
  controls?: string;
  expanded: boolean;
  hiddenCount?: number;
  onToggle: (expanded: boolean) => void;
}): JSX.Element {
  const {t} = useTranslation();
  let label = t('consent:authorizationDetails.showLess', 'Show less');
  if (!expanded) {
    label =
      hiddenCount === undefined
        ? t('consent:authorizationDetails.showMore', 'Show more')
        : t('consent:authorizationDetails.moreItems', {count: hiddenCount, defaultValue: '+{{count}} more'});
  }

  return (
    <Button
      className={cn('Button--root')}
      variant="text"
      size="small"
      aria-controls={controls}
      aria-expanded={expanded}
      onClick={(): void => onToggle(!expanded)}
      sx={{minWidth: 0, px: 0.5, py: 0, textTransform: 'none', fontSize: '0.8125rem', flexShrink: 0}}
    >
      {label}
    </Button>
  );
}

// Text from an authorization detail. It wraps anywhere, so a long unbroken token such as an IBAN or a URL
// never overflows, and text longer than its limit is clamped behind a toggle.
function DetailText({
  align = 'left',
  id = undefined,
  maxChars,
  maxLines,
  monospace = false,
  text,
  variant = 'body2',
}: {
  align?: 'left' | 'right';
  id?: string;
  maxChars: number;
  maxLines: number;
  monospace?: boolean;
  text: string;
  variant?: 'body2' | 'subtitle2';
}): JSX.Element {
  const [expanded, setExpanded] = useState(false);
  const long = isConsentDetailTextClamped(text, maxChars, maxLines);

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: align === 'right' ? 'flex-end' : 'flex-start',
        minWidth: 0,
        maxWidth: '100%',
      }}
    >
      <Typography
        id={id}
        className={cn(variant === 'subtitle2' ? 'Text--subtitle2' : 'Text--body2')}
        variant={variant}
        sx={{
          fontWeight: variant === 'subtitle2' ? 'bold' : 500,
          lineHeight: 1.45,
          maxWidth: '100%',
          overflowWrap: 'anywhere',
          textAlign: align,
          whiteSpace: monospace ? 'pre-wrap' : 'normal',
          ...(monospace && {
            width: '100%',
            boxSizing: 'border-box',
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
            fontSize: '0.75rem',
            fontWeight: 400,
            px: 1,
            py: 0.75,
            border: 1,
            borderColor: 'divider',
            borderRadius: 1,
            backgroundColor: 'action.hover',
          }),
          ...(long &&
            !expanded && {
              display: '-webkit-box',
              overflow: 'hidden',
              WebkitBoxOrient: 'vertical',
              WebkitLineClamp: maxLines,
            }),
        }}
      >
        {text}
      </Typography>
      {long && <DetailToggle controls={id} expanded={expanded} onToggle={setExpanded} />}
    </Box>
  );
}

// A chip keeps to one line; a long item is cut short with an ellipsis and shown in full on hover.
const CHIP_SX = {maxWidth: '100%', minWidth: 0};

export interface AuthorizationDetailValueProps {
  // Side the value is aligned to: end beside its label, start when stacked below it. Defaults to end.
  align?: 'start' | 'end';
  purposeId: string;
  detailId: string;
  row: ConsentDetailRow;
  formValues: Record<string, string>;
  onInputChange: (key: string, value: string) => void;
}

// One field of an authorization detail. A field the user may modify before approving is editable; the
// SDK's useConsentDetailField holds its state, checks what is typed against the value that was shown,
// and writes the decision, so this only draws it. The server checks whatever is submitted. Exported so an
// editor of a type can show a field exactly as the consent screen does.
export function AuthorizationDetailValue({
  align = 'end',
  purposeId,
  detailId,
  row,
  formValues,
  onInputChange,
}: AuthorizationDetailValueProps): JSX.Element {
  const detailText = useDetailText();
  const field = useConsentDetailField({purposeId, detailId, row, formValues, onInputChange, t: detailText});
  const {inputId, label, listDisplay} = field;
  const collapse = (items: ReactNode[]): ReactNode[] => items.slice(0, listDisplay.visibleCount);
  const listToggle = listDisplay.isCollapsible && (
    <DetailToggle
      expanded={field.isListExpanded}
      hiddenCount={listDisplay.hiddenCount}
      onToggle={field.handleListToggle}
    />
  );
  const groupSx = {
    display: 'flex',
    flexWrap: 'wrap',
    alignItems: 'center',
    justifyContent: align === 'end' ? 'flex-end' : 'flex-start',
    gap: 0.75,
    minWidth: 0,
    maxWidth: '100%',
    // Stacked below its label, the list takes the row's width and wraps within it; a chip's one-line label
    // must not widen the screen it sits in.
    ...(align === 'start' && {width: '100%', contain: 'inline-size'}),
  };

  if (field.kind === 'number' || field.kind === 'date') {
    return (
      <TextField
        id={inputId}
        className={cn('TextField--root')}
        type={field.kind}
        size="small"
        value={field.text}
        error={Boolean(field.error)}
        helperText={field.error?.message}
        onChange={(e: ChangeEvent<HTMLInputElement>): void => field.handleChange(e.target.value)}
        onBlur={field.handleBlur}
        slotProps={{
          htmlInput: {
            'aria-label': label,
            ...(field.kind === 'number' && {step: 'any'}),
            max: field.max,
            min: field.min,
          },
        }}
        sx={field.kind === 'number' ? {maxWidth: 140} : undefined}
      />
    );
  }

  if (field.kind === 'removeItems') {
    return (
      <Box role="group" aria-label={label} sx={groupSx}>
        {collapse(
          field.items.map(({selected, toggleable, value}, index) => {
            const toggle = toggleable ? (): void => field.handleToggle(value) : undefined;
            // The whole chip toggles the item: a kept item is removed, except the last one, since to grant
            // none of them the user declines the detail instead. A removed item stays visible so the user can
            // add it back.
            return (
              <Chip
                key={String(value)}
                id={`${inputId}_${index}`}
                className={cn('Chip--root')}
                size="small"
                label={String(value)}
                title={String(value)}
                color={selected ? 'primary' : 'default'}
                variant="outlined"
                aria-pressed={toggleable ? selected : undefined}
                aria-label={
                  toggleable
                    ? detailText(
                        selected
                          ? 'consent.authorization_details.remove_item'
                          : 'consent.authorization_details.add_back_item',
                        {item: String(value)},
                        selected ? '{{item}}, remove' : '{{item}}, add back',
                      )
                    : undefined
                }
                onClick={toggle}
                onDelete={selected ? toggle : undefined}
                sx={
                  selected
                    ? CHIP_SX
                    : {
                        ...CHIP_SX,
                        '&&': {
                          backgroundColor: 'transparent',
                          borderColor: 'divider',
                          color: 'text.disabled',
                          textDecoration: 'line-through',
                        },
                      }
                }
              />
            );
          }),
        )}
        {listToggle}
      </Box>
    );
  }

  if (field.kind === 'addItems') {
    // The items shown cannot be removed, since removing one would grant more. Items the user adds can be
    // taken out again before approving, and the values the type allows are offered to add.
    return (
      <Box role="group" aria-label={label} sx={groupSx}>
        {collapse(
          field.items.map(({origin, value}, index) => {
            if (origin === 'shown') {
              return (
                <Chip
                  key={String(value)}
                  id={`${inputId}_${index}`}
                  className={cn('Chip--root')}
                  size="small"
                  color="primary"
                  variant="outlined"
                  label={String(value)}
                  title={String(value)}
                  sx={CHIP_SX}
                />
              );
            }
            const toggle = (): void => field.handleToggle(value);
            if (origin === 'added') {
              return (
                <Chip
                  key={`added-${String(value)}`}
                  id={`${inputId}_added_${String(value)}`}
                  className={cn('Chip--root')}
                  size="small"
                  color="primary"
                  variant="outlined"
                  label={String(value)}
                  title={String(value)}
                  aria-label={detailText(
                    'consent.authorization_details.remove_item',
                    {item: String(value)},
                    '{{item}}, remove',
                  )}
                  onClick={toggle}
                  onDelete={toggle}
                  sx={CHIP_SX}
                />
              );
            }
            return (
              <Chip
                key={`offered-${String(value)}`}
                id={`${inputId}_offered_${String(value)}`}
                className={cn('Chip--root')}
                size="small"
                variant="outlined"
                label={`+ ${String(value)}`}
                title={String(value)}
                aria-label={detailText(
                  'consent.authorization_details.add_item',
                  {item: String(value)},
                  '{{item}}, add',
                )}
                onClick={toggle}
                sx={{...CHIP_SX, '&&': {borderStyle: 'dashed', borderColor: 'divider', color: 'text.secondary'}}}
              />
            );
          }),
        )}
        {listToggle}
        {field.isTypedItemAllowed && (
          <TextField
            id={`${inputId}_new`}
            className={cn('TextField--root')}
            size="small"
            value={field.text}
            onChange={(e: ChangeEvent<HTMLInputElement>): void => field.handleChange(e.target.value)}
            onKeyDown={(e: KeyboardEvent<HTMLInputElement>): void => {
              if (e.key === 'Enter') {
                e.preventDefault();
                field.handleAdd((e.target as HTMLInputElement).value);
              }
            }}
            slotProps={{
              htmlInput: {
                'aria-label': detailText('consent.authorization_details.new_item', {label}, '{{label}}, add'),
              },
            }}
            sx={{maxWidth: 140}}
          />
        )}
      </Box>
    );
  }

  const textAlign = align === 'end' ? 'right' : 'left';

  // A long list of plain values shows its first items, the rest behind a toggle.
  if (field.isPlainList && listDisplay.isCollapsible) {
    return (
      <Box sx={{...groupSx, alignItems: 'baseline', gap: '2px 6px'}}>
        <DetailText
          align={textAlign}
          maxChars={Number.POSITIVE_INFINITY}
          maxLines={Number.POSITIVE_INFINITY}
          text={formatConsentDetailValue((row.value as unknown[]).slice(0, listDisplay.visibleCount))}
        />
        {listToggle}
      </Box>
    );
  }

  // Structured JSON reads as an indented block rather than one long line.
  if (isConsentDetailStructured(row.value)) {
    return (
      <DetailText
        monospace
        maxChars={Number.POSITIVE_INFINITY}
        maxLines={CONSENT_DETAIL_DISPLAY_LIMITS.structuredLines}
        text={formatConsentDetailValue(row.value, 2)}
      />
    );
  }

  return (
    <DetailText
      align={textAlign}
      maxChars={CONSENT_DETAIL_DISPLAY_LIMITS.valueChars}
      maxLines={CONSENT_DETAIL_DISPLAY_LIMITS.valueLines}
      text={formatConsentDetailValue(row.value)}
    />
  );
}

// The heading of an authorization detail, clamped as long text is. Exported so an editor of a type can
// show the heading exactly as the consent screen does.
export function AuthorizationDetailSummary({id = undefined, text}: {id?: string; text: string}): JSX.Element {
  return (
    <DetailText
      id={id}
      variant="subtitle2"
      maxChars={CONSENT_DETAIL_DISPLAY_LIMITS.summaryChars}
      maxLines={CONSENT_DETAIL_DISPLAY_LIMITS.summaryLines}
      text={text}
    />
  );
}

function AuthorizationDetailCard({
  purposeId,
  element,
  formValues,
  onInputChange,
}: {
  purposeId: string;
  element: ConsentElement;
  formValues: Record<string, string>;
  onInputChange: (key: string, value: string) => void;
}): JSX.Element {
  const {t} = useTranslation();
  const detailId = element.name;
  const summaryId = `consent_authz_detail_${purposeId}_${detailId}_summary`;

  return (
    <ConsentAuthorizationDetail
      purposeId={purposeId}
      element={element}
      formValues={formValues}
      onInputChange={onInputChange}
    >
      {({handleChange, isApproved, rows, summary}) => (
        <Box
          className={cn('Flow--consent-authorization-detail')}
          sx={{border: 1, borderColor: 'divider', borderRadius: 1, p: 1.5, mt: 1}}
        >
          <Box sx={{mb: 1}}>
            <AuthorizationDetailSummary id={summaryId} text={summary} />
          </Box>
          {rows.length > 0 && (
            <Box sx={{display: 'flex', flexDirection: 'column', mb: 1}}>
              {rows.map((row, rowIdx) => {
                const layout = getAuthorizationDetailRowLayout(row);
                return (
                  <Box key={row.path ?? rowIdx} sx={layout.row}>
                    <Typography className={cn('Text--body2')} variant="body2" color="text.secondary" sx={layout.label}>
                      {row.label || row.path}
                    </Typography>
                    <Box sx={layout.value}>
                      <AuthorizationDetailValue
                        align={layout.stacked ? 'start' : 'end'}
                        purposeId={purposeId}
                        detailId={detailId}
                        row={row}
                        formValues={formValues}
                        onInputChange={onInputChange}
                      />
                    </Box>
                  </Box>
                );
              })}
            </Box>
          )}
          <Box sx={{px: 1}}>
            <Divider className={cn('Divider--root')} sx={{opacity: 0.5}} />
            <FormControlLabel
              className={cn('FormControlLabel--root')}
              control={
                <Switch
                  className={cn('Switch--root')}
                  checked={isApproved}
                  slotProps={{input: {id: `consent_opt_${purposeId}_${detailId}`, 'aria-describedby': summaryId}}}
                  onChange={(e): void => handleChange((e.target as HTMLInputElement).checked)}
                  size="small"
                />
              }
              label={
                <Typography className={cn('Text--body2')} variant="body2" sx={{fontWeight: 500}}>
                  {t('consent:authorizationDetails.approve', 'Approve')}
                </Typography>
              }
              labelPlacement="start"
              sx={{m: 0, width: '100%', justifyContent: 'space-between', py: 0.5}}
            />
          </Box>
        </Box>
      )}
    </ConsentAuthorizationDetail>
  );
}

export default function ConsentAdapter({
  consentData = undefined,
  formValues,
  onInputChange,
}: ConsentAdapterProps): JSX.Element | null {
  if (!consentData) return null;

  return (
    <Consent consentData={consentData} formValues={formValues} onInputChange={onInputChange}>
      {({purposes}: ConsentRenderProps) => (
        <Box className={cn('Flow--consent')} sx={{display: 'flex', flexDirection: 'column', gap: 2, mt: 1}}>
          {purposes.map((purpose, idx) => (
            <Box key={purpose.purposeId ?? idx}>
              {isAuthorizationDetailPurpose(purpose) &&
                (purpose.optional ?? []).map((element) => (
                  <AuthorizationDetailCard
                    key={element.name}
                    purposeId={purpose.purposeId}
                    element={element}
                    formValues={formValues}
                    onInputChange={onInputChange}
                  />
                ))}
              {isPermissionPurpose(purpose) && purpose.optional && purpose.optional.length > 0 && (
                <Box sx={{mt: 1}}>
                  <Typography className={cn('Text--subtitle2')} variant="subtitle2" fontWeight="bold" sx={{mb: 0.5}}>
                    Permissions
                  </Typography>
                  {(() => {
                    const grouped = groupPermissionsByParent(purpose.optional);
                    const renderNode = (name: string, depth: number): JSX.Element => {
                      const direct = grouped.childrenOf[name] ?? [];
                      const descendants = collectDescendants(name, grouped.childrenOf);
                      return (
                        <Box key={name}>
                          <PermissionRow
                            purposeId={purpose.purposeId}
                            name={name}
                            formValues={formValues}
                            onInputChange={onInputChange}
                            descendants={descendants}
                            depth={depth}
                          />
                          {direct.map((childName) => renderNode(childName, depth + 1))}
                        </Box>
                      );
                    };
                    return (
                      <Box sx={{display: 'flex', flexDirection: 'column'}}>
                        {grouped.topLevel.map((name) => renderNode(name, 0))}
                      </Box>
                    );
                  })()}
                </Box>
              )}
              {!isAuthorizationDetailPurpose(purpose) &&
                !isPermissionPurpose(purpose) &&
                purpose.essential &&
                purpose.essential.length > 0 && (
                  <Box sx={{mt: 1}}>
                    <Typography className={cn('Text--subtitle2')} variant="subtitle2" fontWeight="bold" sx={{mb: 0.5}}>
                      Essential Attributes
                    </Typography>
                    <ConsentCheckboxList
                      variant="ESSENTIAL"
                      purpose={purpose}
                      formValues={formValues}
                      onInputChange={onInputChange}
                    >
                      {({attributes, isChecked}) => (
                        <Box sx={{display: 'flex', flexDirection: 'column'}}>
                          {attributes.map((attr) => (
                            <Box key={attr} sx={{px: 1}}>
                              <FormControlLabel
                                className={cn('FormControlLabel--root')}
                                control={
                                  <Switch
                                    className={cn('Switch--root')}
                                    checked={isChecked(attr)}
                                    disabled
                                    size="small"
                                  />
                                }
                                label={
                                  <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5}}>
                                    <Box
                                      sx={{
                                        width: 6,
                                        height: 6,
                                        borderRadius: '50%',
                                        backgroundColor: 'text.disabled',
                                        flexShrink: 0,
                                      }}
                                    />
                                    <Typography className={cn('Text--body2')} variant="body2" sx={{fontWeight: 500}}>
                                      {attr}
                                    </Typography>
                                  </Box>
                                }
                                labelPlacement="start"
                                sx={{
                                  m: 0,
                                  width: '100%',
                                  justifyContent: 'space-between',
                                  py: 0.5,
                                }}
                              />
                              <Divider className={cn('Divider--root')} sx={{opacity: 0.5}} />
                            </Box>
                          ))}
                        </Box>
                      )}
                    </ConsentCheckboxList>
                  </Box>
                )}
              {!isAuthorizationDetailPurpose(purpose) &&
                !isPermissionPurpose(purpose) &&
                purpose.optional &&
                purpose.optional.length > 0 && (
                  <Box sx={{mt: 1}}>
                    <Typography className={cn('Text--subtitle2')} variant="subtitle2" fontWeight="bold" sx={{mb: 0.5}}>
                      Optional Attributes
                    </Typography>
                    <ConsentCheckboxList
                      variant="OPTIONAL"
                      purpose={purpose}
                      formValues={formValues}
                      onInputChange={onInputChange}
                    >
                      {({attributes, isChecked, handleChange}) => (
                        <Box sx={{display: 'flex', flexDirection: 'column'}}>
                          {attributes.map((attr) => (
                            <Box key={attr} sx={{px: 1}}>
                              <FormControlLabel
                                className={cn('FormControlLabel--root')}
                                control={
                                  <Switch
                                    className={cn('Switch--root')}
                                    checked={isChecked(attr)}
                                    onChange={(e) => handleChange(attr, (e.target as HTMLInputElement).checked)}
                                    size="small"
                                  />
                                }
                                label={
                                  <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5}}>
                                    <Box
                                      sx={{
                                        width: 6,
                                        height: 6,
                                        borderRadius: '50%',
                                        backgroundColor: 'text.disabled',
                                        flexShrink: 0,
                                      }}
                                    />
                                    <Typography className={cn('Text--body2')} variant="body2" sx={{fontWeight: 500}}>
                                      {attr}
                                    </Typography>
                                  </Box>
                                }
                                labelPlacement="start"
                                sx={{
                                  m: 0,
                                  width: '100%',
                                  justifyContent: 'space-between',
                                  py: 0.5,
                                }}
                              />
                              <Divider className={cn('Divider--root')} sx={{opacity: 0.5}} />
                            </Box>
                          ))}
                        </Box>
                      )}
                    </ConsentCheckboxList>
                  </Box>
                )}
              {idx < purposes.length - 1 && <Divider className={cn('Divider--root')} sx={{mt: 2}} />}
            </Box>
          ))}
        </Box>
      )}
    </Consent>
  );
}
