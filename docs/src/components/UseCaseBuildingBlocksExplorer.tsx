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

import React, { useEffect, useRef, useState } from 'react';

export interface UseCaseBuildingBlockDetail {
  id: string;
  label: string;
  title: string;
  icon: React.ReactNode;
  why: React.ReactNode;
  example?: React.ReactNode;
  capabilityGroups: {
    title: string;
    items: string[];
  }[];
}

export interface UseCaseBuildingBlockGroup {
  id: string;
  title?: string;
  description?: string;
  nodes: UseCaseBuildingBlockDetail[];
  variant?: 'primary' | 'secondary';
}

interface UseCaseBuildingBlockPanelProps {
  capabilitiesHeading?: string;
  capabilityGroups?: { title: string; items: string[] }[];
  example?: React.ReactNode;
  icon: React.ReactNode;
  id?: string;
  role?: string;
  title: string;
  why: React.ReactNode;
  whyHeading?: string;
}

export function UseCaseBuildingBlockPanel({
  capabilitiesHeading = 'Capabilities Involved',
  capabilityGroups = undefined,
  example = undefined,
  icon,
  id = undefined,
  role = undefined,
  title,
  why,
  whyHeading = 'Why This Matters',
}: UseCaseBuildingBlockPanelProps) {
  return (
    <article id={id} className="uc-building-blocks__panel" role={role}>
      <div className="uc-building-blocks__body">
        <div className="uc-building-blocks__panel-header">
          <span className="uc-building-block-node__icon" aria-hidden>
            {icon}
          </span>
          <h3>{title}</h3>
        </div>
        <h4>{whyHeading}</h4>
        <p>{why}</p>
        {example && (
          <>
            <h4>Example</h4>
            <p>{example}</p>
          </>
        )}
        {capabilityGroups && capabilityGroups.length > 0 && (
          <>
            <h4>{capabilitiesHeading}</h4>
            <div className="uc-building-blocks__capability-groups">
              {capabilityGroups.map((group) => (
                <div key={group.title} className="uc-building-blocks__capability-group">
                  <h5>{group.title}</h5>
                  <ul>
                    {group.items.map((capability) => (
                      <li key={capability}>{capability}</li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          </>
        )}
      </div>
    </article>
  );
}

interface UseCaseBuildingBlocksExplorerProps {
  ariaLabel: string;
  capabilitiesHeading?: string;
  compact?: boolean;
  detailPanelId: string;
  groups: UseCaseBuildingBlockGroup[];
  scrollableGroupIds?: string[];
  whyHeading?: string;
}

export function UseCaseBuildingBlocksExplorer({
  ariaLabel,
  capabilitiesHeading = 'Capabilities Involved',
  compact = false,
  detailPanelId,
  groups,
  scrollableGroupIds = [],
  whyHeading = 'Why This Matters',
}: UseCaseBuildingBlocksExplorerProps) {
  const buildingBlocks = groups.flatMap((group) => group.nodes);
  const rowRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const scrollableGroupKey = scrollableGroupIds.join('|');
  const [scrollState, setScrollState] = useState<Record<string, { left: boolean; right: boolean }>>({});
  const [selectedId, setSelectedId] = useState(buildingBlocks[0].id);
  const selectedBlock = buildingBlocks.find((detail) => detail.id === selectedId) ?? buildingBlocks[0];

  const updateScrollState = (groupId: string) => {
    const row = rowRefs.current[groupId];

    if (!row) {
      return;
    }

    const maxScrollLeft = row.scrollWidth - row.clientWidth;

    setScrollState((current) => ({
      ...current,
      [groupId]: {
        left: row.scrollLeft > 1,
        right: row.scrollLeft < maxScrollLeft - 1,
      },
    }));
  };

  useEffect(() => {
    scrollableGroupIds.forEach(updateScrollState);

    const handleResize = () => {
      scrollableGroupIds.forEach(updateScrollState);
    };

    window.addEventListener('resize', handleResize);

    return () => window.removeEventListener('resize', handleResize);
  }, [scrollableGroupKey, scrollableGroupIds]);

  return (
    <section className={`uc-building-blocks${compact ? ' uc-building-blocks--compact' : ''}${scrollableGroupIds.length > 0 ? ' uc-building-blocks--scrollable' : ''}`} aria-label={ariaLabel}>
      <div className="uc-building-blocks__map" role="tablist" aria-label={ariaLabel}>
        {groups.map((group, index) => {
          const isScrollable = scrollableGroupIds.includes(group.id);
          const isSecondary = group.variant === 'secondary' || (group.variant === undefined && index > 0);

          return (
            <div key={group.id} className="uc-building-blocks__group">
              {(group.title ?? group.description) && (
                <div className="uc-building-blocks__group-header">
                  {group.title && <div className="uc-building-blocks__group-title">{group.title}</div>}
                  {group.description && <p>{group.description}</p>}
                </div>
              )}
              <div className={isScrollable ? `uc-building-blocks__scroll-shell${scrollState[group.id]?.left ? ' uc-building-blocks__scroll-shell--fade-left' : ''}${scrollState[group.id]?.right ? ' uc-building-blocks__scroll-shell--fade-right' : ''}` : undefined}>
                <div
                  ref={(element) => {
                    rowRefs.current[group.id] = element;
                  }}
                  onScroll={() => updateScrollState(group.id)}
                  className={isSecondary ? 'uc-building-blocks__secondary-row' : 'uc-building-blocks__primary-row'}
                >
                  {group.nodes.map((detail) => (
                    <button
                      key={detail.id}
                      type="button"
                      role="tab"
                      aria-selected={selectedBlock.id === detail.id}
                      aria-controls={detailPanelId}
                      className={`uc-building-block-node${isSecondary ? ' uc-building-block-node--secondary' : ''}${selectedBlock.id === detail.id ? ' uc-building-block-node--active' : ''}`}
                      onClick={() => setSelectedId(detail.id)}
                    >
                      <span className="uc-building-block-node__icon" aria-hidden>
                        {detail.icon}
                      </span>
                      <span className="uc-building-block-node__label">{detail.label}</span>
                    </button>
                  ))}
                </div>
              </div>
            </div>
          );
        })}
      </div>

      <UseCaseBuildingBlockPanel
        id={detailPanelId}
        role="tabpanel"
        icon={selectedBlock.icon}
        title={selectedBlock.title}
        why={selectedBlock.why}
        example={selectedBlock.example}
        capabilityGroups={selectedBlock.capabilityGroups}
        whyHeading={whyHeading}
        capabilitiesHeading={capabilitiesHeading}
      />
    </section>
  );
}
