package backup_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/federicoserini/mobile-db/server/backup"
)

func TestLocalBackupCopiesFile(t *testing.T) {
	src := filepath.Join(t.TempDir(), "data.db")
	dst := t.TempDir()
	if err := os.WriteFile(src, []byte("sqlite-data"), 0600); err != nil {
		t.Fatal(err)
	}
	b := backup.NewRunner(backup.Config{
		SourcePaths: []string{src},
		Destination: dst,
		Interval:    time.Hour,
	})
	if err := b.RunOnce(); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	entries, err := os.ReadDir(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no backup file created")
	}
}

func TestBackupRunnerStops(t *testing.T) {
	b := backup.NewRunner(backup.Config{
		SourcePaths: []string{},
		Destination: t.TempDir(),
		Interval:    time.Hour,
	})
	done := make(chan struct{})
	go func() { b.Start(); close(done) }()
	b.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop must cause Start to return within 1s")
	}
}
