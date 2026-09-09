import 'package:flutter/material.dart';

import '../../../core/ipc/ipc_client.dart';
import '../../../core/ipc/models.dart';
import '../../../l10n/gen/app_localizations.dart';
import '../../../shared/format.dart';
import '../domain/jobs_repository.dart';

/// Runs `jobs.validate` and shows per-check results (desktop-ui
/// specification, "Run now and validate").
Future<void> showValidateJobDialog(
  BuildContext context, {
  required JobsRepository repository,
  required String jobId,
}) {
  return showDialog<void>(
    context: context,
    builder: (context) =>
        _ValidateJobDialog(repository: repository, jobId: jobId),
  );
}

class _ValidateJobDialog extends StatefulWidget {
  const _ValidateJobDialog({required this.repository, required this.jobId});

  final JobsRepository repository;
  final String jobId;

  @override
  State<_ValidateJobDialog> createState() => _ValidateJobDialogState();
}

class _ValidateJobDialogState extends State<_ValidateJobDialog> {
  late Future<ValidateJobResult> _future;

  @override
  void initState() {
    super.initState();
    _future = widget.repository.validate(widget.jobId);
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return AlertDialog(
      title: Text(l10n.jobValidateTitle),
      content: SizedBox(
        width: 420,
        child: FutureBuilder<ValidateJobResult>(
          future: _future,
          builder: (context, snapshot) {
            if (snapshot.connectionState != ConnectionState.done) {
              return const SizedBox(
                height: 80,
                child: Center(child: CircularProgressIndicator()),
              );
            }
            if (snapshot.hasError) {
              // Desktop-ui "Error presentation": never show raw exception
              // text; an IpcException already carries a safe message.
              final error = snapshot.error;
              return Text(
                error is IpcException
                    ? error.message
                    : l10n.genericErrorMessage,
              );
            }
            final result = snapshot.data!;
            return SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  for (final check in result.checks)
                    ListTile(
                      contentPadding: EdgeInsets.zero,
                      leading: Icon(
                        check.passed ? Icons.check_circle : Icons.cancel,
                        color: check.passed
                            ? Colors.green
                            : Theme.of(context).colorScheme.error,
                      ),
                      title: Text(check.name),
                      subtitle: check.message.isEmpty
                          ? null
                          : Text(check.message),
                    ),
                  const Divider(),
                  if (result.sourceSizeKnown)
                    Text(
                      l10n.jobValidateSourceSize(
                        formatBytes(result.sourceSizeBytes),
                      ),
                    ),
                  if (result.remoteFreeKnown)
                    Text(
                      l10n.jobValidateRemoteFree(
                        formatBytes(result.remoteFreeBytes),
                      ),
                    ),
                  if (result.localFreeKnown)
                    Text(
                      l10n.jobValidateLocalFree(
                        formatBytes(result.localFreeBytes),
                      ),
                    ),
                ],
              ),
            );
          },
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l10n.ok),
        ),
      ],
    );
  }
}
