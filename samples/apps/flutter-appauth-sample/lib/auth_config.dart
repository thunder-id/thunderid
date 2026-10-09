// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/// Connection settings for the ThunderID application this sample signs in to.
///
/// Pass `THUNDERID_CLIENT_ID` (and, for a non-local instance, `THUNDERID_ISSUER`)
/// with `--dart-define` when you run the app.
class AuthConfig {
  static const String issuer = String.fromEnvironment(
    'THUNDERID_ISSUER',
    defaultValue: 'https://localhost:8090',
  );
  static const String clientId = String.fromEnvironment('THUNDERID_CLIENT_ID');

  static const String discoveryUrl = '$issuer/.well-known/openid-configuration';

  // Both must be registered on the ThunderID application. The scheme must also
  // match `appAuthRedirectScheme` (Android) and `CFBundleURLSchemes` (iOS).
  static const String redirectUrl = 'com.example.appauth://callback';
  static const String postLogoutRedirectUrl = 'com.example.appauth://logout';

  static const List<String> scopes = ['openid', 'profile', 'email'];
}
