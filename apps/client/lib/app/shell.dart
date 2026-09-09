import 'package:flutter/material.dart';

import '../features/dashboard/ui/dashboard_screen.dart';
import '../features/jobs/ui/jobs_screen.dart';
import '../features/runs/ui/history_screen.dart';
import '../features/servers/ui/servers_screen.dart';
import '../features/settings/ui/settings_screen.dart';
import '../l10n/gen/app_localizations.dart';

/// The main navigation shell: a rail on the left (desktop-appropriate,
/// unlike a mobile bottom bar) and one of the five top-level screens on
/// the right, kept alive via [IndexedStack] so switching tabs does not
/// lose scroll position or in-flight state.
class AppShell extends StatefulWidget {
  const AppShell({super.key});

  @override
  State<AppShell> createState() => _AppShellState();
}

class _AppShellState extends State<AppShell> {
  int _index = 0;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return Scaffold(
      body: Row(
        children: [
          NavigationRail(
            selectedIndex: _index,
            onDestinationSelected: (i) => setState(() => _index = i),
            labelType: NavigationRailLabelType.all,
            destinations: [
              NavigationRailDestination(
                icon: const Icon(Icons.dashboard_outlined),
                selectedIcon: const Icon(Icons.dashboard),
                label: Text(l10n.navDashboard),
              ),
              NavigationRailDestination(
                icon: const Icon(Icons.dns_outlined),
                selectedIcon: const Icon(Icons.dns),
                label: Text(l10n.navServers),
              ),
              NavigationRailDestination(
                icon: const Icon(Icons.event_repeat_outlined),
                selectedIcon: const Icon(Icons.event_repeat),
                label: Text(l10n.navJobs),
              ),
              NavigationRailDestination(
                icon: const Icon(Icons.history_outlined),
                selectedIcon: const Icon(Icons.history),
                label: Text(l10n.navHistory),
              ),
              NavigationRailDestination(
                icon: const Icon(Icons.settings_outlined),
                selectedIcon: const Icon(Icons.settings),
                label: Text(l10n.navSettings),
              ),
            ],
          ),
          const VerticalDivider(width: 1),
          Expanded(
            child: IndexedStack(
              index: _index,
              children: const [
                DashboardScreen(),
                ServersScreen(),
                JobsScreen(),
                HistoryScreen(),
                SettingsScreen(),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
