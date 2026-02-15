package hosts

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

const marker = "# microfoundry managed"

// hostsPath returns the OS-specific hosts file path.
func hostsPath() string {
	if runtime.GOOS == "windows" {
		return `C:\Windows\System32\drivers\etc\hosts`
	}
	return "/etc/hosts"
}

// Add adds a hostname entry pointing to 127.0.0.1 in the hosts file.
func Add(hostname string) error {
	path := hostsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading hosts file: %w", err)
	}

	entry := fmt.Sprintf("127.0.0.1 %s %s", hostname, marker)

	// Check if entry already exists
	if strings.Contains(string(data), hostname+" "+marker) {
		return nil
	}

	content := strings.TrimRight(string(data), "\n") + "\n" + entry + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing hosts file (may need admin/sudo): %w", err)
	}
	return nil
}

// Remove removes a hostname entry from the hosts file.
func Remove(hostname string) error {
	path := hostsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading hosts file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var filtered []string
	for _, line := range lines {
		if strings.Contains(line, hostname) && strings.Contains(line, marker) {
			continue
		}
		filtered = append(filtered, line)
	}

	if err := os.WriteFile(path, []byte(strings.Join(filtered, "\n")), 0644); err != nil {
		return fmt.Errorf("writing hosts file (may need admin/sudo): %w", err)
	}
	return nil
}

// List returns all MicroFoundry-managed hostnames.
func List() ([]string, error) {
	data, err := os.ReadFile(hostsPath())
	if err != nil {
		return nil, fmt.Errorf("reading hosts file: %w", err)
	}

	var hostnames []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, marker) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				hostnames = append(hostnames, parts[1])
			}
		}
	}
	return hostnames, nil
}
