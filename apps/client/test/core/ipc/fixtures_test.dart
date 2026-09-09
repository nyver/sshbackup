// Validates that Envelope and the DTOs in models.dart correctly parse
// (and round-trip) every fixture in /protocol/fixtures — the wire contract
// shared with the Go service (see server/internal/ipc and
// /protocol/README.md).
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:path/path.dart' as p;
import 'package:vps_backup_manager/core/ipc/envelope.dart';
import 'package:vps_backup_manager/core/ipc/models.dart';

String _readFixture(String name) {
  final path = p.join(
    Directory.current.path,
    '..',
    '..',
    'protocol',
    'fixtures',
    name,
  );
  return File(path).readAsStringSync();
}

void main() {
  group('request fixtures', () {
    test('request_servers_list.json', () {
      final env = Envelope.fromJsonString(
        _readFixture('request_servers_list.json'),
      );
      expect(env.isRequest, isTrue);
      expect(env.id, 'req-1');
      expect(env.protocolVersion, kProtocolVersion);
      expect(env.command, 'servers.list');
      expect(env.payload, isNull);
    });

    test('request_runs_start.json', () {
      final env = Envelope.fromJsonString(
        _readFixture('request_runs_start.json'),
      );
      expect(env.isRequest, isTrue);
      expect(env.command, 'runs.start');
      final payload = env.payload! as Map<String, dynamic>;
      expect(payload['job_id'], 'b1c2d3e4f5061728394a5b6c7d8e9f01');
    });
  });

  group('response fixtures', () {
    test('response_servers_list_success.json parses into ServerDto list', () {
      final env = Envelope.fromJsonString(
        _readFixture('response_servers_list_success.json'),
      );
      expect(env.isResponse, isTrue);
      expect(env.success, isTrue);
      final payload = env.payload! as Map<String, dynamic>;
      final servers = (payload['servers'] as List)
          .map((e) => ServerDto.fromJson(e as Map<String, dynamic>))
          .toList();
      expect(servers, hasLength(1));
      final server = servers.single;
      expect(server.id, '5f9e1c2b7a3d4e6f8091a2b3c4d5e6f7');
      expect(server.name, 'prod');
      expect(server.host, '203.0.113.5');
      expect(server.port, 22);
      expect(server.authType, 'PRIVATE_KEY');
      expect(server.hasCredential, isTrue);
      expect(server.hasTrustedHostKey, isTrue);
    });

    test('response_runs_start_success.json', () {
      final env = Envelope.fromJsonString(
        _readFixture('response_runs_start_success.json'),
      );
      expect(env.success, isTrue);
      final payload = env.payload! as Map<String, dynamic>;
      expect(payload['run_id'], 'c2d3e4f5061728394a5b6c7d8e9f0112');
      expect(payload['status'], 'RUNNING');
    });

    test('response_error_not_found.json', () {
      final env = Envelope.fromJsonString(
        _readFixture('response_error_not_found.json'),
      );
      expect(env.isResponse, isTrue);
      expect(env.success, isFalse);
      expect(env.error, isNotNull);
      expect(env.error!.code, 'NOT_FOUND');
      expect(env.error!.message, 'job "missing-id" not found');
    });

    test('response_error_version_mismatch.json', () {
      final env = Envelope.fromJsonString(
        _readFixture('response_error_version_mismatch.json'),
      );
      expect(env.success, isFalse);
      expect(env.error!.code, 'PROTOCOL_VERSION_MISMATCH');
    });
  });

  group('event fixtures', () {
    test('event_run_started.json parses into RunDto', () {
      final env = Envelope.fromJsonString(
        _readFixture('event_run_started.json'),
      );
      expect(env.isEvent, isTrue);
      expect(env.event, 'run.started');
      final payload = env.payload! as Map<String, dynamic>;
      final run = RunDto.fromJson(payload['run'] as Map<String, dynamic>);
      expect(run.id, 'c2d3e4f5061728394a5b6c7d8e9f0112');
      expect(run.jobId, 'b1c2d3e4f5061728394a5b6c7d8e9f01');
      expect(run.sourcePaths, ['/docker/volumes']);
      expect(run.trigger, 'MANUAL');
      expect(run.status, 'RUNNING');
      expect(run.isTerminal, isFalse);
      expect(run.recoveryOutcome, 'NOT_APPLICABLE');
    });

    test('event_run_step_changed.json parses into StepDto', () {
      final env = Envelope.fromJsonString(
        _readFixture('event_run_step_changed.json'),
      );
      expect(env.event, 'run.stepChanged');
      final payload = env.payload! as Map<String, dynamic>;
      expect(payload['run_id'], 'c2d3e4f5061728394a5b6c7d8e9f0112');
      final step = StepDto.fromJson(payload['step'] as Map<String, dynamic>);
      expect(step.id, 'd3e4f5061728394a5b6c7d8e9f011223');
      expect(step.type, 'ARCHIVE');
      expect(step.status, 'SUCCESS');
      expect(step.truncated, isFalse);
      expect(step.durationMs, 247000);
    });

    test('event_run_finished.json parses into a terminal RunDto', () {
      final env = Envelope.fromJsonString(
        _readFixture('event_run_finished.json'),
      );
      expect(env.event, 'run.finished');
      final payload = env.payload! as Map<String, dynamic>;
      final run = RunDto.fromJson(payload['run'] as Map<String, dynamic>);
      expect(run.status, 'SUCCESS');
      expect(run.isTerminal, isTrue);
      expect(run.archiveName, 'beresta_2026-09-08_03-00-00.tar.gz');
      expect(run.archiveSize, 445123456);
      expect(
        run.checksum,
        '9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08',
      );
    });
  });

  group('Envelope.request round-trip', () {
    test('produces the same shape as request_runs_start.json', () {
      final env = Envelope.request('req-4', 'runs.start', {
        'job_id': 'b1c2d3e4f5061728394a5b6c7d8e9f01',
      });
      final decoded = jsonDecode(env.toJsonString()) as Map<String, dynamic>;
      final expected = jsonDecode(
        _readFixture('request_runs_start.json'),
      ) as Map<String, dynamic>;
      expect(decoded, expected);
    });
  });
}
