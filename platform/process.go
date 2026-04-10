package platform

// This file documents the platform-specific process spawning helpers
// implemented in process_unix.go and process_windows.go. These helpers
// abstract over the differences in how Unix and Windows detach a child
// process from its parent's controlling terminal so the child can run as
// a long-lived background process.
//
// Helpers:
//
//   ApplyDaemonProcAttr(cmd) — detaches the spawned process from the
//   controlling terminal so it survives after the parent shell exits.
//   On Unix this calls Setsid; on Windows it sets CREATE_NEW_PROCESS_GROUP
//   and DETACHED_PROCESS so the child does not inherit the parent's console
//   and is not killed when the parent receives Ctrl+C.
//
// Callers should use these helpers rather than constructing
// syscall.SysProcAttr directly. Direct construction breaks cross-compilation
// because Setsid is Unix-only and CreationFlags is Windows-only.
