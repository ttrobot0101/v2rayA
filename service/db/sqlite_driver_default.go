//go:build !mips && !mipsle && !mips64 && !mips64le && !mips64p32 && !mips64p32le

package db

import _ "modernc.org/sqlite"

const sqliteDriverName = "sqlite"
