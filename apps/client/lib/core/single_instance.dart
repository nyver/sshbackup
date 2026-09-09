import 'dart:ffi';

import 'package:ffi/ffi.dart';
import 'package:win32/win32.dart';

/// Named so a second launch of the same user's UI collides with the first;
/// distinct from the service's own named pipe (`core/ipc/ipc_client.dart`).
const _mutexName = r'Local\VPSBackupManagerUI-SingleInstance';

/// The main window's title, used to find and activate it (desktop-ui
/// specification, "Second launch"). Must match [AppLocalizations.appTitle]
/// in the default locale, since window_manager sets the OS window title
/// from it.
const _windowTitle = 'VPS Backup Manager';

typedef _CreateMutexWNative = Pointer<Void> Function(
  Pointer<Void> lpMutexAttributes,
  Int32 bInitialOwner,
  Pointer<Utf16> lpName,
);
typedef _CreateMutexWDart = Pointer<Void> Function(
  Pointer<Void> lpMutexAttributes,
  int bInitialOwner,
  Pointer<Utf16> lpName,
);

final _createMutexW = DynamicLibrary.open('kernel32.dll')
    .lookupFunction<_CreateMutexWNative, _CreateMutexWDart>('CreateMutexW');

/// Acquires a named OS mutex identifying this UI. Returns true if this
/// process is the only instance (the caller should proceed to `runApp`).
/// Returns false if another instance already holds the mutex — in that
/// case its window is brought to the foreground and the caller should
/// exit immediately without creating a window of its own.
///
/// The mutex handle is intentionally leaked for the process lifetime: it
/// is released automatically by Windows when the process exits, which is
/// the only point at which it should ever be released.
bool acquireSingleInstanceOrActivateExisting() {
  final namePtr = _mutexName.toNativeUtf16();
  final Pointer<Void> handle;
  try {
    handle = _createMutexW(nullptr, 0, namePtr);
  } finally {
    calloc.free(namePtr);
  }
  final alreadyRunning =
      handle != nullptr && GetLastError() == ERROR_ALREADY_EXISTS;
  if (!alreadyRunning) return true;

  final titlePtr = _windowTitle.toNativeUtf16();
  final Win32Result<HWND> found;
  try {
    found = FindWindow(null, PCWSTR(titlePtr));
  } finally {
    calloc.free(titlePtr);
  }
  if (found.value.address != 0) {
    ShowWindow(found.value, SW_RESTORE);
    SetForegroundWindow(found.value);
  }
  return false;
}
