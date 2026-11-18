// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {useSyncExternalStore} from 'react';
import data from '@site/static/data/events.json';

/** hosted: ThunderID's own events. presence: a booth, desk, or project presence. talk: a talk at someone else's event. */
export type EventType = 'hosted' | 'presence' | 'talk';

export interface EventTalk {
  title: string;
  speakers: string[];
}

export interface HubEvent {
  id: string;
  /** `{{ProductName}}` is replaced with the product name. */
  title: string;
  type: EventType;
  /** YYYY-MM-DD. */
  startDate: string;
  /** YYYY-MM-DD, the same as `startDate` for one-day events. */
  endDate: string;
  city?: string;
  country?: string;
  online: boolean;
  /** One line on what ThunderID is doing there. `{{ProductName}}` is replaced. */
  summary: string;
  talks?: EventTalk[];
  /** Site path for hosted events, an external URL for the rest. */
  url: string;
  /** Reserved for a later iteration. */
  poster?: string;
  recapUrl?: string;
}

export const EVENTS = data.events as HubEvent[];

/** Display order everywhere: hosted, then presence, then talk. */
export const TYPE_RANK: Record<EventType, number> = {hosted: 0, presence: 1, talk: 2};

export function isExternalUrl(url: string): boolean {
  return /^https?:\/\//.test(url);
}

function localToday(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

const noopSubscribe = () => () => undefined;

/**
 * Today's date as YYYY-MM-DD. The server render and hydration use the build date, so the markup matches,
 * then the browser switches to the visitor's own date. Events therefore move to Past without a rebuild.
 */
export function useToday(): string {
  const {siteConfig} = useDocusaurusContext();
  const buildDate = (siteConfig.customFields?.buildDate as string | undefined) ?? '1970-01-01';
  return useSyncExternalStore(noopSubscribe, localToday, () => buildDate);
}

export function isPast(event: HubEvent, today: string): boolean {
  return event.endDate < today;
}

const toDate = (iso: string): Date => new Date(`${iso}T00:00:00Z`);

const fmt = (iso: string, options: Intl.DateTimeFormatOptions): string =>
  toDate(iso).toLocaleDateString('en-US', {timeZone: 'UTC', ...options});

/** "Dec 10", "Dec 10–12", or "Jun 30–Jul 2". The year comes from the surrounding group heading. */
export function formatDayRange(start: string, end: string): string {
  const startLabel = fmt(start, {month: 'short', day: 'numeric'});
  if (start === end) return startLabel;
  const sameMonth = start.slice(0, 7) === end.slice(0, 7);
  return `${startLabel}–${fmt(end, sameMonth ? {day: 'numeric'} : {month: 'short', day: 'numeric'})}`;
}

/** "December 10, 2026", or "June 9–10, 2026" for a multi-day event. */
export function formatFullDate(start: string, end: string): string {
  if (start === end) return fmt(start, {month: 'long', day: 'numeric', year: 'numeric'});
  const sameMonth = start.slice(0, 7) === end.slice(0, 7);
  const endLabel = fmt(
    end,
    sameMonth ? {day: 'numeric', year: 'numeric'} : {month: 'long', day: 'numeric', year: 'numeric'},
  );
  return `${fmt(start, {month: 'long', day: 'numeric'})}–${endLabel}`;
}

/** "Thursday" or "Tuesday to Wednesday". */
export function formatWeekdays(start: string, end: string): string {
  const first = fmt(start, {weekday: 'long'});
  return start === end ? first : `${first} to ${fmt(end, {weekday: 'long'})}`;
}

export function formatMonth(iso: string): string {
  return fmt(iso, {month: 'long', year: 'numeric'});
}

export function formatPlace(event: HubEvent): string {
  if (event.online) return 'Online';
  return [event.city, event.country].filter(Boolean).join(', ');
}

/** Groups already-sorted items by a key, keeping first-seen order. */
export function groupBy<T>(items: T[], key: (item: T) => string): {key: string; items: T[]}[] {
  const groups = new Map<string, T[]>();
  items.forEach((item) => {
    const k = key(item);
    groups.set(k, [...(groups.get(k) ?? []), item]);
  });
  return [...groups].map(([k, groupItems]) => ({key: k, items: groupItems}));
}
