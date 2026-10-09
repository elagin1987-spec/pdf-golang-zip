// QA-only export of actual production arguments; no duplicated profile settings.
package main

import (
	"encoding/json"
	"os"
	"pdf-golang-zip/internal/ghostscript"
)

func main() {
	if len(os.Args) != 4 {
		panic("usage: quality-profiles profile input output")
	}
	args, err := ghostscript.BuildArgs(ghostscript.ProfileName(os.Args[1]), os.Args[2], os.Args[3])
	if err != nil {
		panic(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(args); err != nil {
		panic(err)
	}
}
