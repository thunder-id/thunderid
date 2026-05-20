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
  Consent,
  ConsentCheckboxList,
  getConsentOptionalKey,
  type ConsentPurpose,
  type ConsentRenderProps,
} from '@thunderid/react';
import {cn} from '@thunderid/utils';
import {Box, Divider, FormControlLabel, Switch, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';

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

function isPermissionChecked(formValues: Record<string, string>, purposeId: string, name: string): boolean {
  return formValues[getConsentOptionalKey(purposeId, name)] !== 'false';
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
              {!isPermissionPurpose(purpose) && purpose.essential && purpose.essential.length > 0 && (
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
              {!isPermissionPurpose(purpose) && purpose.optional && purpose.optional.length > 0 && (
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
