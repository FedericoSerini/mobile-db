package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	SourcePaths []string
	Destination string
	Interval    time.Duration
}

type Runner struct {
	cfg  Config
	stop chan struct{}
}

func NewRunner(cfg Config) *Runner {
	return &Runner{cfg: cfg, stop: make(chan struct{}, 1)}
}

func (r *Runner) Start() {
	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = r.RunOnce()
		case <-r.stop:
			return
		}
	}
}

func (r *Runner) Stop() {
	select {
	case r.stop <- struct{}{}:
	default:
	}
}

func (r *Runner) RunOnce() error {
	ts := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	for _, src := range r.cfg.SourcePaths {
		if err := r.backupFile(src, ts); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) backupFile(src, ts string) error {
	if err := os.MkdirAll(r.cfg.Destination, 0700); err != nil {
		return fmt.Errorf("mkdir backup dest: %w", err)
	}
	dst := filepath.Join(r.cfg.Destination, fmt.Sprintf("%s_%s.db", filepath.Base(src), ts))
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
