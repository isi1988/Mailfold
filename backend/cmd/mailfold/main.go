// Command mailfold is the open-source entry point for the Mailfold backend HTTP
// server. All process orchestration lives in package app, which this thin main
// simply invokes; the enterprise entry point (cmd/mailfold-ee, in the private
// enterprise module) shares the same app.Run after registering its PostgreSQL
// driver, so there is one bootstrap and no duplicated startup logic.
package main

import (
	"log/slog"
	"os"

	"github.com/isi1988/Mailfold/backend/app"
)

func main() {
	if err := app.Run(); err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("mailfold exited with error", "error", err)
		os.Exit(1)
	}
}
