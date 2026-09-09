import 'package:flutter/material.dart';

import '../../../core/ipc/models.dart';
import '../../../l10n/gen/app_localizations.dart';

/// Editor for a job's backup sources: remote path plus optional include and
/// exclude glob patterns (backup-jobs specification).
class SourceListEditor extends StatelessWidget {
  const SourceListEditor({
    required this.sources,
    required this.onChanged,
    super.key,
  });

  final List<SourceDto> sources;
  final ValueChanged<List<SourceDto>> onChanged;

  void _updateAt(int index, SourceDto source) {
    final next = [...sources];
    next[index] = source;
    onChanged(next);
  }

  void _removeAt(int index) {
    final next = [...sources]..removeAt(index);
    onChanged([
      for (var i = 0; i < next.length; i++)
        SourceDto(
          remotePath: next[i].remotePath,
          position: i,
          include: next[i].include,
          exclude: next[i].exclude,
        ),
    ]);
  }

  void _add() {
    onChanged([
      ...sources,
      SourceDto(remotePath: '', position: sources.length),
    ]);
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (var i = 0; i < sources.length; i++)
          _SourceRow(
            // Keyed by index, not by ObjectKey(source) — see the same
            // note in script_editor.dart: a new SourceDto instance per
            // keystroke would otherwise destroy and recreate the field's
            // Element, dropping focus while typing.
            key: ValueKey(i),
            source: sources[i],
            onChanged: (s) => _updateAt(i, s),
            onRemove: () => _removeAt(i),
          ),
        OutlinedButton.icon(
          onPressed: _add,
          icon: const Icon(Icons.add),
          label: Text(l10n.sourceAdd),
        ),
      ],
    );
  }
}

class _SourceRow extends StatefulWidget {
  const _SourceRow({
    required this.source,
    required this.onChanged,
    required this.onRemove,
    super.key,
  });

  final SourceDto source;
  final ValueChanged<SourceDto> onChanged;
  final VoidCallback onRemove;

  @override
  State<_SourceRow> createState() => _SourceRowState();
}

class _SourceRowState extends State<_SourceRow> {
  bool _expanded = false;

  SourceDto _copyWith({
    String? remotePath,
    List<String>? include,
    List<String>? exclude,
  }) => SourceDto(
    remotePath: remotePath ?? widget.source.remotePath,
    position: widget.source.position,
    include: include ?? widget.source.include,
    exclude: exclude ?? widget.source.exclude,
  );

  List<String> _parsePatterns(String text) =>
      text.split(',').map((s) => s.trim()).where((s) => s.isNotEmpty).toList();

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
            Row(
              children: [
                Expanded(
                  child: TextFormField(
                    initialValue: widget.source.remotePath,
                    decoration: InputDecoration(
                      labelText: l10n.sourceFieldRemotePath,
                      isDense: true,
                    ),
                    style: const TextStyle(fontFamily: 'monospace'),
                    onChanged: (v) =>
                        widget.onChanged(_copyWith(remotePath: v)),
                  ),
                ),
                IconButton(
                  tooltip: l10n.sourceAdvanced,
                  icon: Icon(_expanded ? Icons.expand_less : Icons.tune),
                  onPressed: () => setState(() => _expanded = !_expanded),
                ),
                IconButton(
                  tooltip: l10n.delete,
                  icon: const Icon(Icons.delete_outline),
                  onPressed: widget.onRemove,
                ),
              ],
            ),
            if (_expanded) ...[
              const SizedBox(height: 8),
              TextFormField(
                initialValue: widget.source.include.join(', '),
                decoration: InputDecoration(
                  labelText: l10n.sourceFieldInclude,
                  helperText: l10n.sourcePatternHelp,
                  isDense: true,
                ),
                onChanged: (v) =>
                    widget.onChanged(_copyWith(include: _parsePatterns(v))),
              ),
              const SizedBox(height: 8),
              TextFormField(
                initialValue: widget.source.exclude.join(', '),
                decoration: InputDecoration(
                  labelText: l10n.sourceFieldExclude,
                  helperText: l10n.sourcePatternHelp,
                  isDense: true,
                ),
                onChanged: (v) =>
                    widget.onChanged(_copyWith(exclude: _parsePatterns(v))),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
