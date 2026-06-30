package main

import (
	"flag"
	"log/slog"

	"github.com/rendau/kusec/internal/app"
)

func main() {
	resetTotp := flag.String("reset-totp", "", "break-glass: отключить 2FA пользователю по username и выйти")
	flag.Parse()

	a := &app.App{}

	a.Init()

	// break-glass режим: сбросить 2FA и выйти, не поднимая серверы.
	if *resetTotp != "" {
		if err := a.ResetUserTotp(*resetTotp); err != nil {
			slog.Error("reset-totp failed", slog.String("error", err.Error()))
		} else {
			slog.Info("2FA disabled for user", slog.String("username", *resetTotp))
		}
		a.Exit()
		return
	}

	a.PreStartHook()
	a.Start()
	a.Listen()
	a.Stop()
	a.WaitJobs()
	a.Exit()
}
