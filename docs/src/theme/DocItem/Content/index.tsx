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

import {useDoc} from '@docusaurus/plugin-content-docs/client';
import Heading from '@theme/Heading';
import MDXContent from '@theme/MDXContent';
import {type ReactNode, useEffect, useRef} from 'react';
import GettingStartedJourney from '@site/src/components/GettingStartedJourney';
import {getGettingStartedStepIndex} from '@site/src/components/GettingStartedSteps';

function useSyntheticTitle(): string | null {
  const {metadata, frontMatter, contentTitle} = useDoc();
  const shouldRender = !frontMatter.hide_title && typeof contentTitle === 'undefined';
  if (!shouldRender) {
    return null;
  }
  return metadata.title;
}

export default function DocItemContent({children}: {children: ReactNode}): ReactNode {
  const syntheticTitle = useSyntheticTitle();
  const {metadata} = useDoc();
  const currentJourneyStep = getGettingStartedStepIndex(metadata.id);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const journeyContainerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (currentJourneyStep === null) {
      return;
    }

    const container = containerRef.current;
    const journeyContainer = journeyContainerRef.current;

    if (!container || !journeyContainer) {
      return;
    }

    const title = container.querySelector('h1');

    if (!title) {
      return;
    }

    const titleBlock = title.closest('header') ?? title;

    if (titleBlock.parentElement === container) {
      titleBlock.insertAdjacentElement('afterend', journeyContainer);
    }
  }, [currentJourneyStep, metadata.id]);

  return (
    <div ref={containerRef} className="theme-doc-markdown markdown doc-content-with-copy-btn">
      {syntheticTitle && (
        <header>
          <Heading as="h1">{syntheticTitle}</Heading>
        </header>
      )}
      {currentJourneyStep !== null && (
        <div ref={journeyContainerRef}>
          <GettingStartedJourney current={currentJourneyStep} />
        </div>
      )}
      <MDXContent>{children}</MDXContent>
    </div>
  );
}
