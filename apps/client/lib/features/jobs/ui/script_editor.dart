import 'package:flutter/material.dart';

import '../../../core/ipc/models.dart';
import '../../../l10n/gen/app_localizations.dart';

/// Substrings that make a script command worth an unobtrusive heads-up
/// (desktop-ui specification, "Destructive command hint"). Matching is
/// advisory only: the command is never blocked or modified.
const _destructivePatterns = [
  'rm -rf',
  'rm -r',
  ' rm ',
  'docker compose down',
  'docker-compose down',
  'drop table',
  'drop database',
  'mkfs',
  'dd if=',
  '> /dev/',
  'shutdown',
  'reboot',
];

bool looksDestructive(String command) {
  final lower = ' ${command.toLowerCase()} ';
  return _destructivePatterns.any(lower.contains);
}

/// Editor for one script phase's list (`PRE_BACKUP` or `POST_BACKUP`):
/// command text, timeout, run condition, and the critical-cleanup flag
/// (desktop-ui specification, "Jobs list and job editor"). Always shows the
/// remote-execution permissions notice and never blocks or rewrites a
/// command, even a destructive-looking one ("Script safety warnings").
class ScriptListEditor extends StatelessWidget {
  const ScriptListEditor({
    required this.scripts,
    required this.onChanged,
    super.key,
  });

  final List<ScriptDto> scripts;
  final ValueChanged<List<ScriptDto>> onChanged;

  void _updateAt(int index, ScriptDto script) {
    final next = [...scripts];
    next[index] = script;
    onChanged(next);
  }

  void _removeAt(int index) {
    final next = [...scripts]..removeAt(index);
    onChanged(_reindexed(next));
  }

  void _add() {
    onChanged([
      ...scripts,
      ScriptDto(
        type: '',
        command: '',
        position: scripts.length,
        timeoutSeconds: 300,
        runCondition: 'ON_SUCCESS',
        criticalCleanup: false,
        retryOnFailure: false,
      ),
    ]);
  }

  List<ScriptDto> _reindexed(List<ScriptDto> list) => [
    for (var i = 0; i < list.length; i++)
      ScriptDto(
        type: list[i].type,
        command: list[i].command,
        position: i,
        timeoutSeconds: list[i].timeoutSeconds,
        runCondition: list[i].runCondition,
        criticalCleanup: list[i].criticalCleanup,
        retryOnFailure: list[i].retryOnFailure,
      ),
  ];

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          padding: const EdgeInsets.all(8),
          decoration: BoxDecoration(
            color: Theme.of(context).colorScheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(6),
          ),
          child: Row(
            children: [
              const Icon(Icons.info_outline, size: 18),
              const SizedBox(width: 8),
              Expanded(child: Text(l10n.scriptPermissionsNotice)),
            ],
          ),
        ),
        const SizedBox(height: 12),
        for (var i = 0; i < scripts.length; i++)
          _ScriptCard(
            // Keyed by index, not by ObjectKey(script): a keystroke in the
            // command field replaces the script with a new instance (see
            // _copyWith), and ObjectKey would then treat this as a brand
            // new element on every character — destroying and recreating
            // the TextFormField's Element and dropping focus mid-typing.
            key: ValueKey(i),
            script: scripts[i],
            onChanged: (s) => _updateAt(i, s),
            onRemove: () => _removeAt(i),
          ),
        OutlinedButton.icon(
          onPressed: _add,
          icon: const Icon(Icons.add),
          label: Text(l10n.scriptAdd),
        ),
      ],
    );
  }
}

class _ScriptCard extends StatelessWidget {
  const _ScriptCard({
    required this.script,
    required this.onChanged,
    required this.onRemove,
    super.key,
  });

  final ScriptDto script;
  final ValueChanged<ScriptDto> onChanged;
  final VoidCallback onRemove;

  ScriptDto _copyWith({
    String? command,
    int? timeoutSeconds,
    String? runCondition,
    bool? criticalCleanup,
    bool? retryOnFailure,
  }) => ScriptDto(
    type: script.type,
    command: command ?? script.command,
    position: script.position,
    timeoutSeconds: timeoutSeconds ?? script.timeoutSeconds,
    runCondition: runCondition ?? script.runCondition,
    criticalCleanup: criticalCleanup ?? script.criticalCleanup,
    retryOnFailure: retryOnFailure ?? script.retryOnFailure,
  );

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            TextFormField(
              initialValue: script.command,
              maxLines: 3,
              minLines: 1,
              style: const TextStyle(fontFamily: 'monospace'),
              decoration: InputDecoration(
                labelText: l10n.scriptFieldCommand,
                isDense: true,
              ),
              onChanged: (v) => onChanged(_copyWith(command: v)),
            ),
            if (looksDestructive(script.command)) ...[
              const SizedBox(height: 6),
              Row(
                children: [
                  Icon(
                    Icons.warning_amber_rounded,
                    size: 16,
                    color: Colors.amber.shade800,
                  ),
                  const SizedBox(width: 6),
                  Expanded(
                    child: Text(
                      l10n.scriptDestructiveHint,
                      style: Theme.of(context).textTheme.bodySmall
                          ?.copyWith(color: Colors.amber.shade800),
                    ),
                  ),
                ],
              ),
            ],
            const SizedBox(height: 8),
            Wrap(
              spacing: 12,
              runSpacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                SizedBox(
                  width: 140,
                  child: TextFormField(
                    initialValue: script.timeoutSeconds.toString(),
                    keyboardType: TextInputType.number,
                    decoration: InputDecoration(
                      labelText: l10n.scriptFieldTimeoutSeconds,
                      isDense: true,
                    ),
                    onChanged: (v) {
                      final seconds = int.tryParse(v);
                      if (seconds != null) {
                        onChanged(_copyWith(timeoutSeconds: seconds));
                      }
                    },
                  ),
                ),
                SizedBox(
                  width: 200,
                  child: DropdownButtonFormField<String>(
                    initialValue: script.runCondition,
                    decoration: InputDecoration(
                      labelText: l10n.scriptFieldRunCondition,
                      isDense: true,
                    ),
                    items: [
                      DropdownMenuItem(
                        value: 'ON_SUCCESS',
                        child: Text(l10n.runConditionOnSuccess),
                      ),
                      DropdownMenuItem(
                        value: 'ON_FAILURE',
                        child: Text(l10n.runConditionOnFailure),
                      ),
                      DropdownMenuItem(
                        value: 'ALWAYS',
                        child: Text(l10n.runConditionAlways),
                      ),
                    ],
                    onChanged: (v) {
                      if (v != null) onChanged(_copyWith(runCondition: v));
                    },
                  ),
                ),
                FilterChip(
                  label: Text(l10n.scriptFieldCriticalCleanup),
                  selected: script.criticalCleanup,
                  onSelected: (v) => onChanged(_copyWith(criticalCleanup: v)),
                ),
                FilterChip(
                  label: Text(l10n.scriptFieldRetryOnFailure),
                  selected: script.retryOnFailure,
                  onSelected: (v) => onChanged(_copyWith(retryOnFailure: v)),
                ),
                IconButton(
                  tooltip: l10n.delete,
                  icon: const Icon(Icons.delete_outline),
                  onPressed: onRemove,
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
