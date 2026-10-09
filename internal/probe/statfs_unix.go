package probe

import "syscall"

type unixStatfs = syscall.Statfs_t

func statfs(path string, st *unixStatfs) error { return syscall.Statfs(path, st) }
