import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/ipc/ipc_client.dart';
import '../../../core/ipc/models.dart';
import '../../../l10n/gen/app_localizations.dart';
import '../data/servers_providers.dart';
import '../domain/servers_repository.dart';

/// Opens the add/edit server form as a dialog. Returns the saved server, or
/// null if the user cancelled.
Future<ServerDto?> showServerEditorDialog(
  BuildContext context, {
  ServerDto? existing,
}) {
  return showDialog<ServerDto>(
    context: context,
    builder: (context) => _ServerEditorDialog(existing: existing),
  );
}

class _ServerEditorDialog extends ConsumerStatefulWidget {
  const _ServerEditorDialog({this.existing});

  final ServerDto? existing;

  @override
  ConsumerState<_ServerEditorDialog> createState() =>
      _ServerEditorDialogState();
}

class _ServerEditorDialogState extends ConsumerState<_ServerEditorDialog> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _name;
  late final TextEditingController _host;
  late final TextEditingController _port;
  late final TextEditingController _username;
  late final TextEditingController _privateKeyPath;

  /// The one secret this server's credential holds: the private key's
  /// passphrase when [_authType] is `PRIVATE_KEY`, or the login password
  /// when it is `PASSWORD` — mirroring `SaveServerRequest.Passphrase` on
  /// the wire, which is reused the same way rather than adding a second
  /// field for what is, underneath, always "the one stored secret".
  late final TextEditingController _secret;
  late String _authType;
  bool _saving = false;
  String? _errorMessage;

  bool get _isEdit => widget.existing != null;
  bool get _isPrivateKey => _authType == 'PRIVATE_KEY';

  @override
  void initState() {
    super.initState();
    final e = widget.existing;
    _name = TextEditingController(text: e?.name ?? '');
    _host = TextEditingController(text: e?.host ?? '');
    _port = TextEditingController(text: (e?.port ?? 22).toString());
    _username = TextEditingController(text: e?.username ?? '');
    _privateKeyPath = TextEditingController();
    _secret = TextEditingController();
    _authType = e?.authType ?? 'PRIVATE_KEY';
  }

  @override
  void dispose() {
    _name.dispose();
    _host.dispose();
    _port.dispose();
    _username.dispose();
    _privateKeyPath.dispose();
    _secret.dispose();
    super.dispose();
  }

  Future<void> _pickPrivateKey() async {
    final file = await FilePicker.pickFile();
    final path = file?.path;
    if (path != null && mounted) {
      setState(() => _privateKeyPath.text = path);
    }
  }

  Future<void> _save() async {
    final l10n = AppLocalizations.of(context)!;
    if (!_isEdit && _isPrivateKey && _privateKeyPath.text.trim().isEmpty) {
      setState(() => _errorMessage = l10n.serverPrivateKeyRequired);
      return;
    }
    if (!_isEdit && !_isPrivateKey && _secret.text.isEmpty) {
      setState(() => _errorMessage = l10n.serverPasswordRequired);
      return;
    }
    if (!(_formKey.currentState?.validate() ?? false)) return;

    setState(() {
      _saving = true;
      _errorMessage = null;
    });
    final input = SaveServerInput(
      name: _name.text.trim(),
      host: _host.text.trim(),
      port: int.parse(_port.text.trim()),
      username: _username.text.trim(),
      authType: _authType,
      privateKeyPath: _isPrivateKey ? _privateKeyPath.text.trim() : '',
      passphrase: _secret.text,
    );
    try {
      final notifier = ref.read(serversListProvider.notifier);
      final server = _isEdit
          ? await notifier.updateServer(widget.existing!.id, input)
          : await notifier.create(input);
      if (mounted) Navigator.of(context).pop(server);
    } on IpcException catch (e) {
      if (mounted) setState(() => _errorMessage = e.message);
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return AlertDialog(
      title: Text(_isEdit ? l10n.serverEditTitle : l10n.serverAddTitle),
      content: SizedBox(
        width: 420,
        child: Form(
          key: _formKey,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                TextFormField(
                  controller: _name,
                  decoration: InputDecoration(labelText: l10n.serverFieldName),
                  validator: (v) => (v == null || v.trim().isEmpty)
                      ? l10n.fieldRequired
                      : null,
                ),
                const SizedBox(height: 12),
                Row(
                  children: [
                    Expanded(
                      flex: 3,
                      child: TextFormField(
                        controller: _host,
                        decoration: InputDecoration(
                          labelText: l10n.serverFieldHost,
                        ),
                        validator: (v) => (v == null || v.trim().isEmpty)
                            ? l10n.fieldRequired
                            : null,
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: TextFormField(
                        controller: _port,
                        decoration: InputDecoration(
                          labelText: l10n.serverFieldPort,
                        ),
                        keyboardType: TextInputType.number,
                        validator: (v) {
                          final port = int.tryParse(v?.trim() ?? '');
                          if (port == null || port < 1 || port > 65535) {
                            return l10n.serverFieldPortInvalid;
                          }
                          return null;
                        },
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _username,
                  decoration: InputDecoration(
                    labelText: l10n.serverFieldUsername,
                  ),
                  validator: (v) => (v == null || v.trim().isEmpty)
                      ? l10n.fieldRequired
                      : null,
                ),
                const SizedBox(height: 12),
                SegmentedButton<String>(
                  segments: [
                    ButtonSegment(
                      value: 'PRIVATE_KEY',
                      label: Text(l10n.serverAuthPrivateKey),
                      icon: const Icon(Icons.vpn_key_outlined),
                    ),
                    ButtonSegment(
                      value: 'PASSWORD',
                      label: Text(l10n.serverAuthPassword),
                      icon: const Icon(Icons.password_outlined),
                    ),
                  ],
                  selected: {_authType},
                  onSelectionChanged: (selection) =>
                      setState(() => _authType = selection.first),
                ),
                const SizedBox(height: 12),
                if (_isPrivateKey) ...[
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Expanded(
                        child: TextFormField(
                          controller: _privateKeyPath,
                          decoration: InputDecoration(
                            labelText: l10n.serverFieldPrivateKeyPath,
                            helperText: _isEdit
                                ? l10n.serverFieldPrivateKeyPathKeepHint
                                : null,
                          ),
                        ),
                      ),
                      const SizedBox(width: 8),
                      IconButton(
                        tooltip: l10n.serverFieldPrivateKeyBrowse,
                        icon: const Icon(Icons.folder_open),
                        onPressed: _pickPrivateKey,
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  TextFormField(
                    controller: _secret,
                    obscureText: true,
                    decoration: InputDecoration(
                      labelText: l10n.serverFieldPassphrase,
                      helperText: _isEdit
                          ? l10n.serverFieldPassphraseKeepHint
                          : null,
                    ),
                  ),
                ] else
                  TextFormField(
                    controller: _secret,
                    obscureText: true,
                    decoration: InputDecoration(
                      labelText: l10n.serverFieldPassword,
                      helperText: _isEdit
                          ? l10n.serverFieldPasswordKeepHint
                          : null,
                    ),
                  ),
                if (_errorMessage != null) ...[
                  const SizedBox(height: 12),
                  Text(
                    _errorMessage!,
                    style: TextStyle(
                      color: Theme.of(context).colorScheme.error,
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _saving ? null : () => Navigator.of(context).pop(),
          child: Text(l10n.cancel),
        ),
        FilledButton(
          onPressed: _saving ? null : _save,
          child: _saving
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : Text(l10n.save),
        ),
      ],
    );
  }
}
