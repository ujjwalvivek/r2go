//go:build !dev

package ui

// runDevScript is only built with the dev tag; see devhooks_dev.go.
func (mw *MainWindow) runDevScript() {}
