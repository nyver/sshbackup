import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tray_manager/tray_manager.dart';
import 'package:window_manager/window_manager.dart';

import '../features/jobs/data/jobs_providers.dart';
import '../features/runs/data/runs_providers.dart';
import '../features/settings/data/settings_providers.dart';
import '../l10n/gen/app_localizations.dart';

/// Tray icon with Open/Run a job/Pause schedules/Exit UI (desktop-ui
/// specification, "Tray integration"). Exiting the UI never stops the
/// background service — it only closes this window and tray icon.
class TrayController extends ConsumerStatefulWidget {
  const TrayController({required this.child, super.key});

  final Widget child;

  @override
  ConsumerState<TrayController> createState() => _TrayControllerState();
}

class _TrayControllerState extends ConsumerState<TrayController>
    with TrayListener {
  @override
  void initState() {
    super.initState();
    trayManager.addListener(this);
    unawaited(_initTray());
  }

  Future<void> _initTray() async {
    await trayManager.setIcon('assets/tray_icon.ico');
    await trayManager.setToolTip('VPS Backup Manager');
    await _refreshMenu();
  }

  Future<void> _refreshMenu() async {
    if (!mounted) return;
    final l10n = AppLocalizations.of(context)!;
    final jobs = ref.read(jobsListProvider).value ?? const [];
    final settings = ref.read(settingsProvider).value;

    await trayManager.setContextMenu(
      Menu(
        items: [
          MenuItem(key: 'open', label: l10n.trayOpen, onClick: (_) => _open()),
          MenuItem.separator(),
          MenuItem.submenu(
            key: 'run_job',
            label: l10n.trayRunJob,
            disabled: jobs.isEmpty,
            submenu: Menu(
              items: [
                for (final job in jobs)
                  MenuItem(
                    key: 'run_job:${job.id}',
                    label: job.name,
                    onClick: (_) => _runJob(job.id),
                  ),
              ],
            ),
          ),
          MenuItem.separator(),
          MenuItem.checkbox(
            key: 'pause',
            label: l10n.trayPauseSchedules,
            checked: settings?.schedulesPaused ?? false,
            disabled: settings == null,
            onClick: (_) => _togglePause(),
          ),
          MenuItem.separator(),
          MenuItem(
            key: 'exit_ui',
            label: l10n.trayExitUi,
            onClick: (_) => _exitUi(),
          ),
        ],
      ),
    );
  }

  Future<void> _open() async {
    await windowManager.show();
    await windowManager.focus();
  }

  Future<void> _runJob(String jobId) async {
    // Best-effort: this is a convenience shortcut with no UI to surface an
    // error to. Failures still show up in run history and via notifications.
    try {
      await ref.read(runsRepositoryProvider).start(jobId);
    } on Object {
      // Intentionally ignored — see comment above.
    }
  }

  Future<void> _togglePause() async {
    final settings = ref.read(settingsProvider).value;
    if (settings == null) return;
    await ref
        .read(settingsProvider.notifier)
        .setSchedulesPaused(!settings.schedulesPaused);
  }

  Future<void> _exitUi() async {
    await trayManager.destroy();
    await windowManager.destroy();
    exit(0);
  }

  @override
  void onTrayIconMouseDown() {
    unawaited(_open());
  }

  @override
  void onTrayIconRightMouseDown() {
    unawaited(trayManager.popUpContextMenu());
  }

  @override
  void dispose() {
    trayManager.removeListener(this);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(jobsListProvider, (_, _) => unawaited(_refreshMenu()));
    ref.listen(settingsProvider, (_, _) => unawaited(_refreshMenu()));
    return widget.child;
  }
}
