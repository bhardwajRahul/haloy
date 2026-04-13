package haloy

import (
	"context"
	"reflect"
	"testing"
)

func TestSaveImageTar_UsesDirectArgsForWindowsStyleTempPath(t *testing.T) {
	previous := runCLICommandInDir
	t.Cleanup(func() {
		runCLICommandInDir = previous
	})

	var (
		gotWorkDir string
		gotName    string
		gotArgs    []string
	)

	runCLICommandInDir = func(_ context.Context, workDir, name string, args ...string) error {
		gotWorkDir = workDir
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil
	}

	tarPath := `C:\Users\Familia\AppData\Local\Temp\haloy-upload-normaagro-dev-latest-123.tar`
	imageRef := "normaagro-dev:latest"

	if err := saveImageTar(context.Background(), imageRef, tarPath); err != nil {
		t.Fatalf("saveImageTar() unexpected error: %v", err)
	}

	if gotWorkDir != "." {
		t.Fatalf("workDir = %q, want %q", gotWorkDir, ".")
	}
	if gotName != "docker" {
		t.Fatalf("name = %q, want %q", gotName, "docker")
	}

	wantArgs := []string{"save", "-o", tarPath, imageRef}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args = %#v, want %#v", gotArgs, wantArgs)
	}
}
