package ui

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
)

func dbgLog(format string, args ...any) {
	f, err := os.OpenFile(os.Getenv("TEMP")+`\r2go-debug.log`, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}

func dbgDump(tag string, s *Sidebar) {
	if s == nil {
		dbgLog("%s: sidebar=nil", tag)
		return
	}
	w := s.mw.Window()
	cs := w.Content().Size()
	dbgLog("%s: windowContent=%v sidebar=%v tree=%v", tag, cs, s.container.Size(), s.tree.Size())
}

func dbgDumpLater(s *Sidebar, delay time.Duration, tag string) {
	time.Sleep(delay)
	fyne.Do(func() { dbgDump(tag, s) })
}

func dbgPos(name string, o fyne.CanvasObject) {
	if o == nil {
		dbgLog("  %-14s nil", name)
		return
	}
	if o.Visible() {
		dbgLog("  %-14s pos=%v size=%v min=%v", name, o.Position(), o.Size(), o.MinSize())
	} else {
		dbgLog("  %-14s HIDDEN pos=%v size=%v min=%v", name, o.Position(), o.Size(), o.MinSize())
	}
}

func dbgDumpFile(tag string, f *FileView) {
	if f == nil {
		dbgLog("%s: fileview=nil", tag)
		return
	}
	dbgLog("%s: visible=%d all=%d", tag, len(f.visible), len(f.all))
	dbgPos("container", f.Container)
	dbgPos("listView", f.listView)
	dbgPos("header.row", f.header.row)
	dbgPos("table", f.table)
	dbgPos("loadBtn", f.loadBtn)
	dbgPos("placeholder", f.placeholder)
	dbgPos("loading", f.loading)
	dbgPos("empty", f.empty)
}
