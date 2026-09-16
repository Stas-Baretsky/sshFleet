package history

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"ssh/internal/inventory"
	sshclient "ssh/internal/ssh"
)

func TestRunLifecycleSuccess(t *testing.T) {

	dir := t.TempDir()

	devices := []inventory.Device{
		{Name: "sw1", Address: "10.0.0.1"},
		{Name: "sw2", Address: "10.0.0.2"},
	}

	run, err := NewRun(dir, "test run", devices, []string{"show version"})

	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}

	run.MarkAllRunning()

	for _, d := range devices {

		snap := run.Snapshot()

		found := false

		for _, ds := range snap.Devices {
			if ds.Name == d.Name && ds.Status == StatusRunning {
				found = true
			}
		}

		if !found {
			t.Fatalf("device %s not marked running: %+v", d.Name, snap.Devices)
		}
	}

	id1, ok := run.DeviceIDForName("sw1")

	if !ok {
		t.Fatalf("DeviceIDForName sw1 not found")
	}

	summary, ok := run.RecordResult(id1, sshclient.Result{Host: "sw1", Command: "show version", Output: "ok"})

	if !ok {
		t.Fatalf("RecordResult returned ok=false")
	}

	if summary.Status != StatusSuccess {
		t.Fatalf("expected success status, got %s", summary.Status)
	}

	id2, _ := run.DeviceIDForName("sw2")

	_, _ = run.RecordResult(id2, sshclient.Result{Host: "sw2", Command: "show version", Error: errors.New("boom")})

	summary2, _ := run.MarkDone(id2)

	if summary2.Status != StatusError {
		t.Fatalf("expected error status, got %s", summary2.Status)
	}

	_, _ = run.MarkDone(id1)

	meta := run.Finish(false)

	if meta.FinishedAt == nil {
		t.Fatalf("FinishedAt not set")
	}

	for _, ds := range meta.Devices {

		if ds.Name == "sw1" && ds.Status != StatusSuccess {
			t.Fatalf("sw1 expected success, got %s", ds.Status)
		}

		if ds.Name == "sw2" && ds.Status != StatusError {
			t.Fatalf("sw2 expected error, got %s", ds.Status)
		}
	}

	// файлы должны существовать и содержать вывод
	data, err := os.ReadFile(filepath.Join(dir, meta.ID, id1+".txt"))

	if err != nil {
		t.Fatalf("read device file: %v", err)
	}

	if len(data) == 0 {
		t.Fatalf("device file is empty")
	}

	// ListRuns/LoadRun/ReadDeviceOutput должны видеть тот же запуск
	list, err := ListRuns(dir)

	if err != nil || len(list) != 1 {
		t.Fatalf("ListRuns: %v, %+v", err, list)
	}

	loaded, err := LoadRun(dir, meta.ID)

	if err != nil || loaded.ID != meta.ID {
		t.Fatalf("LoadRun: %v, %+v", err, loaded)
	}

	out, err := ReadDeviceOutput(dir, meta.ID, id1)

	if err != nil || out == "" {
		t.Fatalf("ReadDeviceOutput: %v, %q", err, out)
	}
}

func TestRunDeviceNeverRespondsIsError(t *testing.T) {

	dir := t.TempDir()

	devices := []inventory.Device{{Name: "sw1", Address: "10.0.0.1"}}

	run, err := NewRun(dir, "", devices, []string{"help"})

	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}

	run.MarkAllRunning()

	id, _ := run.DeviceIDForName("sw1")

	summary, ok := run.MarkDone(id)

	if !ok {
		t.Fatalf("MarkDone ok=false")
	}

	if summary.Status != StatusError {
		t.Fatalf("expected error status when no results ever arrived, got %s", summary.Status)
	}
}

func TestSanitizeIDDeduplication(t *testing.T) {

	dir := t.TempDir()

	devices := []inventory.Device{
		{Name: "sw 1", Address: "10.0.0.1"},
		{Name: "sw/1", Address: "10.0.0.2"},
	}

	run, err := NewRun(dir, "", devices, []string{"help"})

	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}

	snap := run.Snapshot()

	if len(snap.Devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(snap.Devices))
	}

	if snap.Devices[0].ID == snap.Devices[1].ID {
		t.Fatalf("expected unique IDs, both are %q", snap.Devices[0].ID)
	}
}

func TestValidateIDRejectsTraversal(t *testing.T) {

	if err := validateID("../etc/passwd"); err == nil {
		t.Fatalf("expected error for path traversal id")
	}

	if err := validateID(""); err == nil {
		t.Fatalf("expected error for empty id")
	}

	if err := validateID("normal-id"); err != nil {
		t.Fatalf("unexpected error for valid id: %v", err)
	}
}
