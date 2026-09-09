import 'package:flutter/material.dart';

import '../../core/ipc/ipc_client.dart';
import '../../l10n/gen/app_localizations.dart';

/// Renders any error as a plain, actionable message with a retry button —
/// never raw exception text, per the desktop-ui "Error presentation"
/// requirement. [error] is typically an [IpcException] (structured code +
/// message safe to show as-is) but any object is accepted so callers never
/// need a try/catch just to render this widget.
class ErrorView extends StatelessWidget {
  const ErrorView({required this.error, this.onRetry, super.key});

  final Object error;
  final VoidCallback? onRetry;

  String _message(BuildContext context) {
    final err = error;
    if (err is IpcException) return err.message;
    return AppLocalizations.of(context)!.genericErrorMessage;
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.error_outline,
              size: 40,
              color: Theme.of(context).colorScheme.error,
            ),
            const SizedBox(height: 12),
            Text(
              _message(context),
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodyLarge,
            ),
            if (onRetry != null) ...[
              const SizedBox(height: 16),
              FilledButton.tonal(onPressed: onRetry, child: Text(l10n.retry)),
            ],
          ],
        ),
      ),
    );
  }
}
