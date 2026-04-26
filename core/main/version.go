// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
package main

import (
"fmt"
"runtime"

xray_core "github.com/xtls/xray-core/core"
"github.com/xtls/xray-core/main/commands/base"
)

var cmdVersion = &base.Command{
UsageLine: "{{.Exec}} version",
Short:     "Show current version of v2raya-core",
Long:      `Version prints the build information for v2raya-core.`,
Run:       executeVersion,
}

func executeVersion(cmd *base.Command, args []string) {
printVersion()
}

// printVersion prints the version string.
// The first line MUST start with "V2RAYA_CORE " for v2rayA variant detection.
func printVersion() {
fmt.Printf("V2RAYA_CORE %s (xray-core) (%s %s/%s)\n",
xray_core.Version(),
runtime.Version(),
runtime.GOOS,
runtime.GOARCH,
)
}
