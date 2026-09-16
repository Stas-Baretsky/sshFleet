package runner

import "testing"

func TestResolveDevicesDefaultsNameToAddressAndDedupes(t *testing.T) {

	inputs := []DeviceInput{
		{Address: "10.0.0.1"}, // без имени -> имя = адрес
		{Name: "core", Address: "10.0.0.2"},
		{Name: "core", Address: "10.0.0.3"}, // дубль имени -> должен получить суффикс
		{Address: "  "},                     // пустой адрес -> пропускается
	}

	devices, err := resolveDevices(inputs, 22)

	if err != nil {
		t.Fatalf("resolveDevices: %v", err)
	}

	if len(devices) != 3 {
		t.Fatalf("expected 3 devices, got %d: %+v", len(devices), devices)
	}

	if devices[0].Name != "10.0.0.1" {
		t.Fatalf("expected fallback name = address, got %q", devices[0].Name)
	}

	if devices[1].Name != "core" {
		t.Fatalf("expected first 'core' to keep name, got %q", devices[1].Name)
	}

	if devices[2].Name != "core (2)" {
		t.Fatalf("expected deduped name 'core (2)', got %q", devices[2].Name)
	}

	for _, d := range devices {
		if d.Port != 22 {
			t.Fatalf("expected default port 22, got %d for %s", d.Port, d.Name)
		}
	}
}

func TestResolveDevicesEmptyIsError(t *testing.T) {

	if _, err := resolveDevices(nil, 22); err == nil {
		t.Fatalf("expected error for empty input")
	}

	if _, err := resolveDevices([]DeviceInput{{Address: " "}}, 22); err == nil {
		t.Fatalf("expected error when all addresses are blank")
	}
}

func TestCleanCommandsDropsEmptyLines(t *testing.T) {

	out := cleanCommands([]string{"show version", "", "   ", "configure terminal\r"})

	if len(out) != 2 {
		t.Fatalf("expected 2 commands, got %d: %+v", len(out), out)
	}

	if out[1] != "configure terminal" {
		t.Fatalf("expected trimmed CR, got %q", out[1])
	}
}
