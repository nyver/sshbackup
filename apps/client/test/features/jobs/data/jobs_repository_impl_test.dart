import 'package:flutter_test/flutter_test.dart';
import 'package:vps_backup_manager/core/ipc/models.dart';
import 'package:vps_backup_manager/features/jobs/data/jobs_repository_impl.dart';

import '../../../test_helpers/fake_ipc_client.dart';
import '../../../test_helpers/fixtures.dart';

void main() {
  late FakeIpcClient client;
  late IpcJobsRepository repository;

  setUp(() {
    client = FakeIpcClient();
    repository = IpcJobsRepository(client);
  });

  test('list sends jobs.list and parses jobs', () async {
    client.handlers['jobs.list'] = (_) => {
      'jobs': [jobJson()],
    };

    final jobs = await repository.list();

    expect(client.sentRequests.single.$1, 'jobs.list');
    expect(jobs.single.name, 'nightly');
  });

  test('create wraps the job under a "job" key', () async {
    client.handlers['jobs.create'] = (payload) {
      final map = payload! as Map<String, dynamic>;
      expect(map['job'], isA<Map<String, dynamic>>());
      expect((map['job'] as Map<String, dynamic>)['name'], 'nightly');
      return {'job': jobJson(id: 'new-id')};
    };

    final job = JobDto.fromJson(jobJson());
    final result = await repository.create(job);

    expect(result.id, 'new-id');
  });

  test('setEnabled picks jobs.enable or jobs.disable', () async {
    client.handlers['jobs.enable'] = (payload) {
      expect((payload! as Map<String, dynamic>)['id'], 'j1');
      return <String, dynamic>{};
    };
    await repository.setEnabled('j1', enabled: true);
    expect(client.sentRequests.single.$1, 'jobs.enable');

    client.sentRequests.clear();
    client.handlers['jobs.disable'] = (_) => <String, dynamic>{};
    await repository.setEnabled('j1', enabled: false);
    expect(client.sentRequests.single.$1, 'jobs.disable');
  });

  test('validate parses per-check results', () async {
    client.handlers['jobs.validate'] = (payload) {
      expect((payload! as Map<String, dynamic>)['id'], 'j1');
      return {
        'success': false,
        'checks': [
          {'name': 'tar available', 'passed': true, 'message': ''},
          {
            'name': 'free space',
            'passed': false,
            'message': 'not enough space',
          },
        ],
        'source_size_known': false,
        'remote_free_known': false,
        'local_free_known': false,
      };
    };

    final result = await repository.validate('j1');

    expect(result.success, isFalse);
    expect(result.checks, hasLength(2));
    expect(result.checks[1].passed, isFalse);
  });
}
