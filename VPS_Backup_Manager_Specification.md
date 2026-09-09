# VPS Backup Manager

## 1. Назначение документа

Этот документ описывает требования и архитектуру Windows-приложения для автоматизированного резервного копирования данных с удалённых VPS по SSH.

Приложение должно позволять пользователю:

- подключать один или несколько VPS по SSH;
- создавать задания резервного копирования;
- задавать расписание выполнения;
- выполнять произвольные команды или скрипты до резервного копирования;
- архивировать одну или несколько директорий на удалённом сервере;
- скачивать архивы на Windows-компьютер;
- проверять целостность скачанного архива;
- выполнять произвольные команды или скрипты после резервного копирования;
- автоматически удалять старые резервные копии;
- видеть историю, журнал выполнения и причины ошибок;
- запускать задания вручную;
- выполнять задания автоматически после старта Windows без необходимости держать GUI открытым.

---

# 2. Основная идея

Приложение является локальным оркестратором резервного копирования VPS.

Типовой сценарий:

```text
Windows
  │
  │ Scheduler
  ▼
Backup Service
  │
  │ SSH
  ▼
VPS
  │
  ├── выполнить pre-backup script
  │
  ├── остановить Docker Compose
  │
  ├── создать tar.gz
  │
  ├── вычислить SHA-256
  │
  ▼
Windows
  │
  ├── скачать архив по SFTP
  ├── проверить SHA-256
  ├── сохранить backup
  │
  ▼
VPS
  │
  ├── выполнить post-backup script
  ├── запустить Docker Compose
  └── удалить временный архив
```

Пример:

```text
1. cd /docker/beresta && docker compose down
2. tar -czf /tmp/beresta.tar.gz /docker/volumes
3. скачать /tmp/beresta.tar.gz
4. проверить SHA-256
5. cd /docker/beresta && docker compose up -d
6. rm /tmp/beresta.tar.gz
```

---

# 3. Главные принципы

## 3.1. Надёжность важнее скорости

Если backup завершился ошибкой, приложение должно максимально безопасно привести удалённый сервис в рабочее состояние.

Например, если выполнен:

```bash
docker compose down
```

а архивирование завершилось ошибкой, команда:

```bash
docker compose up -d
```

всё равно должна быть выполнена.

---

## 3.2. UI не должен отвечать за выполнение backup

GUI является только средством управления.

Фактическое выполнение заданий осуществляется отдельным Windows Service.

```text
VpsBackupService.exe
        │
        ├── Scheduler
        ├── SSH
        ├── SFTP
        ├── Backup Engine
        ├── Retention
        └── Logs

VpsBackupManager.exe
        │
        └── UI
```

Закрытие UI не должно останавливать резервное копирование.

---

## 3.3. На VPS не требуется устанавливать агент

Для работы должны быть достаточны:

- SSH;
- стандартный shell;
- `tar`;
- выбранный алгоритм компрессии;
- достаточные права пользователя.

Опционально могут использоваться:

- Docker;
- Docker Compose;
- `sha256sum`;
- `zstd`;
- `gzip`.

---

## 3.4. Пользователь управляет workflow

Приложение не должно быть жёстко привязано к Docker.

Пользователь должен иметь возможность создавать произвольный сценарий:

```text
Before backup
    ↓
Backup
    ↓
After backup
```

В дальнейшем модель должна позволять перейти к универсальному workflow engine.

---

# 4. Целевая платформа

## 4.1. Клиент

Поддерживаемая ОС MVP:

```text
Windows 10 x64
Windows 11 x64
```

---

## 4.2. Удалённые серверы

Основная целевая ОС:

```text
Ubuntu Server
Debian
```

Приложение не должно зависеть от systemd и может работать с другими Unix-подобными системами при наличии совместимого SSH shell.

---

# 5. Предлагаемый технологический стек

## UI

```text
Flutter
```

Причины:

- современный Windows UI;
- удобная кроссплатформенная разработка;
- возможность дальнейшего добавления других desktop-платформ.

---

## Backup Agent / Windows Service

```text
Go
```

Основные библиотеки:

```text
golang.org/x/crypto/ssh
github.com/pkg/sftp
github.com/robfig/cron/v3
golang.org/x/sys/windows/svc
```

---

## Локальная БД

```text
SQLite
```

---

## IPC

Предпочтительно:

```text
Named Pipes
```

Допустимая альтернатива:

```text
gRPC localhost
```

---

## Хранилище секретов

Предпочтительно:

```text
Windows DPAPI
```

Допустимо:

```text
Windows Credential Manager
```

---

# 6. Архитектура

```text
┌─────────────────────────────┐
│        Flutter UI           │
│                             │
│ Dashboard                   │
│ Servers                     │
│ Backup Jobs                 │
│ History                     │
│ Settings                    │
└──────────────┬──────────────┘
               │
               │ Named Pipes
               │
┌──────────────▼──────────────┐
│      Windows Service        │
│                             │
│ Scheduler                   │
│ Job Executor                │
│ SSH Client                  │
│ SFTP Client                 │
│ Archive Manager             │
│ Checksum Manager            │
│ Retention Manager           │
│ Notification Manager        │
│ Logging                     │
└──────────────┬──────────────┘
               │
               │ SSH / SFTP
               │
┌──────────────▼──────────────┐
│            VPS              │
│                             │
│ shell                       │
│ tar                         │
│ Docker / Compose optional   │
└─────────────────────────────┘
```

---

# 7. Основные сущности

## 7.1. Server

Удалённый VPS.

Поля:

```text
id
name
host
port
username
authentication_type
credential_reference
host_key_fingerprint
connection_timeout
command_timeout
created_at
updated_at
```

---

## 7.2. Backup Job

Задание резервного копирования.

Поля:

```text
id
name
server_id
enabled
schedule_id
local_destination
archive_format
remote_temp_directory
retention_policy_id
created_at
updated_at
```

---

## 7.3. Backup Source

Удалённая директория или файл.

Пример:

```text
/docker/volumes
/home/dev/config
/etc/nginx
```

Поля:

```text
id
job_id
remote_path
include
exclude
```

---

## 7.4. Script

Команда или shell script.

Типы:

```text
PRE_BACKUP
POST_BACKUP
ON_SUCCESS
ON_FAILURE
ALWAYS
```

Пример:

```bash
cd /docker/beresta && docker compose down
```

---

## 7.5. Schedule

Расписание запуска.

Поддерживаемые варианты:

```text
manual
daily
weekly
monthly
cron
```

---

## 7.6. Backup Run

Один запуск задания.

```text
id
job_id
trigger
started_at
finished_at
status
archive_filename
archive_size
checksum
error_code
error_message
```

---

# 8. Backup Job

Пример задания:

```text
Name:
Beresta Production

Server:
VPS-1

Schedule:
Daily 03:00

Before backup:
cd /docker/beresta && docker compose down

Directories:
/docker/volumes
/docker/beresta/docker-compose.yml

Archive:
tar.gz

Remote temp:
/tmp/vps-backup

Destination:
D:\Backups\Beresta

After backup:
cd /docker/beresta && docker compose up -d

Retention:
Keep last 10 backups
```

---

# 9. Жизненный цикл Backup Job

## 9.1. Состояния

```text
PENDING
RUNNING
SUCCESS
WARNING
FAILED
CANCELLED
SKIPPED
```

---

## 9.2. Основной workflow

```text
Acquire Job Lock
        ↓
Validate configuration
        ↓
Connect SSH
        ↓
Pre-flight checks
        ↓
Run PRE_BACKUP scripts
        ↓
Create archive
        ↓
Calculate remote checksum
        ↓
Download archive
        ↓
Calculate local checksum
        ↓
Verify checksum
        ↓
Run POST_BACKUP scripts
        ↓
Run health check
        ↓
Cleanup remote temporary files
        ↓
Apply retention policy
        ↓
Release Job Lock
```

---

# 10. Условия выполнения script

Каждый script должен иметь `run_condition`.

Варианты:

```text
ON_SUCCESS
ON_FAILURE
ALWAYS
```

Пример:

```text
docker compose down
    condition: ON_SUCCESS

docker compose up -d
    condition: ALWAYS
```

`ALWAYS` означает, что команда должна быть выполнена даже при ошибке предыдущих шагов.

---

# 11. Обработка критических post-actions

Некоторые команды являются восстановительными.

Например:

```bash
docker compose up -d
```

Такие команды должны быть помечены:

```text
critical_cleanup = true
```

При ошибке backup движок обязан попытаться выполнить их перед завершением задания.

Пример:

```text
docker compose down
        ↓
archive
        ↓
ERROR: No space left
        ↓
docker compose up -d
        ↓
FAILED
```

Итог:

```text
Backup: FAILED
Recovery action: SUCCESS
```

---

# 12. Расписание

Пользователь должен иметь возможность выбрать:

## Daily

```text
Every day at 03:00
```

## Weekly

```text
Monday
Wednesday
Friday
03:00
```

## Monthly

```text
First day of month
03:00
```

## Custom cron

Пример:

```text
0 3 * * *
```

---

# 13. Поведение при пропущенном запуске

Например:

```text
03:00 scheduled backup

Windows выключен

08:00 Windows включён
```

Для Job должна настраиваться политика:

```text
SKIP

или

RUN_AS_SOON_AS_POSSIBLE
```

По умолчанию:

```text
RUN_AS_SOON_AS_POSSIBLE
```

---

# 14. Защита от параллельного запуска

Для каждого Job разрешён только один активный Run.

```text
Job A
RUNNING
```

При попытке повторного запуска:

```text
Job already running
```

Результат:

```text
SKIPPED
```

---

# 15. Backup Lock

При начале выполнения создаётся локальный lock:

```text
job_id
run_id
started_at
process_id
```

После аварийного завершения Windows Service lock должен определяться как stale и безопасно очищаться.

---

# 16. Pre-flight checks

Перед выполнением backup приложение должно проверить:

```text
SSH connection
authentication
host key
remote directory exists
tar available
compression utility available
remote temporary directory writable
remote free disk space
local destination writable
local free disk space
```

Опционально:

```text
docker available
docker compose available
```

---

# 17. Проверка свободного места

Перед созданием архива:

```bash
du -sb /docker/volumes
df -B1 /tmp
```

Приложение должно показывать:

```text
Source size:
18 GB

Remote free:
31 GB

Local free:
412 GB
```

Если свободного места явно недостаточно, backup не запускается.

---

# 18. Архивирование

## MVP

Формат:

```text
tar.gz
```

Пример:

```bash
tar -czf /tmp/vpsbackup/job-1-20260908-030000.tar.gz \
    /docker/volumes \
    /home/dev/config
```

---

## Будущие форматы

```text
tar
tar.gz
tar.zst
none
```

---

# 19. Исключения

Пользователь должен иметь возможность исключать пути.

Пример:

```text
/docker/volumes
```

Exclude:

```text
*.log
cache/
tmp/
```

Генерируемая команда:

```bash
tar \
  --exclude='*.log' \
  --exclude='cache' \
  -czf backup.tar.gz \
  /docker/volumes
```

---

# 20. Именование backup

Формат по умолчанию:

```text
{job-name}_{yyyy-MM-dd_HH-mm-ss}.tar.gz
```

Пример:

```text
beresta_2026-09-08_03-00-00.tar.gz
```

---

# 21. Скачивание

В MVP используется:

```text
SFTP
```

Алгоритм:

```text
remote archive
    ↓
temporary local file
    ↓
checksum verification
    ↓
atomic rename
```

Пример:

```text
beresta_2026-09-08.tar.gz.part
```

После успешной проверки:

```text
beresta_2026-09-08.tar.gz
```

---

# 22. Resume download

Желательно предусмотреть архитектурно.

Если загрузка оборвалась:

```text
1.4 GB / 4.0 GB
```

повторное скачивание может продолжиться с 1.4 GB.

Для MVP допустимо повторное скачивание файла целиком.

---

# 23. Проверка целостности

На VPS:

```bash
sha256sum backup.tar.gz
```

После скачивания Windows вычисляет локальный SHA-256.

Сравниваются:

```text
remote SHA256
local SHA256
```

Если совпадают:

```text
VERIFIED
```

Если нет:

```text
CHECKSUM_MISMATCH
```

Backup считается неуспешным.

---

# 24. Удаление временного файла

Удалённый архив удаляется только после:

```text
download successful
+
checksum verified
```

Пример:

```bash
rm -f /tmp/vpsbackup/backup.tar.gz
```

---

# 25. Streaming backup

Будущая версия может поддерживать режим без временного архива на VPS.

```text
tar stdout
    ↓
SSH stream
    ↓
Windows file
```

Эквивалент:

```bash
ssh server "tar -czf - /docker/volumes" > backup.tar.gz
```

Плюсы:

- не требуется свободное место под архив на VPS;
- меньше операций с диском VPS.

Минусы:

- сложнее retry;
- сложнее resume;
- сложнее контроль промежуточного состояния.

Для MVP не используется.

---

# 26. Retention Policy

## MVP

Поддержать:

```text
Keep last N backups
```

и:

```text
Delete backups older than N days
```

Пример:

```text
Keep last:
10

Delete older than:
30 days
```

---

# 27. Будущий GFS Retention

В дальнейшем:

```text
14 daily
8 weekly
12 monthly
```

---

# 28. SSH Authentication

## MVP

Поддержать:

```text
SSH private key
SSH private key + passphrase
```

Парольный SSH допускается как дополнительная функция.

---

# 29. SSH Host Key Verification

При первом подключении:

```text
Unknown SSH server

Host:
95.140.x.x

ED25519:
SHA256:....
```

Пользователь должен явно подтвердить fingerprint.

После этого fingerprint сохраняется.

---

# 30. Изменение SSH fingerprint

Если ключ сервера изменился:

```text
SECURITY ERROR

SSH host key changed
```

Соединение запрещается.

Автоматическое принятие нового ключа не допускается.

---

# 31. Хранение секретов

Запрещается хранить:

```text
private_key_passphrase
password
```

в SQLite открытым текстом.

Использовать:

```text
Windows DPAPI
```

В БД хранится:

```text
credential_reference
```

---

# 32. Права SSH пользователя

Не рекомендуется подключаться под:

```text
root
```

Предпочтительно создать отдельного пользователя:

```text
backup
```

Ему выдаются минимально необходимые права.

Например:

```text
read /docker/volumes
execute docker compose
write /tmp/vpsbackup
```

---

# 33. Произвольные команды

Приложение сознательно позволяет выполнение shell-команд.

Поэтому UI должен показывать предупреждение:

```text
Scripts are executed on the remote server
with the permissions of the configured SSH user.
```

Команды не должны автоматически модифицироваться приложением.

---

# 34. Command timeout

Для каждой команды:

```text
timeout
```

По умолчанию:

```text
5 minutes
```

Для архивирования используется отдельный:

```text
archive_timeout
```

например:

```text
2 hours
```

---

# 35. Retry Policy

SSH connection:

```text
3 attempts
```

Интервалы:

```text
5 sec
15 sec
30 sec
```

---

Для shell script повтор по умолчанию отключён, потому что команда может быть неидемпотентной.

Пользователь может включить:

```text
Retry on failure
```

для конкретного шага.

---

# 36. Health Check

После post-backup пользователь может добавить проверку сервиса.

Типы:

```text
HTTP
TCP
SSH command
```

---

## HTTP

```text
URL:
https://example.com/health

Expected:
HTTP 200
```

---

## TCP

```text
host:
127.0.0.1

port:
8080
```

---

## SSH command

```bash
docker compose ps
```

---

# 37. Health check retries

Пример:

```text
attempts: 10
interval: 5 sec
```

---

# 38. Backup Result

Backup Run должен показывать несколько независимых результатов.

Пример:

```text
Archive:
SUCCESS

Download:
SUCCESS

Checksum:
SUCCESS

Post-script:
SUCCESS

Health Check:
SUCCESS
```

Общий статус:

```text
SUCCESS
```

---

Другой вариант:

```text
Archive:
FAILED

Post-script:
SUCCESS
```

Общий статус:

```text
FAILED
```

с пояснением:

```text
Service successfully restored after backup failure.
```

---

# 39. Уведомления Windows

После успешного backup:

```text
Backup completed

Beresta VPS
2.8 GB
8m 15s
```

После ошибки:

```text
Backup failed

Beresta VPS

Archive failed:
No space left on device

Recovery:
docker compose up -d — SUCCESS
```

---

# 40. Будущие каналы уведомлений

```text
Telegram
Email
Webhook
Slack
```

---

# 41. Dashboard

Главный экран:

```text
VPS Backup Manager

Next backups
──────────────────────────

Beresta
Today 03:00

FamilyHub
Today 04:00


Recent backups
──────────────────────────

✓ Beresta
Today 03:08
2.8 GB

✓ FamilyHub
Yesterday 04:12
5.2 GB

✕ HomeBox
Sep 6
Connection timeout
```

---

# 42. Servers

Экран:

```text
Servers
```

Карточка:

```text
Beresta VPS

95.140.x.x:22

user:
backup

authentication:
SSH key

connection:
Connected
```

Действия:

```text
Edit
Test connection
Delete
```

---

# 43. Backup Jobs

Список:

```text
Backup Jobs

Beresta
Daily 03:00
Next: Today 03:00
Enabled

FamilyHub
Daily 04:00
Next: Today 04:00
Enabled
```

---

# 44. Job Editor

Разделы:

```text
General

Server

Schedule

Before backup

Backup sources

Archive

Destination

After backup

Health check

Retention

Advanced
```

---

# 45. Script Editor

Пример:

```text
Before backup

Command:

cd /docker/beresta &&
docker compose down

Timeout:
5 minutes

Run condition:
ON_SUCCESS
```

---

# 46. Backup Sources UI

```text
Directories

/docker/volumes

/home/dev/config

[ + Add ]
```

---

# 47. Run Now

Для каждого Job:

```text
Run now
```

Перед запуском можно показывать:

```text
Start backup "Beresta" now?

Estimated source size:
18 GB
```

---

# 48. History

Таблица:

```text
Date
Job
Status
Size
Duration
Trigger
```

Пример:

```text
08 Sep 03:00 | Beresta | SUCCESS | 2.8 GB | 8m | Schedule
07 Sep 03:00 | Beresta | SUCCESS | 2.7 GB | 8m | Schedule
06 Sep 03:00 | Beresta | FAILED  | —      | 1m | Schedule
```

---

# 49. Run Details

```text
Beresta

Started:
03:00:00

Finished:
03:08:15

Status:
SUCCESS

Archive:
2.8 GB

SHA-256:
a84f...

Logs
────────────────────────

03:00:00 Connecting
03:00:01 Connected
03:00:02 Running pre-script
03:00:07 Archive started
03:05:42 Archive created
03:05:43 Download started
03:08:11 Download completed
03:08:12 SHA-256 verified
03:08:13 Running post-script
03:08:15 Completed
```

---

# 50. Logs

Логи разделяются на:

```text
Application logs
Service logs
Backup run logs
```

---

## Уровни

```text
DEBUG
INFO
WARNING
ERROR
```

---

# 51. Sensitive data in logs

Не должны записываться:

```text
password
private key
private key passphrase
credential content
```

Shell command может содержать секрет.

Поэтому в будущем желательно поддержать:

```text
secret variables
```

Например:

```text
${DB_PASSWORD}
```

---

# 52. Конфигурационные переменные

Job может иметь:

```text
variables
```

Пример:

```text
APP_DIR=/docker/beresta
BACKUP_DIR=/docker/volumes
```

Script:

```bash
cd ${APP_DIR} && docker compose down
```

---

# 53. Secret variables

Должны храниться через DPAPI.

Пример:

```text
DB_PASSWORD
API_TOKEN
```

При выводе в лог:

```text
***
```

---

# 54. SQLite Schema

Базовая схема:

```sql
servers
-------
id
name
host
port
username
authentication_type
credential_reference
host_key_fingerprint
created_at
updated_at


backup_jobs
-----------
id
name
server_id
enabled
archive_format
remote_temp_directory
local_destination
created_at
updated_at


backup_sources
--------------
id
job_id
remote_path
position


job_scripts
-----------
id
job_id
script_type
command
position
timeout_seconds
run_condition
critical_cleanup


schedules
---------
id
job_id
schedule_type
cron_expression
missed_run_policy


retention_policies
------------------
id
job_id
keep_last
max_age_days


backup_runs
-----------
id
job_id
trigger
started_at
finished_at
status
archive_name
archive_size
checksum
error_code
error_message


run_steps
---------
id
run_id
step_type
started_at
finished_at
status
output
error
```

---

# 55. Windows Service

Имя:

```text
VPS Backup Manager Service
```

Startup:

```text
Automatic
```

После запуска:

```text
load database
    ↓
load enabled jobs
    ↓
initialize scheduler
    ↓
detect interrupted runs
    ↓
wait
```

---

# 56. Поведение при перезагрузке

Если Windows перезагрузился во время backup:

```text
RUNNING
```

Run после старта Service должен быть переведён в:

```text
INTERRUPTED
```

Приложение должно попытаться определить наличие зарегистрированных critical cleanup actions.

Автоматическое удалённое recovery после неожиданной перезагрузки может быть отдельной настройкой.

---

# 57. System Tray

UI может иметь tray icon.

Меню:

```text
Open VPS Backup Manager

Run backup
    Beresta
    FamilyHub

Pause schedules

Exit UI
```

`Exit UI` не останавливает Windows Service.

---

# 58. Pause

Пользователь должен иметь возможность:

```text
Pause all scheduled backups
```

При этом:

```text
Run Now
```

остаётся доступным.

---

# 59. Test Connection

Для Server:

```text
Test connection
```

Проверяется:

```text
TCP
SSH handshake
host fingerprint
authentication
shell command
```

Результат:

```text
Connected

Ubuntu 24.04
OpenSSH 9.x
```

---

# 60. Test Job

Кнопка:

```text
Validate Job
```

Не выполняет destructive scripts.

Проверяет:

```text
SSH
source directories
tar
disk space
destination
permissions
```

---

# 61. Dry Run

Для произвольных пользовательских shell scripts полноценный dry run гарантировать невозможно.

Поэтому режим должен называться:

```text
Validate configuration
```

а не:

```text
Dry run
```

---

# 62. Удалённый временный каталог

По умолчанию:

```text
/tmp/vps-backup-manager
```

Для каждого Job:

```text
/tmp/vps-backup-manager/{job-id}/
```

---

# 63. Cleanup stale archives

Service периодически может находить архивы старше:

```text
24 hours
```

и предлагать удалить их.

Автоматическое удаление должно быть конфигурируемым.

---

# 64. Коды ошибок

Примеры:

```text
SSH_CONNECTION_FAILED
SSH_AUTH_FAILED
SSH_HOST_KEY_CHANGED

REMOTE_SOURCE_NOT_FOUND
REMOTE_PERMISSION_DENIED
REMOTE_NO_SPACE

ARCHIVE_FAILED
ARCHIVE_TIMEOUT

DOWNLOAD_FAILED
CHECKSUM_MISMATCH

POST_SCRIPT_FAILED
HEALTH_CHECK_FAILED

LOCAL_NO_SPACE
LOCAL_PERMISSION_DENIED
```

---

# 65. Exit status shell command

Для shell command сохраняются:

```text
exit_code
stdout
stderr
duration
```

`exit_code != 0` считается ошибкой шага.

---

# 66. Ограничение логов shell

Чтобы команда не создала гигабайты логов:

```text
max stdout/stderr:
10 MB per step
```

После лимита:

```text
output truncated
```

---

# 67. Безопасное удаление Job

При удалении Job приложение спрашивает:

```text
Delete job configuration only

или

Delete job and local backup history
```

По умолчанию существующие архивы не удаляются.

---

# 68. Импорт / экспорт конфигурации

Будущая функция:

```text
Export configuration
```

Секреты по умолчанию не включаются.

Формат:

```text
JSON
```

---

# 69. Backup самого приложения

SQLite и конфигурация приложения должны храниться:

```text
%ProgramData%\VPSBackupManager\
```

Пользователь может вручную скопировать:

```text
backup.db
```

Secrets, защищённые DPAPI, могут зависеть от Windows user/machine scope.

---

# 70. Нефункциональные требования

## Надёжность

Backup Job не должен оставлять сервисы пользователя остановленными из-за ошибки архивирования, если задан корректный recovery script.

---

## Производительность

Скачивание должно работать потоково и не загружать весь архив в RAM.

Memory usage желательно ограничивать:

```text
< 250 MB
```

при обычном SFTP backup.

---

## Масштаб

MVP:

```text
до 50 Servers
до 200 Jobs
до 10 параллельных Job между разными серверами
```

По умолчанию:

```text
max_parallel_jobs = 3
```

---

# 71. Ограничение параллельности

Параллельность должна ограничиваться:

```text
global limit
server limit
job lock
```

Пример:

```text
Global:
3

Per server:
1
```

Это предотвращает запуск одновременно нескольких тяжёлых backup на одном VPS.

---

# 72. Cancel

Пользователь может нажать:

```text
Cancel
```

После этого:

```text
остановить текущий cancellable step
    ↓
выполнить ALWAYS / critical cleanup
    ↓
пометить run CANCELLED
```

---

# 73. Graceful shutdown

При остановке Windows Service:

```text
stop accepting new jobs
    ↓
mark running jobs
    ↓
attempt critical cleanup
    ↓
shutdown
```

---

# 74. Обновление приложения

Будущая функция:

```text
Check for updates
```

Обновление Windows Service не должно происходить во время активного backup.

---

# 75. MVP

Версия `0.1` должна включать:

- Windows Service;
- Flutter UI;
- SQLite;
- подключение VPS по SSH;
- SSH private key authentication;
- проверку SSH fingerprint;
- создание Server;
- создание Backup Job;
- расписание Daily / Weekly / Cron;
- выполнение pre-backup scripts;
- backup нескольких директорий;
- `tar.gz`;
- SFTP download;
- SHA-256 verification;
- post-backup scripts;
- `ALWAYS` scripts;
- локальную retention policy;
- Job lock;
- retry SSH connection;
- историю запусков;
- подробные логи;
- Windows notifications;
- Run Now;
- Test Connection;
- Validate Job.

---

# 76. Не входит в MVP

Не включать в первую версию:

```text
Cloud storage
S3
Google Drive
OneDrive

incremental backup

block-level backup

deduplication

client-side encryption

streaming backup

backup restore wizard

Docker-specific UI

PostgreSQL-specific UI

Telegram notifications

distributed agents

mobile application
```

Архитектура при этом не должна мешать добавить эти функции позже.

---

# 77. Этап 2

После стабилизации MVP:

- `tar.zst`;
- streaming backup;
- resumable downloads;
- HTTP/TCP health checks;
- Telegram notifications;
- GFS retention;
- variables;
- secret variables;
- configuration export/import;
- hooks;
- расширенный workflow editor.

---

# 78. Этап 3

Возможные функции:

```text
PostgreSQL backup step
MySQL backup step
Docker Compose step
Docker volume discovery
remote restore
encrypted backups
S3 upload
NAS upload
incremental backup
```

---

# 79. Будущий Workflow Engine

Вместо только:

```text
PRE
BACKUP
POST
```

можно перейти к:

```text
Workflow
```

Пример:

```text
1 SSH Command
2 PostgreSQL Dump
3 Archive
4 Download
5 Verify
6 SSH Command
7 HTTP Health Check
8 Delete Remote File
```

---

# 80. Типы Workflow Step

Потенциально:

```text
SSH_COMMAND
ARCHIVE
DOWNLOAD
UPLOAD
DELETE
CHECKSUM
WAIT
HTTP_REQUEST
TCP_CHECK
DOCKER_COMPOSE
POSTGRES_DUMP
MYSQL_DUMP
```

---

# 81. UX-принципы

Главная задача интерфейса:

> Пользователь должен создать надёжный backup VPS, не разбираясь во внутренней архитектуре приложения.

---

Не следует заставлять пользователя понимать:

```text
IPC
DPAPI
SFTP sessions
job locks
run states
cleanup handlers
```

Пользователь работает с понятными сущностями:

```text
Server
Backup Job
Schedule
Before backup
Directories
After backup
History
```

---

# 82. Предупреждения UI

Для потенциально опасных команд:

```text
rm
docker compose down
shutdown
reboot
```

приложение может показывать ненавязчивое предупреждение.

Но оно не должно запрещать выполнение произвольных команд.

---

# 83. Backup Templates

Полезная функция после MVP.

Например:

```text
Docker Compose Application
```

Автоматически создаёт:

```text
Before:
docker compose down

Backup:
volume paths

After:
docker compose up -d
```

---

Другие шаблоны:

```text
Generic directory

Docker Compose

PostgreSQL

Docker Compose + PostgreSQL
```

---

# 84. Restore

Restore не входит в MVP.

Однако для каждой резервной копии приложение уже должно хранить:

```text
job_id
server
source paths
created_at
archive_format
checksum
```

Это позволит позже реализовать Restore Wizard.

---

# 85. Критерии готовности MVP

MVP считается готовым, когда успешно выполняется следующий end-to-end сценарий.

Пользователь:

1. устанавливает приложение;
2. добавляет VPS;
3. подтверждает SSH fingerprint;
4. выбирает SSH private key;
5. создаёт Job;
6. указывает:

```text
/docker/volumes
```

7. добавляет pre-script:

```bash
cd /docker/app && docker compose down
```

8. добавляет post-script:

```bash
cd /docker/app && docker compose up -d
```

с условием:

```text
ALWAYS
```

9. выбирает:

```text
Daily 03:00
```

10. выбирает Windows-каталог:

```text
D:\Backups\App
```

11. закрывает UI.

Windows Service в 03:00:

1. подключается к VPS;
2. выполняет `docker compose down`;
3. создаёт `tar.gz`;
4. вычисляет SHA-256;
5. скачивает файл;
6. вычисляет локальный SHA-256;
7. проверяет совпадение;
8. выполняет `docker compose up -d`;
9. удаляет временный архив;
10. применяет retention;
11. сохраняет историю;
12. показывает Windows notification.

После перезагрузки Windows следующий backup также должен выполняться без запуска UI.

---

# 86. Критический failure scenario

Должен быть обязательно покрыт интеграционным тестом.

Сценарий:

```text
docker compose down
        ↓
archive
        ↓
No space left on device
```

Ожидаемое поведение:

```text
archive = FAILED

docker compose up -d = EXECUTED

backup run = FAILED

recovery status = SUCCESS

notification = ERROR
```

Сервис не должен оставаться остановленным из-за ошибки backup.

---

# 87. Итоговая архитектурная концепция

Приложение представляет собой:

```text
Windows-native VPS backup orchestrator
```

без установки агентов на VPS.

Ключевые характеристики:

- SSH-based;
- agentless;
- schedule-driven;
- scriptable;
- безопасное хранение credentials;
- проверка SSH host key;
- архивирование удалённых директорий;
- SFTP download;
- checksum verification;
- гарантированные cleanup actions;
- retention;
- история и логи;
- Windows Service;
- независимый GUI;
- возможность дальнейшего развития в полноценный workflow-based backup manager.

Главная ценность продукта:

> Не просто скачать файлы с VPS, а надёжно выполнить весь operational workflow вокруг backup: подготовить сервис, сохранить данные, проверить результат и вернуть систему в рабочее состояние.
