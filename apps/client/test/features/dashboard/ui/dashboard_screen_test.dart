import 'package:flutter_test/flutter_test.dart';
import 'package:vps_backup_manager/core/ipc/ipc_client.dart';
import 'package:vps_backup_manager/features/dashboard/ui/dashboard_screen.dart';

import '../../../test_helpers/fake_ipc_client.dart';
import '../../../test_helpers/fixtures.dart';
import '../../../test_helpers/test_app.dart';

void main() {
  testWidgets('shows the empty state when there are no servers', (
    tester,
  ) async {
    final client = FakeIpcClient();
    client.setConnectionState(IpcConnectionState.connected);
    client.handlers['servers.list'] = (_) => {
      'servers': <Map<String, dynamic>>[],
    };
    client.handlers['jobs.list'] = (_) => {'jobs': <Map<String, dynamic>>[]};
    client.handlers['runs.list'] = (_) => {'runs': <Map<String, dynamic>>[]};

    await tester.pumpWidget(wrapForTest(const DashboardScreen(), client));
    await tester.pumpAndSettle();

    expect(
      find.text('No servers yet. Add a server to get started.'),
      findsOneWidget,
    );
  });

  testWidgets('shows upcoming jobs and recent runs once servers exist', (
    tester,
  ) async {
    final client = FakeIpcClient();
    client.setConnectionState(IpcConnectionState.connected);
    client.handlers['servers.list'] = (_) => {
      'servers': [serverJson()],
    };
    client.handlers['jobs.list'] = (_) => {
      'jobs': [jobJson(nextRunAt: '2026-06-01T02:00:00Z')],
    };
    client.handlers['runs.list'] = (_) => {
      'runs': [runJson()],
    };

    await tester.pumpWidget(wrapForTest(const DashboardScreen(), client));
    await tester.pumpAndSettle();

    expect(find.text('nightly'), findsOneWidget);
    expect(
      find.text('No servers yet. Add a server to get started.'),
      findsNothing,
    );
  });
}
