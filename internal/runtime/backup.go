package runtime

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

var backupName = regexp.MustCompile(`^miao_backup_[a-zA-Z0-9_-]+\.zip$`)

func operationTimestamp() string {
	now := time.Now().UTC()
	return now.Format("20060102_150405") + fmt.Sprintf("_%09d", now.Nanosecond())
}

// Backup uses PocketBase's consistent snapshot, including its file storage.
// Local operations are deliberately offline and hold the same data lock as serve.
func (r *Runtime) Backup(ctx context.Context, directory string, retentionDays int) (string, error) {
	if directory == "" {
		directory = filepath.Join(r.Root, "backups")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	name := "miao_backup_" + operationTimestamp() + ".zip"
	if err := r.App.CreateBackup(ctx, name); err != nil {
		return "", err
	}
	fsys, err := r.App.NewBackupsFilesystem()
	if err != nil {
		return "", err
	}
	defer fsys.Close()
	fsys.SetContext(ctx)
	reader, err := fsys.GetReader(name)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	target := filepath.Join(directory, name)
	temp, err := os.CreateTemp(directory, ".miao-backup-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(temp.Name())
	_, copyErr := io.Copy(temp, reader)
	syncErr := temp.Sync()
	closeErr := temp.Close()
	if err := errors.Join(copyErr, syncErr, closeErr); err != nil {
		return "", err
	}
	if err := os.Rename(temp.Name(), target); err != nil {
		return "", err
	}
	if err := fsys.Delete(name); err != nil {
		return target, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return target, err
	}
	cutoff := time.Now().AddDate(0, 0, -max(1, retentionDays))
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !backupName.MatchString(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return target, err
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(directory, entry.Name())); err != nil {
				return target, err
			}
		}
	}
	return target, nil
}

func extractBackup(archive, directory string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	if len(reader.File) > 100000 {
		return errors.New("backup contains too many files")
	}
	var total uint64
	seen := map[string]bool{}
	for _, file := range reader.File {
		name := filepath.Clean(file.Name)
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) || strings.Contains(file.Name, "\\") || file.Mode()&os.ModeSymlink != 0 || seen[name] {
			return fmt.Errorf("invalid backup entry %q", file.Name)
		}
		seen[name] = true
		total += file.UncompressedSize64
		if total > 100<<30 {
			return errors.New("backup exceeds 100 GiB extraction limit")
		}
		target := filepath.Join(directory, name)
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		input, err := file.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		if err := errors.Join(copyErr, output.Close(), input.Close()); err != nil {
			return err
		}
	}
	if !seen["data.db"] {
		return errors.New("backup does not contain data.db")
	}
	return nil
}

// Restore validates and isolates the staged database before replacing pb_data.
// It preserves the pre-restore directory for recovery and never starts a worker.
func Restore(ctx context.Context, root, archive string) (string, error) {
	if !backupName.MatchString(filepath.Base(archive)) {
		return "", errors.New("select an archive named miao_backup_*.zip")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	lock, err := lockData(root)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	stage, err := os.MkdirTemp(root, ".miao-restore-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	if err := extractBackup(archive, stage); err != nil {
		return "", err
	}
	app, _, err := openApp(stage)
	if err != nil {
		return "", fmt.Errorf("backup database validation: %w", err)
	}
	var integrity string
	err = app.ConcurrentDB().NewQuery("PRAGMA integrity_check").WithContext(ctx).Row(&integrity)
	if err == nil && integrity != "ok" {
		err = errors.New("backup database integrity check failed")
	}
	if err == nil {
		err = isolateHistoricalWork(ctx, app)
	}
	err = errors.Join(err, app.ResetBootstrapState())
	if err != nil {
		return "", err
	}
	data := filepath.Join(root, "pb_data")
	previous := filepath.Join(root, "pb_data.before_restore_"+operationTimestamp())
	if _, err := os.Stat(data); err == nil {
		if err := os.Rename(data, previous); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	} else {
		previous = ""
	}
	if err := os.Rename(stage, data); err != nil {
		if previous != "" {
			err = errors.Join(err, os.Rename(previous, data))
		}
		return "", err
	}
	return previous, nil
}

func isolateHistoricalWork(ctx context.Context, app core.App) error {
	return app.RunInTransaction(func(tx core.App) error {
		for _, change := range []struct {
			collection, filter string
			data               map[string]any
		}{
			{"miao_tasks", `status = "enabled"`, map[string]any{"status": "paused", "next_run_at": "", "pause_reason": "历史备份恢复，核实恢复点后已发生的动作再重新授权"}},
			{"miao_runs", `status = "queued" || status = "running" || status = "waiting"`, map[string]any{"status": "cancelled", "cancel_requested": true, "finished_at": time.Now().UTC().Format(time.RFC3339Nano), "delivery_status": "suppressed", "error": "历史备份恢复：保留原动作证据，不自动恢复或重放", "harness_state": "cancelled", "harness_phase": "cancelled", "harness_lease_owner": "", "harness_lease_expires_at": "", "harness_active_started_at": ""}},
			{"miao_harness_runs", `state != "completed" && state != "failed" && state != "cancelled" && state != "budget_exhausted" && state != "unsupported"`, map[string]any{"state": "cancelled", "phase": "cancelled", "cancel_requested": true, "error": "历史备份恢复：保留原动作证据，不自动恢复或重放", "lease_owner": "", "lease_expires_at": "", "active_started_at": ""}},
			{"miao_run_attempts", `status = "running"`, map[string]any{"status": "interrupted", "finished_at": time.Now().UTC().Format(time.RFC3339Nano), "error": "历史备份恢复，原运行已隔离"}},
			{"automation_rules", `enabled = true`, map[string]any{"enabled": false, "pause_reason": "历史备份恢复，等待负责人重新核实启用"}},
		} {
			rows, err := tx.FindRecordsByFilter(change.collection, change.filter, "", 0, 0)
			if err != nil {
				return err
			}
			for _, row := range rows {
				row.Load(change.data)
				if err := tx.SaveWithContext(ctx, row); err != nil {
					return err
				}
			}
		}
		locks, err := tx.FindAllRecords("miao_runtime_locks")
		if err != nil {
			return err
		}
		for _, lock := range locks {
			if err := tx.DeleteWithContext(ctx, lock); err != nil {
				return err
			}
		}
		return nil
	})
}
