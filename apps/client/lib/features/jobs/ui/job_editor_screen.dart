import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/ipc/ipc_client.dart';
import '../../../core/ipc/models.dart';
import '../../../l10n/gen/app_localizations.dart';
import '../../../shared/widgets/section_card.dart';
import '../../servers/data/servers_providers.dart';
import '../data/jobs_providers.dart';
import 'script_editor.dart';
import 'source_editor.dart';

/// The weekday numbering used by the wire contract: Go's `time.Weekday`
/// (Sunday = 0 .. Saturday = 6), not ISO-8601.
const _weekdayLabelsSundayFirst = [
  'Sun',
  'Mon',
  'Tue',
  'Wed',
  'Thu',
  'Fri',
  'Sat',
];

class JobEditorScreen extends ConsumerStatefulWidget {
  const JobEditorScreen({this.existing, super.key});

  final JobDto? existing;

  @override
  ConsumerState<JobEditorScreen> createState() => _JobEditorScreenState();
}

class _JobEditorScreenState extends ConsumerState<JobEditorScreen> {
  final _formKey = GlobalKey<FormState>();

  late final TextEditingController _name;
  late bool _enabled;
  String? _serverId;

  late List<SourceDto> _sources;
  late List<ScriptDto> _preScripts;
  late List<ScriptDto> _postScripts;

  late String _scheduleType;
  late int _hour;
  late int _minute;
  late Set<int> _weekdays;
  late int _dayOfMonth;
  late final TextEditingController _cronExpression;
  late String _missedRunPolicy;

  late final TextEditingController _archiveTimeoutSeconds;
  late final TextEditingController _remoteTempDirectory;
  late final TextEditingController _localDestination;

  late bool _healthCheckEnabled;
  late final TextEditingController _healthCheckCommand;
  late final TextEditingController _healthCheckAttempts;
  late final TextEditingController _healthCheckIntervalSeconds;

  late final TextEditingController _keepLast;
  late final TextEditingController _maxAgeDays;

  bool _saving = false;
  String? _errorMessage;

  bool get _isEdit => widget.existing != null;

  @override
  void initState() {
    super.initState();
    final e = widget.existing;
    _name = TextEditingController(text: e?.name ?? '');
    _enabled = e?.enabled ?? true;
    _serverId = e?.serverId;

    _sources = e?.sources ?? const [];
    _preScripts =
        e?.scripts.where((s) => s.type == 'PRE_BACKUP').toList() ?? const [];
    _postScripts =
        e?.scripts.where((s) => s.type == 'POST_BACKUP').toList() ?? const [];

    final schedule = e?.schedule;
    _scheduleType = schedule?.type ?? 'MANUAL';
    _hour = schedule?.hour ?? 2;
    _minute = schedule?.minute ?? 0;
    _weekdays = {...(schedule?.weekdays ?? const [])};
    _dayOfMonth = schedule?.dayOfMonth ?? 1;
    _cronExpression = TextEditingController(
      text: schedule?.cronExpression ?? '',
    );
    _missedRunPolicy = schedule?.missedRunPolicy ?? 'RUN_AS_SOON_AS_POSSIBLE';

    _archiveTimeoutSeconds = TextEditingController(
      text: (e?.archiveTimeoutSeconds ?? 3600).toString(),
    );
    _remoteTempDirectory = TextEditingController(
      text: e?.remoteTempDirectory ?? '/tmp/vps-backup-manager',
    );
    _localDestination = TextEditingController(text: e?.localDestination ?? '');

    _healthCheckEnabled = e?.healthCheck != null;
    _healthCheckCommand = TextEditingController(
      text: e?.healthCheck?.command ?? '',
    );
    _healthCheckAttempts = TextEditingController(
      text: (e?.healthCheck?.attempts ?? 3).toString(),
    );
    _healthCheckIntervalSeconds = TextEditingController(
      text: (e?.healthCheck?.intervalSeconds ?? 5).toString(),
    );

    _keepLast = TextEditingController(
      text: e?.retentionPolicy.keepLast?.toString() ?? '',
    );
    _maxAgeDays = TextEditingController(
      text: e?.retentionPolicy.maxAgeDays?.toString() ?? '',
    );
  }

  @override
  void dispose() {
    _name.dispose();
    _cronExpression.dispose();
    _archiveTimeoutSeconds.dispose();
    _remoteTempDirectory.dispose();
    _localDestination.dispose();
    _healthCheckCommand.dispose();
    _healthCheckAttempts.dispose();
    _healthCheckIntervalSeconds.dispose();
    _keepLast.dispose();
    _maxAgeDays.dispose();
    super.dispose();
  }

  Future<void> _pickDestination() async {
    final path = await FilePicker.getDirectoryPath();
    if (path != null && mounted) {
      setState(() => _localDestination.text = path);
    }
  }

  Future<void> _save() async {
    final l10n = AppLocalizations.of(context)!;
    if (!(_formKey.currentState?.validate() ?? false)) return;
    if (_serverId == null) {
      setState(() => _errorMessage = l10n.jobFieldServerRequired);
      return;
    }

    setState(() {
      _saving = true;
      _errorMessage = null;
    });

    final scripts = [
      for (var i = 0; i < _preScripts.length; i++)
        _withTypeAndPosition(_preScripts[i], 'PRE_BACKUP', i),
      for (var i = 0; i < _postScripts.length; i++)
        _withTypeAndPosition(_postScripts[i], 'POST_BACKUP', i),
    ];

    final job = JobDto(
      id: widget.existing?.id ?? '',
      name: _name.text.trim(),
      serverId: _serverId!,
      enabled: _enabled,
      sources: _sources,
      scripts: scripts,
      schedule: ScheduleDto(
        type: _scheduleType,
        hour: _hour,
        minute: _minute,
        weekdays: _weekdays.toList()..sort(),
        dayOfMonth: _dayOfMonth,
        cronExpression: _cronExpression.text.trim(),
        missedRunPolicy: _missedRunPolicy,
      ),
      retentionPolicy: RetentionPolicyDto(
        keepLast: int.tryParse(_keepLast.text.trim()),
        maxAgeDays: int.tryParse(_maxAgeDays.text.trim()),
      ),
      healthCheck: _healthCheckEnabled
          ? HealthCheckDto(
              command: _healthCheckCommand.text.trim(),
              attempts: int.tryParse(_healthCheckAttempts.text.trim()) ?? 3,
              intervalSeconds:
                  int.tryParse(_healthCheckIntervalSeconds.text.trim()) ?? 5,
            )
          : null,
      archiveFormat: 'tar.gz',
      remoteTempDirectory: _remoteTempDirectory.text.trim(),
      localDestination: _localDestination.text.trim(),
      archiveTimeoutSeconds:
          int.tryParse(_archiveTimeoutSeconds.text.trim()) ?? 3600,
      createdAt: widget.existing?.createdAt ?? '',
      updatedAt: widget.existing?.updatedAt ?? '',
    );

    try {
      final notifier = ref.read(jobsListProvider.notifier);
      await (_isEdit ? notifier.updateJob(job) : notifier.create(job));
      if (mounted) Navigator.of(context).pop(true);
    } on IpcException catch (e) {
      if (mounted) setState(() => _errorMessage = e.message);
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  ScriptDto _withTypeAndPosition(ScriptDto s, String type, int position) =>
      ScriptDto(
        type: type,
        command: s.command,
        position: position,
        timeoutSeconds: s.timeoutSeconds,
        runCondition: s.runCondition,
        criticalCleanup: s.criticalCleanup,
        retryOnFailure: s.retryOnFailure,
      );

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final servers = ref.watch(serversListProvider).value ?? const [];

    return Scaffold(
      appBar: AppBar(
        title: Text(_isEdit ? l10n.jobEditTitle : l10n.jobAddTitle),
        actions: [
          Padding(
            padding: const EdgeInsets.only(right: 16),
            child: FilledButton(
              key: const Key('job_save_button'),
              onPressed: _saving ? null : _save,
              child: _saving
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: Colors.white,
                      ),
                    )
                  : Text(l10n.save),
            ),
          ),
        ],
      ),
      body: Form(
        key: _formKey,
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            if (_errorMessage != null)
              Container(
                margin: const EdgeInsets.only(bottom: 16),
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: Theme.of(context).colorScheme.errorContainer,
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Text(
                  _errorMessage!,
                  style: TextStyle(
                    color: Theme.of(context).colorScheme.onErrorContainer,
                  ),
                ),
              ),
            SectionCard(
              title: l10n.jobSectionGeneral,
              children: [
                TextFormField(
                  key: const Key('job_name_field'),
                  controller: _name,
                  decoration: InputDecoration(labelText: l10n.jobFieldName),
                  validator: (v) => (v == null || v.trim().isEmpty)
                      ? l10n.fieldRequired
                      : null,
                ),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  value: _enabled,
                  onChanged: (v) => setState(() => _enabled = v),
                  title: Text(l10n.jobFieldEnabled),
                ),
              ],
            ),
            SectionCard(
              title: l10n.jobSectionServer,
              children: [
                DropdownButtonFormField<String>(
                  key: const Key('job_server_dropdown'),
                  initialValue: _serverId,
                  decoration: InputDecoration(labelText: l10n.jobFieldServer),
                  items: [
                    for (final s in servers)
                      DropdownMenuItem(value: s.id, child: Text(s.name)),
                  ],
                  onChanged: (v) => setState(() => _serverId = v),
                ),
                if (servers.isEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: Text(
                      l10n.jobNoServersHint,
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ),
              ],
            ),
            SectionCard(
              title: l10n.jobSectionSchedule,
              children: [_buildScheduleFields(context, l10n)],
            ),
            SectionCard(
              title: l10n.jobSectionBeforeBackup,
              children: [
                ScriptListEditor(
                  scripts: _preScripts,
                  onChanged: (v) => setState(() => _preScripts = v),
                ),
              ],
            ),
            SectionCard(
              title: l10n.jobSectionBackupSources,
              children: [
                SourceListEditor(
                  sources: _sources,
                  onChanged: (v) => setState(() => _sources = v),
                ),
              ],
            ),
            SectionCard(
              title: l10n.jobSectionArchive,
              subtitle: l10n.jobArchiveFormatFixed,
              children: [
                TextFormField(
                  controller: _archiveTimeoutSeconds,
                  keyboardType: TextInputType.number,
                  decoration: InputDecoration(
                    labelText: l10n.jobFieldArchiveTimeoutSeconds,
                  ),
                ),
              ],
            ),
            SectionCard(
              title: l10n.jobSectionDestination,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: TextFormField(
                        key: const Key('job_local_destination_field'),
                        controller: _localDestination,
                        decoration: InputDecoration(
                          labelText: l10n.jobFieldLocalDestination,
                        ),
                        validator: (v) => (v == null || v.trim().isEmpty)
                            ? l10n.fieldRequired
                            : null,
                      ),
                    ),
                    const SizedBox(width: 8),
                    IconButton(
                      tooltip: l10n.jobBrowseDestination,
                      icon: const Icon(Icons.folder_open),
                      onPressed: _pickDestination,
                    ),
                  ],
                ),
              ],
            ),
            SectionCard(
              title: l10n.jobSectionAfterBackup,
              children: [
                ScriptListEditor(
                  scripts: _postScripts,
                  onChanged: (v) => setState(() => _postScripts = v),
                ),
              ],
            ),
            SectionCard(
              title: l10n.jobSectionHealthCheck,
              children: [
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  value: _healthCheckEnabled,
                  onChanged: (v) => setState(() => _healthCheckEnabled = v),
                  title: Text(l10n.jobHealthCheckEnable),
                ),
                if (_healthCheckEnabled) ...[
                  TextFormField(
                    controller: _healthCheckCommand,
                    style: const TextStyle(fontFamily: 'monospace'),
                    decoration: InputDecoration(
                      labelText: l10n.scriptFieldCommand,
                    ),
                  ),
                  Row(
                    children: [
                      Expanded(
                        child: TextFormField(
                          controller: _healthCheckAttempts,
                          keyboardType: TextInputType.number,
                          decoration: InputDecoration(
                            labelText: l10n.jobFieldHealthCheckAttempts,
                          ),
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: TextFormField(
                          controller: _healthCheckIntervalSeconds,
                          keyboardType: TextInputType.number,
                          decoration: InputDecoration(
                            labelText: l10n.jobFieldHealthCheckIntervalSeconds,
                          ),
                        ),
                      ),
                    ],
                  ),
                ],
              ],
            ),
            SectionCard(
              title: l10n.jobSectionRetention,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: TextFormField(
                        controller: _keepLast,
                        keyboardType: TextInputType.number,
                        decoration: InputDecoration(
                          labelText: l10n.jobFieldKeepLast,
                          helperText: l10n.jobFieldOptionalHint,
                        ),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: TextFormField(
                        controller: _maxAgeDays,
                        keyboardType: TextInputType.number,
                        decoration: InputDecoration(
                          labelText: l10n.jobFieldMaxAgeDays,
                          helperText: l10n.jobFieldOptionalHint,
                        ),
                      ),
                    ),
                  ],
                ),
              ],
            ),
            SectionCard(
              title: l10n.jobSectionAdvanced,
              children: [
                TextFormField(
                  controller: _remoteTempDirectory,
                  style: const TextStyle(fontFamily: 'monospace'),
                  decoration: InputDecoration(
                    labelText: l10n.jobFieldRemoteTempDirectory,
                  ),
                  validator: (v) => (v == null || v.trim().isEmpty)
                      ? l10n.fieldRequired
                      : null,
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildScheduleFields(BuildContext context, AppLocalizations l10n) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        DropdownButtonFormField<String>(
          initialValue: _scheduleType,
          decoration: InputDecoration(labelText: l10n.jobFieldScheduleType),
          items: [
            DropdownMenuItem(value: 'MANUAL', child: Text(l10n.scheduleManual)),
            DropdownMenuItem(value: 'DAILY', child: Text(l10n.scheduleDaily)),
            DropdownMenuItem(value: 'WEEKLY', child: Text(l10n.scheduleWeekly)),
            DropdownMenuItem(
              value: 'MONTHLY',
              child: Text(l10n.scheduleMonthly),
            ),
            DropdownMenuItem(value: 'CRON', child: Text(l10n.scheduleCron)),
          ],
          onChanged: (v) => setState(() => _scheduleType = v ?? 'MANUAL'),
        ),
        const SizedBox(height: 12),
        if (_scheduleType == 'DAILY' ||
            _scheduleType == 'WEEKLY' ||
            _scheduleType == 'MONTHLY') ...[
          Row(
            children: [
              Expanded(
                child: TextFormField(
                  initialValue: _hour.toString(),
                  keyboardType: TextInputType.number,
                  decoration: InputDecoration(labelText: l10n.scheduleHour),
                  onChanged: (v) =>
                      _hour = int.tryParse(v)?.clamp(0, 23) ?? _hour,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: TextFormField(
                  initialValue: _minute.toString(),
                  keyboardType: TextInputType.number,
                  decoration: InputDecoration(labelText: l10n.scheduleMinute),
                  onChanged: (v) =>
                      _minute = int.tryParse(v)?.clamp(0, 59) ?? _minute,
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
        ],
        if (_scheduleType == 'WEEKLY') ...[
          Wrap(
            spacing: 8,
            children: [
              for (var day = 0; day < 7; day++)
                FilterChip(
                  label: Text(_weekdayLabelsSundayFirst[day]),
                  selected: _weekdays.contains(day),
                  onSelected: (selected) => setState(() {
                    if (selected) {
                      _weekdays.add(day);
                    } else {
                      _weekdays.remove(day);
                    }
                  }),
                ),
            ],
          ),
          const SizedBox(height: 12),
        ],
        if (_scheduleType == 'MONTHLY') ...[
          TextFormField(
            initialValue: _dayOfMonth.toString(),
            keyboardType: TextInputType.number,
            decoration: InputDecoration(labelText: l10n.scheduleDayOfMonth),
            onChanged: (v) =>
                _dayOfMonth = int.tryParse(v)?.clamp(1, 31) ?? _dayOfMonth,
          ),
          const SizedBox(height: 12),
        ],
        if (_scheduleType == 'CRON') ...[
          TextFormField(
            controller: _cronExpression,
            style: const TextStyle(fontFamily: 'monospace'),
            decoration: InputDecoration(
              labelText: l10n.scheduleCronExpression,
              helperText: l10n.scheduleCronExpressionHelp,
            ),
          ),
          const SizedBox(height: 12),
        ],
        if (_scheduleType != 'MANUAL')
          DropdownButtonFormField<String>(
            initialValue: _missedRunPolicy,
            decoration: InputDecoration(
              labelText: l10n.scheduleMissedRunPolicy,
            ),
            items: [
              DropdownMenuItem(
                value: 'RUN_AS_SOON_AS_POSSIBLE',
                child: Text(l10n.missedRunAsSoonAsPossible),
              ),
              DropdownMenuItem(value: 'SKIP', child: Text(l10n.missedRunSkip)),
            ],
            onChanged: (v) => setState(
              () => _missedRunPolicy = v ?? 'RUN_AS_SOON_AS_POSSIBLE',
            ),
          ),
      ],
    );
  }
}
