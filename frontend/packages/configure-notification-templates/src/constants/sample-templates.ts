// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {NotificationChannel, TemplateColorScheme} from '../models/notification-template';

/**
 * A ready-made starting point offered in the create wizard. Content uses the same token grammar as
 * the editor — {{t(key)}}, {{design(token)}} (email body only), {{ctx(key)}} — and the {{ctx}} keys
 * mirror the runtime values the backend executors inject (`appName`, `otpCode`, `expiryTime`,
 * `magicLink`, `inviteLink`, …); see `constants/template-tokens.ts`.
 *
 * These mirror the default `notification_template` resources the backend seeds via
 * `backend/cmd/server/bootstrap/01-default-resources.yaml`, so the samples offered here match what
 * the product ships. Hardcoded (like the flow templates in `@thunderid/configure-flows`'s
 * `data/templates.json`) until a sample-template catalog API exists; keep in sync with the bootstrap.
 *
 * @public
 */
export interface SampleNotificationTemplate {
  /** Stable id, used as the card key and selection value — `${channel}-${handle}`. */
  id: string;
  /** Channel this sample applies to. */
  channel: NotificationChannel;
  /** Catalog card copy. */
  display: {
    label: string;
    description: string;
  };
  /** Subject line (email only). */
  subject?: string;
  /** Body — HTML for email, plain text for SMS. */
  body: string;
  /** Color scheme the body is authored against (email only). */
  colorScheme?: TemplateColorScheme;
}

const USER_INVITE_EMAIL_BODY = `<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #181818;">
  <h2>You've Been Invited!</h2>
  <p>You have been invited to create your account. Click the link below to complete your registration:</p>
  <p><a href="{{ctx(inviteLink)}}" style="display: inline-block; padding: 10px 20px;
    background-color: #3a87ed; color: #fff; text-decoration: none;
    border-radius: 4px;">Complete Registration</a></p>
  <p>If the button above doesn't work, copy and paste the following link into your browser:</p>
  <p><a href="{{ctx(inviteLink)}}">{{ctx(inviteLink)}}</a></p>
  <p>If you did not expect this invitation, you can safely ignore this email.</p>
</body>
</html>
`;

const SELF_REGISTRATION_EMAIL_BODY = `<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #181818;">
  <h2>Complete Your Registration for {{ctx(appName)}}</h2>
  <p>You recently started the registration process for <strong>{{ctx(appName)}}</strong>. Click the link below to complete your account setup:</p>
  <p><a href="{{ctx(inviteLink)}}" style="display: inline-block; padding: 10px 20px;
    background-color: #3a87ed; color: #fff; text-decoration: none;
    border-radius: 4px;">Complete Registration</a></p>
  <p>If the button above doesn't work, copy and paste the following link into your browser:</p>
  <p><a href="{{ctx(inviteLink)}}">{{ctx(inviteLink)}}</a></p>
  <p>If you did not initiate this request, you can safely ignore this email.</p>
</body>
</html>
`;

const MAGIC_LINK_EMAIL_BODY = `<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #181818;">
  <h2>Sign in to your account</h2>
  <p>Use the secure link below to sign in:</p>
  <p><a href="{{ctx(magicLink)}}" style="display: inline-block; padding: 10px 20px;
    background-color: #3a87ed; color: #fff; text-decoration: none;
    border-radius: 4px;">Sign in</a></p>
  <p>This link expires in {{ctx(expiryTime)}}.</p>
  <p>If the button above doesn't work, copy and paste the following link into your browser:</p>
  <p style="word-break: break-all;"><a href="{{ctx(magicLink)}}">{{ctx(magicLink)}}</a></p>
  <p>If you did not request this, you can safely ignore this email.</p>
</body>
</html>
`;

const PASSWORD_RECOVERY_EMAIL_BODY = `<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #181818;">
  <h2>Reset Your Password</h2>
  <p>Hello,</p>
  <p>A request was made to reset your account password for {{ctx(appName)}}. Please use the button below to set a new password:</p>
  <p>
    <a href="{{ctx(inviteLink)}}" style="display: inline-block; padding: 12px 24px;
    background-color: #3a87ed; color: #ffffff; text-decoration: none;
    border-radius: 4px; font-weight: bold;">
      Reset Password
    </a>
  </p>
  <p>If the button doesn't work, copy and paste this link into your browser:</p>
  <p style="word-break: break-all;">
    <a href="{{ctx(inviteLink)}}">{{ctx(inviteLink)}}</a>
  </p>
  <p>This link will expire in <strong>30 minutes</strong> for your security.</p>
  <p>If you did not request a password reset, you can safely ignore this email, your account will remain unchanged.</p>
</body>
</html>
`;

const CIBA_NOTIFICATION_EMAIL_BODY = `<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #181818;">
  <h2>Authentication Request</h2>
  <p><strong>{{ctx(appName)}}</strong> is requesting to authenticate you.</p>
  <p style="padding: 12px; background-color: #f5f5f5; border-left: 4px solid #3a87ed;
    border-radius: 4px; margin: 16px 0;">{{ctx(bindingMessage)}}</p>
  <p>If you recognize this request, click the link below to approve:</p>
  <p><a href="{{ctx(inviteLink)}}" style="display: inline-block; padding: 10px 20px;
    background-color: #3a87ed; color: #fff; text-decoration: none;
    border-radius: 4px;">Approve Authentication</a></p>
  <p>If the button above doesn't work, copy and paste the following link into your browser:</p>
  <p><a href="{{ctx(inviteLink)}}">{{ctx(inviteLink)}}</a></p>
  <p>If you did not initiate this request, you can safely ignore this email.</p>
</body>
</html>
`;

const OTP_EMAIL_BODY = `<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #181818;">
  <h2>Your verification code</h2>
  <p>Use the following code to complete your verification:</p>
  <table cellpadding="0" cellspacing="0" border="0">
    <tr>
      <td style="font-size: 32px; font-weight: bold; letter-spacing: 8px; color: #3a87ed; background: #f0f4ff; border: 1px solid #c0d0f0; border-radius: 6px; padding: 12px 24px;">{{ctx(otpCode)}}</td>
    </tr>
  </table>
  <p>This code will expire in {{ctx(expiryTime)}}.</p>
  <p>If you did not request this, you can safely ignore this email.</p>
</body>
</html>
`;

/**
 * Hardcoded sample templates, mirroring the backend bootstrap defaults (per channel + handle). Keep
 * the {{ctx(...)}} keys aligned with the runtime executor keys so a sample resolves on send.
 *
 * @public
 */
export const SAMPLE_TEMPLATES: readonly SampleNotificationTemplate[] = [
  // ── Email ──────────────────────────────────────────────────────────────
  {
    id: 'email-user-invite',
    channel: 'email',
    display: {label: 'User Invitation', description: 'Sent to invite a new user to complete their registration.'},
    subject: "You're Invited to Register",
    body: USER_INVITE_EMAIL_BODY,
    colorScheme: 'light',
  },
  {
    id: 'email-self-registration',
    channel: 'email',
    display: {label: 'Self Registration Invite', description: 'Sent to complete a self-service registration.'},
    subject: 'Complete Your Registration for {{ctx(appName)}}',
    body: SELF_REGISTRATION_EMAIL_BODY,
    colorScheme: 'light',
  },
  {
    id: 'email-magic-link',
    channel: 'email',
    display: {label: 'Magic Link Sign-In', description: 'Sent to let a user sign in through a secure link.'},
    subject: 'Sign in to your account',
    body: MAGIC_LINK_EMAIL_BODY,
    colorScheme: 'light',
  },
  {
    id: 'email-password-recovery',
    channel: 'email',
    display: {label: 'Password Recovery', description: 'Sent to let a user reset their account password.'},
    subject: 'Reset Your Password',
    body: PASSWORD_RECOVERY_EMAIL_BODY,
    colorScheme: 'light',
  },
  {
    id: 'email-ciba-notification',
    channel: 'email',
    display: {
      label: 'CIBA Authentication Notification',
      description: 'Sent to approve a backchannel authentication request.',
    },
    subject: 'Authentication Request from {{ctx(appName)}}',
    body: CIBA_NOTIFICATION_EMAIL_BODY,
    colorScheme: 'light',
  },
  {
    id: 'email-otp',
    channel: 'email',
    display: {
      label: 'Email OTP Verification',
      description: "Sent to verify a user's email address with a one-time passcode.",
    },
    subject: 'Your verification code',
    body: OTP_EMAIL_BODY,
    colorScheme: 'light',
  },
  // ── SMS ────────────────────────────────────────────────────────────────
  {
    id: 'sms-user-invite',
    channel: 'sms',
    display: {label: 'User Invitation', description: 'Sent to invite a new user to complete their registration.'},
    body: "You've been invited to register. Complete your account setup here: {{ctx(inviteLink)}}",
  },
  {
    id: 'sms-self-registration',
    channel: 'sms',
    display: {label: 'Self Registration Invite', description: 'Sent to complete a self-service registration.'},
    body: '{{ctx(appName)}}: Finish your registration by clicking here: {{ctx(inviteLink)}}',
  },
  {
    id: 'sms-ciba-notification',
    channel: 'sms',
    display: {
      label: 'CIBA Authentication Notification',
      description: 'Sent to approve a backchannel authentication request.',
    },
    body: '{{ctx(appName)}}: {{ctx(bindingMessage)}} Tap to approve: {{ctx(inviteLink)}}',
  },
  {
    id: 'sms-otp',
    channel: 'sms',
    display: {
      label: 'SMS OTP Verification',
      description: "Sent to verify a user's phone number with a one-time passcode.",
    },
    body: 'Your verification code is: {{ctx(otpCode)}}. It expires in {{ctx(expiryTime)}}.',
  },
];

/** Returns the hardcoded samples applicable to a channel. */
export function getSampleTemplates(channel: NotificationChannel): SampleNotificationTemplate[] {
  return SAMPLE_TEMPLATES.filter((sample) => sample.channel === channel);
}
