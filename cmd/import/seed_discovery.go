package main

import (
	"fmt"
	"os"
)

// resolveSeedFile returns the trainer seed file path for a game slug if the
// file exists. The convention is db/seed/<slug>_trainers.json. This lets new
// games be added as pure data without any Go code change.
func resolveSeedFile(slug string) (string, bool) {
	path := fmt.Sprintf("db/seed/%s_trainers.json", slug)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}
