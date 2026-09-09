import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:window_manager/window_manager.dart';

import 'app/app.dart';
import 'core/single_instance.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Desktop-ui specification, "Second launch": bring the existing window
  // forward instead of opening a second instance.
  if (Platform.isWindows && !acquireSingleInstanceOrActivateExisting()) {
    exit(0);
  }

  await windowManager.ensureInitialized();
  const windowOptions = WindowOptions(
    size: Size(1200, 800),
    minimumSize: Size(900, 600),
    center: true,
    title: 'VPS Backup Manager',
  );
  unawaited(
    windowManager.waitUntilReadyToShow(windowOptions, () async {
      await windowManager.show();
      await windowManager.focus();
    }),
  );

  runApp(const ProviderScope(child: App()));
}
