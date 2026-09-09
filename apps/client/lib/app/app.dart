import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';

import '../l10n/gen/app_localizations.dart';
import 'connection_gate.dart';
import 'shell.dart';
import 'theme.dart';
import 'tray_controller.dart';

/// The application root: theme, localization, and the single window.
///
/// [TrayController] wraps everything so the tray icon (open/run a
/// job/pause/exit) works regardless of connection state; [ConnectionGate]
/// then blocks the actual screens behind the background service being
/// reachable (desktop-ui specification, "UI contains no backup logic").
class App extends StatelessWidget {
  const App({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      onGenerateTitle: (context) => AppLocalizations.of(context)!.appTitle,
      debugShowCheckedModeBanner: false,
      theme: AppTheme.light,
      darkTheme: AppTheme.dark,
      localizationsDelegates: const [
        AppLocalizations.delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      supportedLocales: AppLocalizations.supportedLocales,
      home: const TrayController(child: ConnectionGate(child: AppShell())),
    );
  }
}
