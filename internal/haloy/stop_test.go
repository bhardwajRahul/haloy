package haloy

import "testing"

func TestStopInfoMessageSuggestsStatusCommand(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
		all     bool
		want    string
	}{
		{
			name: "default target",
			want: "Stop operation started. Use 'haloy status' to check whether containers are still running.",
		},
		{
			name:    "specific target",
			targets: []string{"postgres"},
			want:    "Stop operation started. Use 'haloy status -t postgres' to check whether containers are still running.",
		},
		{
			name:    "multiple targets",
			targets: []string{"postgres", "redis"},
			want:    "Stop operation started. Use 'haloy status -t postgres,redis' to check whether containers are still running.",
		},
		{
			name: "all targets",
			all:  true,
			want: "Stop operation started. Use 'haloy status --all' to check whether containers are still running.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stopInfoMessage(tt.targets, tt.all)
			if got != tt.want {
				t.Fatalf("stopInfoMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
