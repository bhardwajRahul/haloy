package haloy

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/haloydev/haloy/internal/cmdexec"
	"github.com/haloydev/haloy/internal/config"
	"github.com/haloydev/haloy/internal/constants"
)

func TestDefaultPruneKeep(t *testing.T) {
	localKeep := 3
	registryKeep := 2

	tests := []struct {
		name   string
		target config.TargetConfig
		want   int
	}{
		{
			name:   "no image falls back to default",
			target: config.TargetConfig{},
			want:   int(constants.DefaultDeploymentsToKeep),
		},
		{
			name: "history none defaults to zero",
			target: config.TargetConfig{
				Image: &config.Image{
					History: &config.ImageHistory{Strategy: config.HistoryStrategyNone},
				},
			},
			want: 0,
		},
		{
			name: "local history uses configured count",
			target: config.TargetConfig{
				Image: &config.Image{
					History: &config.ImageHistory{Strategy: config.HistoryStrategyLocal, Count: &localKeep},
				},
			},
			want: 3,
		},
		{
			name: "registry history uses configured count",
			target: config.TargetConfig{
				Image: &config.Image{
					History: &config.ImageHistory{Strategy: config.HistoryStrategyRegistry, Count: &registryKeep},
				},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultPruneKeep(tt.target); got != tt.want {
				t.Fatalf("defaultPruneKeep() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestWithImagePruneHint_AddsTargetAwareHintsForDiskErrors(t *testing.T) {
	keep := 3
	target := config.TargetConfig{
		Name:       "app",
		TargetName: "staging",
		Server:     "haloy.example.com",
		Image: &config.Image{
			History: &config.ImageHistory{
				Strategy: config.HistoryStrategyLocal,
				Count:    &keep,
			},
		},
	}

	err := withImagePruneHint(errors.New("server disk space too low on /var/lib/haloy/tmp"), target)
	msg := err.Error()

	if !strings.Contains(msg, "haloy prune-images --keep 3 --yes") {
		t.Fatalf("msg = %q, want prune command hint", msg)
	}
	if !strings.Contains(msg, "image.history.count is 3") {
		t.Fatalf("msg = %q, want count hint", msg)
	}
	if !strings.Contains(msg, "haloy prune-images --keep 2 --yes") {
		t.Fatalf("msg = %q, want reduced keep hint", msg)
	}
}

func TestWithImagePruneHint_LeavesNonDiskErrorsUnchanged(t *testing.T) {
	target := config.TargetConfig{Name: "app", TargetName: "staging", Server: "haloy.example.com"}
	err := withImagePruneHint(errors.New("authentication failed"), target)
	if err.Error() != "authentication failed" {
		t.Fatalf("err = %q, want unchanged error", err.Error())
	}
}

func TestWithImagePruneHint_AddsHintsForServerNoSpaceLeftErrors(t *testing.T) {
	target := config.TargetConfig{Name: "app", TargetName: "staging", Server: "haloy.example.com"}
	err := withImagePruneHint(errors.New("failed to upload image: docker load: no space left on device"), target)
	if !strings.Contains(err.Error(), "haloy prune-images") {
		t.Fatalf("err = %q, want prune command hint", err.Error())
	}
}

func TestWithLocalDockerDiskFullHint_AddsHintsForDiskErrors(t *testing.T) {
	buildErr := fmt.Errorf("failed to build image myapp:abc123: %w", &cmdexec.ExitError{
		Name:       "docker",
		ExitCode:   1,
		StderrTail: "ERROR: failed to solve: write /var/lib/docker/tmp/buildkit: no space left on device",
	})

	msg := withLocalDockerDiskFullHint(buildErr).Error()
	if !strings.Contains(msg, "docker system prune") {
		t.Fatalf("msg = %q, want docker system prune hint", msg)
	}
	if !strings.Contains(msg, "virtual disk size") {
		t.Fatalf("msg = %q, want Docker Desktop disk size hint", msg)
	}
	if !strings.Contains(msg, "failed to build image myapp:abc123") {
		t.Fatalf("msg = %q, want original error preserved", msg)
	}
}

func TestWithLocalDockerDiskFullHint_MatchesErrorMessageText(t *testing.T) {
	err := withLocalDockerDiskFullHint(errors.New("failed to save image to tar: write /tmp/haloy-upload.tar: no space left on device"))
	if !strings.Contains(err.Error(), "docker system prune") {
		t.Fatalf("err = %q, want disk full hint", err.Error())
	}
}

func TestWithLocalDockerDiskFullHint_LeavesOtherErrorsUnchanged(t *testing.T) {
	buildErr := fmt.Errorf("failed to build image myapp:abc123: %w", &cmdexec.ExitError{
		Name:       "docker",
		ExitCode:   1,
		StderrTail: "ERROR: dockerfile parse error on line 3",
	})

	err := withLocalDockerDiskFullHint(buildErr)
	if err != buildErr {
		t.Fatalf("err = %q, want unchanged error", err.Error())
	}
}
