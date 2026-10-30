// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * An application that has joined an SSO session.
 *
 * @public
 */
export interface SessionParticipant {
  appId: string;
  appName?: string;
  firstJoinedAt: string;
  lastActiveAt: string;
}

/**
 * A live SSO session, as returned by the session management API.
 *
 * @public
 */
export interface Session {
  id: string;
  userId: string;
  userName?: string;
  userAgent?: string;
  creationIp?: string;
  lastActiveIp?: string;
  authenticatedAt: string;
  createdAt: string;
  lastActiveAt: string;
  idleExpiresAt?: string;
  absoluteExpiresAt?: string;
  participants: SessionParticipant[];
}

/**
 * One page of sessions.
 *
 * @public
 */
export interface SessionListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  sessions: Session[];
}

/**
 * Whose sessions to list: one user's, or the ones an application has joined.
 *
 * @public
 */
export type SessionFilter = {userId: string} | {appId: string};
