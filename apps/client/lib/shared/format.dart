/// Formatting helpers shared across screens: byte sizes, durations, and
/// timestamps. Kept dependency-free (no `intl.NumberFormat`/`DateFormat`
/// locale plumbing) since only English is shipped for MVP `0.1`.
library;

const _byteUnits = ['B', 'KB', 'MB', 'GB', 'TB'];

/// Formats a byte count as e.g. `1.5 MB`. Values under 1 KB are shown as a
/// whole number of bytes.
String formatBytes(int bytes) {
  if (bytes < 1024) return '$bytes B';
  var value = bytes.toDouble();
  var unitIndex = 0;
  while (value >= 1024 && unitIndex < _byteUnits.length - 1) {
    value /= 1024;
    unitIndex++;
  }
  return '${value.toStringAsFixed(value < 10 ? 2 : 1)} ${_byteUnits[unitIndex]}';
}

/// Formats a duration as `1h 02m 03s`, `02m 03s`, or `3s`, dropping leading
/// zero components.
String formatDuration(Duration d) {
  final hours = d.inHours;
  final minutes = d.inMinutes.remainder(60);
  final seconds = d.inSeconds.remainder(60);
  if (hours > 0) {
    return '${hours}h ${minutes.toString().padLeft(2, '0')}m '
        '${seconds.toString().padLeft(2, '0')}s';
  }
  if (minutes > 0) {
    return '${minutes}m ${seconds.toString().padLeft(2, '0')}s';
  }
  return '${seconds}s';
}

/// Formats an RFC3339 UTC timestamp (as returned over IPC) for display in
/// the host's local time. Returns an empty string for an empty input and
/// the raw input if it fails to parse, rather than throwing.
String formatTimestamp(String rfc3339) {
  if (rfc3339.isEmpty) return '';
  final parsed = DateTime.tryParse(rfc3339);
  if (parsed == null) return rfc3339;
  final local = parsed.toLocal();
  String two(int n) => n.toString().padLeft(2, '0');
  return '${local.year}-${two(local.month)}-${two(local.day)} '
      '${two(local.hour)}:${two(local.minute)}:${two(local.second)}';
}

/// The wall-clock duration between two RFC3339 timestamps, or null if
/// either is missing/unparseable.
Duration? durationBetween(String startedAtRfc3339, String finishedAtRfc3339) {
  if (startedAtRfc3339.isEmpty || finishedAtRfc3339.isEmpty) return null;
  final start = DateTime.tryParse(startedAtRfc3339);
  final end = DateTime.tryParse(finishedAtRfc3339);
  if (start == null || end == null) return null;
  return end.difference(start);
}
