package ready

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
	"testing/iotest"
)

func TestWatcherFiresOnceAcrossChunks(t *testing.T) {
	fired := 0
	w := NewWatcher(regexp.MustCompile(`Done \(`), func() { fired++ })
	src := iotest.OneByteReader(strings.NewReader("Loading\r\n[12:00] Done (3.2s)! For help\nDone (again)\n> "))
	var dst bytes.Buffer
	if err := w.Copy(&dst, src); err != nil {
		t.Fatal(err)
	}
	if fired != 1 {
		t.Fatalf("fired %d times", fired)
	}
	if !strings.HasSuffix(dst.String(), "> ") {
		t.Fatalf("output altered: %q", dst.String())
	}
}

func TestWatcherMatchesUnterminatedLastLine(t *testing.T) {
	fired := false
	w := NewWatcher(regexp.MustCompile(`Server started`), func() { fired = true })
	_ = w.Copy(io.Discard, strings.NewReader("Server started"))
	if !fired {
		t.Fatal("not fired")
	}
}

func TestMarker(t *testing.T) {
	dir := t.TempDir()
	if Healthy(dir) {
		t.Fatal("healthy before mark")
	}
	if err := Mark(dir); err != nil {
		t.Fatal(err)
	}
	if !Healthy(dir) {
		t.Fatal("not healthy after mark")
	}
	Clear(dir)
	if Healthy(dir) {
		t.Fatal("healthy after clear")
	}
}
