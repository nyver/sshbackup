import 'package:flutter_test/flutter_test.dart';
import 'package:vps_backup_manager/shared/format.dart';

void main() {
  group('formatBytes', () {
    test('shows bytes under 1 KB as a whole number', () {
      expect(formatBytes(512), '512 B');
    });

    test('shows kilobytes with two decimals under 10', () {
      expect(formatBytes(1536), '1.50 KB');
    });

    test('shows megabytes with one decimal at or above 10', () {
      expect(formatBytes(15 * 1024 * 1024), '15.0 MB');
    });

    test('caps at the largest known unit', () {
      const huge = 5 * 1024 * 1024 * 1024 * 1024; // 5 TB
      expect(formatBytes(huge), '5.00 TB');
    });
  });

  group('formatDuration', () {
    test('drops leading zero components', () {
      expect(formatDuration(const Duration(seconds: 9)), '9s');
      expect(formatDuration(const Duration(minutes: 2, seconds: 3)), '2m 03s');
      expect(
        formatDuration(const Duration(hours: 1, minutes: 2, seconds: 3)),
        '1h 02m 03s',
      );
    });
  });

  group('formatTimestamp', () {
    test('returns empty string for empty input', () {
      expect(formatTimestamp(''), '');
    });

    test('returns the raw input for unparseable text', () {
      expect(formatTimestamp('not-a-date'), 'not-a-date');
    });

    test('formats a valid RFC3339 timestamp', () {
      expect(
        formatTimestamp('2026-01-02T02:00:00Z'),
        matches(RegExp(r'^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$')),
      );
    });
  });

  group('durationBetween', () {
    test('returns null when either timestamp is missing', () {
      expect(durationBetween('', '2026-01-01T00:00:00Z'), isNull);
      expect(durationBetween('2026-01-01T00:00:00Z', ''), isNull);
    });

    test('computes the wall-clock difference', () {
      final d = durationBetween('2026-01-02T02:00:00Z', '2026-01-02T02:05:30Z');
      expect(d, const Duration(minutes: 5, seconds: 30));
    });
  });
}
