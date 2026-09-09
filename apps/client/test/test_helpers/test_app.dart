import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:vps_backup_manager/core/ipc_providers.dart';
import 'package:vps_backup_manager/l10n/gen/app_localizations.dart';

import 'fake_ipc_client.dart';

/// Wraps [child] with the `ProviderScope`/`MaterialApp`/localization
/// boilerplate every screen test needs (matching `app/app.dart`'s real
/// delegate list), with [client] standing in for the real IPC connection.
Widget wrapForTest(Widget child, FakeIpcClient client) {
  return ProviderScope(
    overrides: [ipcClientProvider.overrideWithValue(client)],
    child: MaterialApp(
      localizationsDelegates: const [
        AppLocalizations.delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      supportedLocales: AppLocalizations.supportedLocales,
      home: child,
    ),
  );
}
