import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:vps_backup_manager/core/ipc/ipc_client.dart';
import 'package:vps_backup_manager/features/jobs/ui/job_editor_screen.dart';

import '../test_helpers/fake_ipc_client.dart';
import '../test_helpers/fixtures.dart';
import '../test_helpers/test_app.dart';

void main() {
  // Golden images are rendered with this host's fonts and are therefore
  // sensitive to the OS/Flutter-SDK font rendering pipeline (a well-known
  // limitation of Flutter golden tests, not specific to this screen).
  // Regenerate with `flutter test --update-goldens` after an intentional
  // layout change, and expect a diff when comparing across hosts.
  testWidgets('job editor layout (General/Server/Schedule sections)', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1000, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    final client = FakeIpcClient();
    client.setConnectionState(IpcConnectionState.connected);
    client.handlers['servers.list'] = (_) => {
      'servers': [serverJson()],
    };

    await tester.pumpWidget(wrapForTest(const JobEditorScreen(), client));
    await tester.pumpAndSettle();

    await expectLater(
      find.byType(JobEditorScreen),
      matchesGoldenFile('job_editor_screen.png'),
    );
  });
}
