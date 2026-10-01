# ThunderID Flutter AppAuth Sample

A minimal Flutter app that signs users in to ThunderID with [flutter_appauth](https://pub.dev/packages/flutter_appauth), a third-party OpenID Connect plugin that wraps AppAuth for iOS and Android. It uses the authorization code flow with PKCE in the system browser, refreshes tokens, and signs out through ThunderID's end session endpoint.

The full walkthrough is the [Flutter AppAuth guide](https://thunderid.dev/sdks-and-tools/flutter-appauth) in the ThunderID docs.

## Prerequisites

- Flutter 3.38.1 or later
- An Android emulator (API 24+) or an iOS Simulator (iOS 13+)
- A running ThunderID instance (default: `https://localhost:8090`)

## Configure ThunderID

1. In the Console, go to **Applications**, click **Add Application**, and choose **Flutter**.
2. Under **Sign-In Approach**, choose **Redirect to ThunderID Gate**.
3. Under **Deep Link / Universal Link**, enter `com.example.appauth://callback`.
4. Create the application and copy its **Client ID**.
5. On the **Advanced** tab, add `com.example.appauth://logout` under **Post-Logout Redirect URIs**.
6. On the **Token** tab, open **User**, then **ID Token**, and add the attributes the app should see, such as `email` and `name`.

## Trust the Local Certificate

A local ThunderID instance serves a self-signed certificate from `<THUNDERID_HOME>/config/certs/server.cert`. The device must trust it before the browser or AppAuth can reach the server.

**Android emulator**

```bash
adb reverse tcp:8090 tcp:8090
adb push <THUNDERID_HOME>/config/certs/server.cert /sdcard/Download/thunderid-dev.crt
```

Then install it on the emulator under **Settings > Security & privacy > More security & privacy > Encryption & credentials > Install a certificate > CA certificate**, and pick `thunderid-dev.crt` from **Downloads**. Older Android versions list **Encryption & credentials** directly under **Settings > Security**. Debug builds of this sample trust user-installed certificates through `android/app/src/debug/res/xml/network_security_config.xml`; release builds do not.

**iOS Simulator**

```bash
xcrun simctl keychain booted add-root-cert <THUNDERID_HOME>/config/certs/server.cert
```

## Run

```bash
flutter pub get
flutter run --dart-define=THUNDERID_CLIENT_ID=<your-client-id>
```

For an instance that is not at `https://localhost:8090`, also pass `--dart-define=THUNDERID_ISSUER=<issuer-url>`.

## Where Things Are

| File | What it does |
|------|--------------|
| `lib/auth_config.dart` | Issuer, client ID, redirect URIs, and scopes |
| `lib/auth_service.dart` | Sign-in, refresh, session restore, and sign-out with flutter_appauth |
| `lib/main.dart` | A single screen that drives the service |
| `android/app/build.gradle.kts` | Registers the `com.example.appauth` redirect scheme (`appAuthRedirectScheme`) |
| `android/app/src/main/AndroidManifest.xml` | `MainActivity` without the `android:taskAffinity=""` that `flutter create` adds, which would stop the redirect from completing sign-in |
| `ios/Runner/Info.plist` | Registers the same scheme under `CFBundleURLTypes` |

To use your own scheme, change it in all three of `auth_config.dart`, `build.gradle.kts`, and `Info.plist`, keep it lowercase, and register the new URIs on the ThunderID application.
