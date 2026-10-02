// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';

import 'package:flutter_appauth/flutter_appauth.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'auth_config.dart';

/// Tokens from the latest sign-in or refresh.
class AuthSession {
  const AuthSession({
    required this.accessToken,
    required this.idToken,
    this.refreshToken,
    this.accessTokenExpiry,
  });

  final String accessToken;
  final String idToken;
  final String? refreshToken;
  final DateTime? accessTokenExpiry;

  /// Claims from the ID token payload. The token was validated by AppAuth when
  /// it was issued, so it is only decoded here.
  Map<String, dynamic> get claims {
    final payload = idToken.split('.')[1];
    return jsonDecode(utf8.decode(base64Url.decode(base64Url.normalize(payload))))
        as Map<String, dynamic>;
  }
}

/// Wraps flutter_appauth for sign-in, token refresh, and sign-out against ThunderID.
class AuthService {
  static const FlutterAppAuth _appAuth = FlutterAppAuth();
  static const FlutterSecureStorage _storage = FlutterSecureStorage();
  static const String _refreshTokenKey = 'thunderid_refresh_token';

  /// Runs the authorization code flow with PKCE in the system browser and
  /// exchanges the code for tokens.
  Future<AuthSession> signIn() async {
    final response = await _appAuth.authorizeAndExchangeCode(
      AuthorizationTokenRequest(
        AuthConfig.clientId,
        AuthConfig.redirectUrl,
        discoveryUrl: AuthConfig.discoveryUrl,
        scopes: AuthConfig.scopes,
      ),
    );
    return _save(response);
  }

  /// Exchanges a refresh token for a new set of tokens.
  Future<AuthSession> refresh(String refreshToken, {String? currentIdToken}) async {
    final response = await _appAuth.token(
      TokenRequest(
        AuthConfig.clientId,
        AuthConfig.redirectUrl,
        discoveryUrl: AuthConfig.discoveryUrl,
        refreshToken: refreshToken,
        scopes: AuthConfig.scopes,
      ),
    );
    return _save(response, fallbackIdToken: currentIdToken);
  }

  /// Restores a session from the stored refresh token, or returns null when
  /// there is none or it is no longer valid.
  Future<AuthSession?> restore() async {
    final refreshToken = await _storage.read(key: _refreshTokenKey);
    if (refreshToken == null) return null;
    try {
      return await refresh(refreshToken);
    } on FlutterAppAuthPlatformException {
      await _storage.delete(key: _refreshTokenKey);
      return null;
    }
  }

  /// Ends the ThunderID session in the browser and forgets the local tokens.
  Future<void> signOut(String idToken) async {
    try {
      await _appAuth.endSession(
        EndSessionRequest(
          idTokenHint: idToken,
          postLogoutRedirectUrl: AuthConfig.postLogoutRedirectUrl,
          discoveryUrl: AuthConfig.discoveryUrl,
        ),
      );
    } finally {
      await _storage.delete(key: _refreshTokenKey);
    }
  }

  Future<AuthSession> _save(TokenResponse response, {String? fallbackIdToken}) async {
    // ThunderID rotates refresh tokens: each refresh revokes the one it used,
    // so always keep the newest.
    if (response.refreshToken != null) {
      await _storage.write(key: _refreshTokenKey, value: response.refreshToken);
    }
    return AuthSession(
      accessToken: response.accessToken!,
      idToken: response.idToken ?? fallbackIdToken!,
      refreshToken: response.refreshToken,
      accessTokenExpiry: response.accessTokenExpirationDateTime,
    );
  }
}
