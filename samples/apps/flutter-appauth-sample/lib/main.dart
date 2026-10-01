// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import 'package:flutter/material.dart';
import 'package:flutter_appauth/flutter_appauth.dart';

import 'auth_service.dart';

void main() {
  runApp(const SampleApp());
}

class SampleApp extends StatelessWidget {
  const SampleApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'ThunderID Flutter AppAuth Integration',
      theme: ThemeData(colorSchemeSeed: Colors.indigo),
      home: const HomePage(),
    );
  }
}

class HomePage extends StatefulWidget {
  const HomePage({super.key});

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  final AuthService _auth = AuthService();
  AuthSession? _session;
  bool _busy = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _run(() async => _session = await _auth.restore());
  }

  Future<void> _run(Future<void> Function() action) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await action();
    } on FlutterAppAuthUserCancelledException {
      // The user closed the browser. Nothing to report.
    } on FlutterAppAuthPlatformException catch (e) {
      _error = e.platformErrorDetails.errorDescription ?? e.message;
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _signIn() => _run(() async => _session = await _auth.signIn());

  void _refresh() => _run(() async {
        final session = _session!;
        _session = await _auth.refresh(session.refreshToken!, currentIdToken: session.idToken);
      });

  void _signOut() => _run(() async {
        final idToken = _session!.idToken;
        _session = null;
        await _auth.signOut(idToken);
      });

  @override
  Widget build(BuildContext context) {
    final session = _session;
    return Scaffold(
      appBar: AppBar(title: const Text('ThunderID Flutter AppAuth Integration')),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: _busy
              ? const CircularProgressIndicator()
              : Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    if (session == null)
                      FilledButton(onPressed: _signIn, child: const Text('Sign in'))
                    else ...[
                      Text(
                        'Signed in as ${session.claims['name'] ?? session.claims['email'] ?? session.claims['sub']}',
                        style: Theme.of(context).textTheme.titleMedium,
                      ),
                      const SizedBox(height: 8),
                      Text('Access token expires: ${session.accessTokenExpiry?.toLocal() ?? 'unknown'}'),
                      const SizedBox(height: 24),
                      OutlinedButton(
                        onPressed: session.refreshToken == null ? null : _refresh,
                        child: const Text('Refresh tokens'),
                      ),
                      const SizedBox(height: 8),
                      FilledButton(onPressed: _signOut, child: const Text('Sign out')),
                    ],
                    if (_error != null) ...[
                      const SizedBox(height: 16),
                      Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
                    ],
                  ],
                ),
        ),
      ),
    );
  }
}
