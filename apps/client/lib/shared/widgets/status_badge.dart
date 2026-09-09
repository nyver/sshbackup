import 'package:flutter/material.dart';

/// A colored chip for a run/step status string (`PENDING`, `RUNNING`,
/// `SUCCESS`, `WARNING`, `FAILED`, `CANCELLED`, `SKIPPED`, `INTERRUPTED`).
/// Falls back to a neutral color for any status this build does not
/// recognize, so an older UI degrades gracefully against a newer service.
class StatusBadge extends StatelessWidget {
  const StatusBadge({required this.status, super.key});

  final String status;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final (background, foreground) = switch (status) {
      'SUCCESS' => (Colors.green.shade100, Colors.green.shade900),
      'WARNING' => (Colors.amber.shade100, Colors.amber.shade900),
      'FAILED' => (Colors.red.shade100, Colors.red.shade900),
      'CANCELLED' => (Colors.grey.shade300, Colors.grey.shade800),
      'SKIPPED' => (Colors.grey.shade300, Colors.grey.shade800),
      'INTERRUPTED' => (Colors.deepOrange.shade100, Colors.deepOrange.shade900),
      'RUNNING' => (scheme.primaryContainer, scheme.onPrimaryContainer),
      'PENDING' => (scheme.secondaryContainer, scheme.onSecondaryContainer),
      _ => (scheme.surfaceContainerHighest, scheme.onSurfaceVariant),
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: background,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(
        status,
        style: Theme.of(context).textTheme.labelSmall
            ?.copyWith(color: foreground, fontWeight: FontWeight.w600),
      ),
    );
  }
}
