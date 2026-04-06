package tui

import "github.com/BestDevSpace/linkstatus/pkg/service"

func formatServiceBar() string {
	installed, running, _, err := service.Describe()
	if err != nil {
		return "Monitor svc: unknown (see /service-status)"
	}
	if !installed {
		return "Monitor svc: not installed — /service-install"
	}
	if running {
		return "Monitor svc: running"
	}
	return "Monitor svc: installed (idle) — /service-status"
}
