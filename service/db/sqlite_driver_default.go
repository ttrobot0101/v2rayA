//go:build !mipsle

package db

import _ "modernc.org/sqlite"

const sqliteDriverName = "sqlite"
