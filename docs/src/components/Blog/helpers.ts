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

import type {Content} from '@theme/BlogPostPage';
import {BlogHeroIconKey, DEFAULT_HERO_GRADIENT} from './icons';

/**
 * Custom, optional blog front matter this design introduces on top of Docusaurus's
 * built-in fields: `category` drives the filter pills, `featured` picks the hero
 * slot, `heroGradient`/`heroIcon` control the card thumbnail art.
 */
export interface BlogPostCustomFrontMatter {
  category?: string;
  featured?: boolean;
  heroGradient?: string;
  heroIcon?: BlogHeroIconKey;
  thumbnail?: string;
  bannerImage?: string;
}

// Accepts just the `frontMatter` slice so callers with a narrower blog-post
// context (e.g. `useBlogPost()`, which lacks `contentTitle`) can use it too.
type FrontMatterSource = Pick<Content, 'frontMatter'>;

export function getFrontMatter(content: FrontMatterSource): BlogPostCustomFrontMatter {
  return (content.frontMatter ?? {}) as BlogPostCustomFrontMatter;
}

export function getCategory(content: FrontMatterSource): string {
  return getFrontMatter(content).category ?? 'General';
}

export function getHeroGradient(content: FrontMatterSource): string {
  return getFrontMatter(content).heroGradient ?? DEFAULT_HERO_GRADIENT;
}

export function getHeroIcon(content: FrontMatterSource): BlogHeroIconKey {
  return getFrontMatter(content).heroIcon ?? 'default';
}

export function isFeatured(content: FrontMatterSource): boolean {
  return getFrontMatter(content).featured === true;
}

export function getThumbnail(content: FrontMatterSource): string | undefined {
  return getFrontMatter(content).thumbnail;
}

export function getBannerImage(content: FrontMatterSource): string | undefined {
  return getFrontMatter(content).bannerImage;
}

export function getInitials(name: string): string {
  const parts = name.trim().split(/\s+/);
  const first = parts[0]?.[0] ?? '';
  const last = parts.length > 1 ? (parts[parts.length - 1]?.[0] ?? '') : '';
  return (first + last).toUpperCase();
}

const DATE_FORMATTER = new Intl.DateTimeFormat('en-US', {month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC'});

export function formatDate(date: string): string {
  return DATE_FORMATTER.format(new Date(date));
}

export function formatReadingTime(readingTime: number): string {
  return `${Math.max(1, Math.round(readingTime))} min read`;
}

export function formatMetaLine(date: string, readingTime?: number): string {
  const formattedDate = formatDate(date);
  if (!readingTime) return formattedDate;
  return `${formattedDate} · ${formatReadingTime(readingTime)}`;
}
