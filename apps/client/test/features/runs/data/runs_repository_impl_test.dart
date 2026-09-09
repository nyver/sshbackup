import 'package:flutter_test/flutter_test.dart';
import 'package:vps_backup_manager/features/runs/data/runs_repository_impl.dart';

import '../../../test_helpers/fake_ipc_client.dart';
import '../../../test_helpers/fixtures.dart';

void main() {
  late FakeIpcClient client;
  late IpcRunsRepository repository;

  setUp(() {
    client = FakeIpcClient();
    repository = IpcRunsRepository(client);
  });

  test('start sends the job id and returns the new run', () async {
    client.handlers['runs.start'] = (payload) {
      expect((payload! as Map<String, dynamic>)['job_id'], 'j1');
      return {'run_id': 'r1', 'status': 'PENDING'};
    };

    final result = await repository.start('j1');

    expect(result.runId, 'r1');
    expect(result.status, 'PENDING');
  });

  test('list omits empty filters', () async {
    client.handlers['runs.list'] = (payload) {
      final map = payload! as Map<String, dynamic>;
      expect(map.containsKey('job_id'), isFalse);
      expect(map.containsKey('status'), isFalse);
      expect(map['limit'], 20);
      return {
        'runs': [runJson()],
      };
    };

    final runs = await repository.list(limit: 20);

    expect(runs.single.id, 'r1');
  });

  test('list includes non-empty filters', () async {
    client.handlers['runs.list'] = (payload) {
      final map = payload! as Map<String, dynamic>;
      expect(map['job_id'], 'j1');
      expect(map['status'], 'FAILED');
      return {'runs': <Map<String, dynamic>>[]};
    };

    await repository.list(jobId: 'j1', status: 'FAILED');
  });

  test('get returns the run with its steps', () async {
    client.handlers['runs.get'] = (payload) {
      expect((payload! as Map<String, dynamic>)['id'], 'r1');
      return {
        'run': runJson(),
        'steps': [stepJson()],
      };
    };

    final details = await repository.get('r1');

    expect(details.run.id, 'r1');
    expect(details.steps.single.type, 'ARCHIVE');
  });

  test('cancel sends the run id', () async {
    client.handlers['runs.cancel'] = (payload) {
      expect((payload! as Map<String, dynamic>)['run_id'], 'r1');
      return <String, dynamic>{};
    };

    await repository.cancel('r1');
  });
}
