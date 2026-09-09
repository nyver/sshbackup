import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:vps_backup_manager/app/app.dart';
import 'package:vps_backup_manager/core/ipc/ipc_client.dart';
import 'package:vps_backup_manager/core/ipc_providers.dart';

import 'test_helpers/fake_ipc_client.dart';

void main() {
  setUp(() {
    // The tray icon lives behind a platform channel with no test-host
    // implementation; answer every call with null so TrayController's
    // initState doesn't throw MissingPluginException.
    final binding = TestWidgetsFlutterBinding.ensureInitialized();
    binding.defaultBinaryMessenger.setMockMethodCallHandler(
      const MethodChannel('tray_manager'),
      (call) async => null,
    );
  });

  testWidgets(
    'App shows the service-unavailable state when the service is unreachable',
    (WidgetTester tester) async {
      final client = FakeIpcClient()
        ..setConnectionState(IpcConnectionState.disconnected);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [ipcClientProvider.overrideWithValue(client)],
          child: const App(),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Background service unavailable'), findsOneWidget);
      expect(find.text('Retry'), findsOneWidget);
    },
  );

  testWidgets('App shows the dashboard once the service connects', (
    WidgetTester tester,
  ) async {
    final client = FakeIpcClient()
      ..setConnectionState(IpcConnectionState.connected)
      ..handlers['servers.list'] = (_) => {'servers': <Map<String, dynamic>>[]};

    await tester.pumpWidget(
      ProviderScope(
        overrides: [ipcClientProvider.overrideWithValue(client)],
        child: const App(),
      ),
    );
    await tester.pumpAndSettle();

    // "Dashboard" appears both as the nav rail label and the app bar title.
    expect(find.text('Dashboard'), findsWidgets);
    expect(
      find.text('No servers yet. Add a server to get started.'),
      findsOneWidget,
    );
  });
}
