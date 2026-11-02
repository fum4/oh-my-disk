package main

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPTYResizeIOAndClose(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("/bin/sh", "-c", `stty -echo; printf 'ready\n'; read answer; stty size; printf 'answer:%s\n' "$answer"; sleep 60`)
	session, err := startPTY(cmd, dir, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{terminal: session}
	defer a.CloseAgent()
	var output strings.Builder
	wait := func(want string) {
		t.Helper()
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			out := a.ReadAgent()
			b, e := base64.StdEncoding.DecodeString(out.Data)
			if e != nil {
				t.Fatal(e)
			}
			output.Write(b)
			if strings.Contains(output.String(), want) {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("missing %q in %q", want, output.String())
	}
	wait("ready")
	if err = a.ResizeAgent(101, 31); err != nil {
		t.Fatal(err)
	}
	if err = a.WriteAgent("hello\n"); err != nil {
		t.Fatal(err)
	}
	wait("31 101")
	wait("answer:hello")
	marker := filepath.Join(dir, "owned")
	if err = os.WriteFile(marker, []byte("temp"), 0600); err != nil {
		t.Fatal(err)
	}
	a.CloseAgent()
	a.CloseAgent()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		session.mu.Lock()
		done := session.waited
		session.mu.Unlock()
		if done {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	session.mu.Lock()
	done := session.waited
	session.mu.Unlock()
	if !done {
		t.Fatal("process not reaped")
	}
	if _, err = os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("session workspace survived close")
	}
	if err = a.WriteAgent("x"); err == nil {
		t.Fatal("write after close accepted")
	}
}
