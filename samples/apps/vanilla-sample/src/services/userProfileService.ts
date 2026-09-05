// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

export interface UserProfile {
    id: string;
    ouId?: string;
    ouHandle?: string;
    type?: string;
    display?: string;
    isReadOnly?: boolean;
    attributes: Record<string, unknown>;
}

const getErrorMessage = async (response: Response, fallback: string) => {
    const errorData = await response.json().catch(() => ({})) as {
        message?: { defaultValue?: string } | string;
        description?: { defaultValue?: string } | string;
    };

    if (typeof errorData.message === 'string' && errorData.message) {
        return errorData.message;
    }

    if (typeof errorData.description === 'string' && errorData.description) {
        return errorData.description;
    }

    if (typeof errorData.message === 'object' && errorData.message?.defaultValue) {
        return errorData.message.defaultValue;
    }

    if (typeof errorData.description === 'object' && errorData.description?.defaultValue) {
        return errorData.description.defaultValue;
    }

    return fallback;
};

const normalizeUserProfile = (data: Partial<UserProfile>): UserProfile => ({
    id: data.id || '',
    ouId: data.ouId,
    ouHandle: data.ouHandle,
    type: data.type,
    display: data.display,
    isReadOnly: data.isReadOnly,
    attributes: data.attributes && typeof data.attributes === 'object' ? data.attributes : {},
});

/**
 * Fetches the current user's profile via the same-origin proxy. No bearer token is passed from
 * here; the proxy attaches the session's auth assertion server-side.
 */
export const getCurrentUserProfile = async (): Promise<UserProfile> => {
    const response = await fetch('/api/profile', {
        method: 'GET',
        headers: { Accept: 'application/json' },
    });

    if (!response.ok) {
        throw new Error(await getErrorMessage(response, 'Failed to load user profile.'));
    }

    return normalizeUserProfile(await response.json() as Partial<UserProfile>);
};

export const updateCurrentUserProfile = async (
    attributes: Record<string, unknown>
): Promise<UserProfile> => {
    const response = await fetch('/api/profile', {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json',
            Accept: 'application/json',
        },
        body: JSON.stringify({ attributes }),
    });

    if (!response.ok) {
        throw new Error(await getErrorMessage(response, 'Failed to update user profile.'));
    }

    return normalizeUserProfile(await response.json() as Partial<UserProfile>);
};

export const updateCurrentUserPassword = async (password: string, currentPassword: string): Promise<void> => {
    const response = await fetch('/api/profile/password', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            Accept: 'application/json',
        },
        body: JSON.stringify({
            currentPassword,
            attributes: {
                password,
            },
        }),
    });

    if (!response.ok) {
        throw new Error(await getErrorMessage(response, 'Failed to update password.'));
    }
};
