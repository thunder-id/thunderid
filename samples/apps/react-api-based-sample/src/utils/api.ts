/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { getDirectAuthHeaders, getConfig } from '../config';

export interface OrganizationUnit {
  id: string;
  handle: string;
  name: string;
  description?: string;
}

interface OrganizationUnitListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  organizationUnits: OrganizationUnit[];
}

const cachedOrganizationUnits: Record<string, string> = {};

async function fetchOrganizationUnits(): Promise<OrganizationUnit[]> {
  const { baseUrl } = getConfig();

  const response = await fetch(`${baseUrl}/organization-units`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error('Failed to fetch organization units');
  }

  const data: OrganizationUnitListResponse = await response.json();
  return data.organizationUnits;
}

export async function getOrganizationUnitId(handle: string): Promise<string> {
  if (cachedOrganizationUnits[handle]) {
    return cachedOrganizationUnits[handle];
  }

  const organizationUnits = await fetchOrganizationUnits();

  if (organizationUnits.length === 0) {
    throw new Error('No organization units found');
  }

  const ou = organizationUnits.find((ou) => ou.handle === handle);

  if (!ou) {
    throw new Error(`Organization unit "${handle}" not found`);
  }

  cachedOrganizationUnits[handle] = ou.id;
  return ou.id;
}

export async function getDefaultOrganizationUnitId(): Promise<string> {
  return getOrganizationUnitId('default');
}

export async function getCustomersOrganizationUnitId(): Promise<string> {
  return getOrganizationUnitId('customers');
}

export interface User {
  id: string;
  ouId: string;
  type: string;
  attributes: {
    username?: string;
    given_name?: string;
    family_name?: string;
    email?: string;
    [key: string]: unknown;
  };
}

interface UserListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  users: User[];
}

export async function fetchUsers(filter?: string): Promise<User[]> {
  const { baseUrl } = getConfig();

  let url = `${baseUrl}/users?limit=100`;
  if (filter) {
    url += `&filter=${encodeURIComponent(filter)}`;
  }

  const response = await fetch(url, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error('Failed to fetch users');
  }

  const data: UserListResponse = await response.json();
  return data.users;
}

/**
 * Fetches a single user by ID
 * @param userId - The user ID to fetch
 * @returns Promise with user data
 */
export async function fetchUserById(userId: string): Promise<User> {
  const { baseUrl } = getConfig();

  const response = await fetch(`${baseUrl}/users/${userId}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    if (response.status === 404) {
      throw new Error('User not found');
    }
    throw new Error('Failed to fetch user');
  }

  return await response.json();
}

// SMS OTP Step-up Authentication APIs

export interface SendOTPResponse {
  status: string;
  session_token: string;
}

export interface VerifyOTPResponse {
  id: string;
  type: string;
  ouId?: string;
  assertion: string;
}

export interface ApiError {
  code: string;
  message: { defaultValue?: string };
  description?: { defaultValue?: string };
}

/**
 * Sends an SMS OTP to the user's mobile number
 * @param senderId - The notification sender ID configured in ThunderID
 * @param recipient - The mobile number to send OTP to (e.g., +1234567890)
 * @returns Promise with session token for OTP verification
 */
export async function sendSMSOTP(
  senderId: string,
  recipient: string,
): Promise<SendOTPResponse> {
  const { baseUrl } = getConfig();

  const response = await fetch(`${baseUrl}/auth/otp/sms/send`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...getDirectAuthHeaders(),
    },
    body: JSON.stringify({
      sender_id: senderId,
      recipient: recipient,
    }),
  });

  if (!response.ok) {
    let errorMessage = 'Failed to send OTP';
    try {
      const contentType = response.headers.get('content-type');
      if (contentType && contentType.includes('application/json')) {
        const errorData: ApiError = await response.json();
        errorMessage = errorData.message?.defaultValue || errorMessage;
      } else {
        errorMessage = `HTTP ${response.status}: ${response.statusText}`;
      }
    } catch {
      errorMessage = `HTTP ${response.status}: ${response.statusText}`;
    }
    throw new Error(errorMessage);
  }

  return await response.json();
}

/**
 * Verifies the SMS OTP and enriches the assertion token with step-up authentication
 * @param sessionToken - Session token received from sendSMSOTP
 * @param otp - The OTP code entered by the user
 * @param existingAssertion - The existing assertion token to enrich
 * @returns Promise with enriched assertion token
 */
export async function verifySMSOTP(
  sessionToken: string,
  otp: string,
  existingAssertion: string,
): Promise<VerifyOTPResponse> {
  const { baseUrl } = getConfig();

  const response = await fetch(`${baseUrl}/auth/otp/sms/verify`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...getDirectAuthHeaders(),
    },
    body: JSON.stringify({
      session_token: sessionToken,
      otp: otp,
      assertion: existingAssertion,
    }),
  });

  if (!response.ok) {
    let errorMessage = 'OTP verification failed';
    try {
      const contentType = response.headers.get('content-type');
      if (contentType && contentType.includes('application/json')) {
        const errorData: ApiError = await response.json();
        errorMessage = errorData.message?.defaultValue || errorMessage;
      } else {
        errorMessage = `HTTP ${response.status}: ${response.statusText}`;
      }
    } catch {
      errorMessage = `HTTP ${response.status}: ${response.statusText}`;
    }
    throw new Error(errorMessage);
  }

  return await response.json();
}
