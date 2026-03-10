package helpers

import "fmt"

func FormatBinaryBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	suffixes := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	value := float64(bytes) / float64(div)
	return fmt.Sprintf("%.1f %s", value, suffixes[exp])
}
