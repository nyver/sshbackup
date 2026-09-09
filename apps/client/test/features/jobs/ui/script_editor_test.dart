import 'package:flutter_test/flutter_test.dart';
import 'package:vps_backup_manager/features/jobs/ui/script_editor.dart';

void main() {
  group('looksDestructive', () {
    test('flags common destructive commands', () {
      expect(looksDestructive('rm -rf /var/www/*'), isTrue);
      expect(looksDestructive('docker compose down'), isTrue);
      expect(looksDestructive('DROP TABLE users;'), isTrue);
    });

    test('does not flag ordinary backup commands', () {
      expect(looksDestructive('tar -czf backup.tar.gz /var/www'), isFalse);
      expect(looksDestructive('systemctl reload nginx'), isFalse);
    });
  });
}
