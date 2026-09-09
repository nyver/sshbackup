import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:vps_backup_manager/core/ipc/ipc_client.dart';
import 'package:vps_backup_manager/features/jobs/ui/job_editor_screen.dart';

import '../../../test_helpers/fake_ipc_client.dart';
import '../../../test_helpers/fixtures.dart';
import '../../../test_helpers/test_app.dart';

/// The job editor is a long scrollable form; a default 800x600 test
/// surface only builds the first section (`ListView` lazily builds
/// children near the viewport). A tall surface keeps every section built
/// so tests can reach fields without simulating scrolling.
void _useTallSurface(WidgetTester tester) {
  tester.view.physicalSize = const Size(1400, 6000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
}

Future<void> _fillRequiredFields(WidgetTester tester) async {
  await tester.enterText(find.byKey(const Key('job_name_field')), 'nightly');
  await tester.tap(find.byKey(const Key('job_server_dropdown')));
  await tester.pumpAndSettle();
  await tester.tap(find.text('prod').last);
  await tester.pumpAndSettle();
  await tester.enterText(
    find.byKey(const Key('job_local_destination_field')),
    r'D:\Backups',
  );
}

void main() {
  testWidgets('saving a new job sends the expected jobs.create payload', (
    tester,
  ) async {
    _useTallSurface(tester);
    final client = FakeIpcClient();
    client.setConnectionState(IpcConnectionState.connected);
    client.handlers['servers.list'] = (_) => {
      'servers': [serverJson()],
    };
    Map<String, dynamic>? sentJob;
    client.handlers['jobs.create'] = (payload) {
      sentJob =
          (payload! as Map<String, dynamic>)['job'] as Map<String, dynamic>;
      return {'job': jobJson(id: 'new-job', name: sentJob!['name'] as String)};
    };
    client.handlers['jobs.list'] = (_) => {'jobs': <Map<String, dynamic>>[]};

    await tester.pumpWidget(
      wrapForTest(
        Builder(
          builder: (context) => ElevatedButton(
            onPressed: () => Navigator.of(context).push(
              MaterialPageRoute<void>(builder: (_) => const JobEditorScreen()),
            ),
            child: const Text('open'),
          ),
        ),
        client,
      ),
    );

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await _fillRequiredFields(tester);

    await tester.tap(find.byKey(const Key('job_save_button')));
    await tester.pumpAndSettle();

    expect(sentJob, isNotNull);
    expect(sentJob!['name'], 'nightly');
    expect(sentJob!['server_id'], 's1');
    expect(sentJob!['local_destination'], r'D:\Backups');
    // The editor is popped after a successful save.
    expect(find.byType(JobEditorScreen), findsNothing);
  });

  testWidgets('a service error is shown without raw exception text', (
    tester,
  ) async {
    _useTallSurface(tester);
    final client = FakeIpcClient();
    client.setConnectionState(IpcConnectionState.connected);
    client.handlers['servers.list'] = (_) => {
      'servers': [serverJson()],
    };
    client.handlers['jobs.create'] = (_) {
      throw IpcException('INVALID_CONFIG', 'the cron expression is invalid');
    };

    await tester.pumpWidget(wrapForTest(const JobEditorScreen(), client));
    await tester.pumpAndSettle();

    await _fillRequiredFields(tester);

    await tester.tap(find.byKey(const Key('job_save_button')));
    await tester.pumpAndSettle();

    expect(find.text('the cron expression is invalid'), findsOneWidget);
  });
}
