import 'package:flutter_test/flutter_test.dart';
import 'package:vps_backup_manager/core/ipc/ipc_client.dart';
import 'package:vps_backup_manager/features/runs/ui/run_details_screen.dart';

import '../../../test_helpers/fake_ipc_client.dart';
import '../../../test_helpers/fixtures.dart';
import '../../../test_helpers/test_app.dart';

void main() {
  testWidgets('shows run status, archive info, and steps', (tester) async {
    final client = FakeIpcClient();
    client.setConnectionState(IpcConnectionState.connected);
    client.handlers['runs.get'] = (_) => {
      'run': runJson(),
      'steps': [stepJson()],
    };

    await tester.pumpWidget(
      wrapForTest(const RunDetailsScreen(runId: 'r1'), client),
    );
    await tester.pumpAndSettle();

    expect(find.text('SUCCESS'), findsWidgets);
    expect(find.text('nightly-20260102-020000.tar.gz'), findsOneWidget);
    expect(find.text('ARCHIVE'), findsOneWidget);
    // A terminal run offers no cancel action.
    expect(find.text('Cancel run'), findsNothing);
  });

  testWidgets('a failed script step shows its command and starts expanded', (
    tester,
  ) async {
    final client = FakeIpcClient();
    client.setConnectionState(IpcConnectionState.connected);
    client.handlers['runs.get'] = (_) => {
      'run': runJson(status: 'FAILED'),
      'steps': [
        stepJson(
          type: 'PRE_BACKUP_SCRIPT',
          status: 'FAILED',
          command: 'docker compose down',
        ),
      ],
    };

    await tester.pumpWidget(
      wrapForTest(const RunDetailsScreen(runId: 'r1'), client),
    );
    await tester.pumpAndSettle();

    // The command shows in the collapsed subtitle and, since the step
    // starts expanded on failure, also in the expanded command block —
    // so it appears twice rather than needing a manual expand first.
    expect(find.text('docker compose down'), findsNWidgets(2));
  });

  testWidgets('an active run offers cancel, which sends runs.cancel', (
    tester,
  ) async {
    final client = FakeIpcClient();
    client.setConnectionState(IpcConnectionState.connected);
    client.handlers['runs.get'] = (_) => {
      'run': runJson(status: 'RUNNING', finishedAt: ''),
      'steps': <Map<String, dynamic>>[],
    };
    var cancelled = false;
    client.handlers['runs.cancel'] = (payload) {
      expect((payload! as Map<String, dynamic>)['run_id'], 'r1');
      cancelled = true;
      return <String, dynamic>{};
    };

    await tester.pumpWidget(
      wrapForTest(const RunDetailsScreen(runId: 'r1'), client),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Cancel run'));
    await tester.pumpAndSettle();

    expect(cancelled, isTrue);
  });
}
