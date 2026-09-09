import 'package:flutter/material.dart';

import '../../../l10n/gen/app_localizations.dart';

/// First-use host key confirmation: shows host, algorithm, and SHA-256
/// fingerprint, and requires an explicit confirmation action before the
/// service trusts the key (server-management specification, "Confirming a
/// fingerprint").
Future<bool> showHostKeyConfirmDialog({
  required BuildContext context,
  required String host,
  required String algorithm,
  required String fingerprint,
}) async {
  final l10n = AppLocalizations.of(context)!;
  final result = await showDialog<bool>(
    context: context,
    builder: (context) => AlertDialog(
      title: Text(l10n.hostKeyConfirmTitle),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 420),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(l10n.hostKeyConfirmBody(host)),
            const SizedBox(height: 16),
            _FingerprintRow(label: l10n.hostKeyAlgorithm, value: algorithm),
            const SizedBox(height: 4),
            _FingerprintRow(label: l10n.hostKeyFingerprint, value: fingerprint),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(l10n.cancel),
        ),
        FilledButton(
          onPressed: () => Navigator.of(context).pop(true),
          child: Text(l10n.hostKeyTrust),
        ),
      ],
    ),
  );
  return result ?? false;
}

/// Changed-host-key security warning: shows both fingerprints and
/// deliberately does not offer a one-click accept — trusting the new key
/// requires checking an explicit "I have verified this out of band" box
/// first (server-management specification, "Changed host key").
Future<bool> showHostKeyChangedDialog({
  required BuildContext context,
  required String host,
  required String storedFingerprint,
  required String presentedAlgorithm,
  required String presentedFingerprint,
}) async {
  final result = await showDialog<bool>(
    context: context,
    barrierDismissible: false,
    builder: (context) => _HostKeyChangedDialogContent(
      host: host,
      storedFingerprint: storedFingerprint,
      presentedAlgorithm: presentedAlgorithm,
      presentedFingerprint: presentedFingerprint,
    ),
  );
  return result ?? false;
}

class _HostKeyChangedDialogContent extends StatefulWidget {
  const _HostKeyChangedDialogContent({
    required this.host,
    required this.storedFingerprint,
    required this.presentedAlgorithm,
    required this.presentedFingerprint,
  });

  final String host;
  final String storedFingerprint;
  final String presentedAlgorithm;
  final String presentedFingerprint;

  @override
  State<_HostKeyChangedDialogContent> createState() =>
      _HostKeyChangedDialogContentState();
}

class _HostKeyChangedDialogContentState
    extends State<_HostKeyChangedDialogContent> {
  bool _verified = false;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;
    return AlertDialog(
      icon: Icon(Icons.warning_amber_rounded, color: scheme.error, size: 32),
      title: Text(l10n.hostKeyChangedTitle),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 460),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(l10n.hostKeyChangedBody(widget.host)),
            const SizedBox(height: 16),
            _FingerprintRow(
              label: l10n.hostKeyFingerprintStored,
              value: widget.storedFingerprint,
            ),
            const SizedBox(height: 4),
            _FingerprintRow(
              label: l10n.hostKeyFingerprintPresented,
              value:
                  '${widget.presentedAlgorithm} ${widget.presentedFingerprint}',
            ),
            const SizedBox(height: 16),
            CheckboxListTile(
              contentPadding: EdgeInsets.zero,
              controlAffinity: ListTileControlAffinity.leading,
              value: _verified,
              onChanged: (v) => setState(() => _verified = v ?? false),
              title: Text(l10n.hostKeyChangedVerifyCheckbox),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(l10n.cancel),
        ),
        FilledButton(
          onPressed: _verified ? () => Navigator.of(context).pop(true) : null,
          style: FilledButton.styleFrom(backgroundColor: scheme.error),
          child: Text(l10n.hostKeyTrustNewKey),
        ),
      ],
    );
  }
}

class _FingerprintRow extends StatelessWidget {
  const _FingerprintRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 110,
          child: Text(label, style: Theme.of(context).textTheme.bodySmall),
        ),
        Expanded(
          child: SelectableText(
            value,
            style: Theme.of(context).textTheme.bodyMedium
                ?.copyWith(fontFamily: 'monospace'),
          ),
        ),
      ],
    );
  }
}
