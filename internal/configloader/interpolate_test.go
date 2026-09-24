package configloader

import (
	"strings"
	"testing"

	"github.com/haloydev/haloy/internal/config"
)

func makeEnv(name, value string) config.EnvVar {
	return config.EnvVar{
		Name: name,
		ValueSource: config.ValueSource{
			Value: value,
		},
	}
}

func TestInterpolateEnvVars(t *testing.T) {
	tests := []struct {
		name string
		envs []config.EnvVar
		want []string
	}{
		{
			name: "no references",
			envs: []config.EnvVar{makeEnv("FOO", "bar"), makeEnv("BAZ", "qux")},
			want: []string{"bar", "qux"},
		},
		{
			name: "simple reference",
			envs: []config.EnvVar{
				makeEnv("PASSWORD", "secret123"),
				makeEnv("DATABASE_URL", "postgresql://user:${PASSWORD}@host:5432/db"),
			},
			want: []string{"secret123", "postgresql://user:secret123@host:5432/db"},
		},
		{
			name: "reference declared before its source",
			envs: []config.EnvVar{
				makeEnv("URL", "postgresql://user:${PASSWORD}@host/db"),
				makeEnv("PASSWORD", "secret"),
			},
			want: []string{"postgresql://user:secret@host/db", "secret"},
		},
		{
			name: "chained references",
			envs: []config.EnvVar{
				makeEnv("C", "${B}-end"),
				makeEnv("B", "${A}-middle"),
				makeEnv("A", "start"),
			},
			want: []string{"start-middle-end", "start-middle", "start"},
		},
		{
			name: "multiple references in one value",
			envs: []config.EnvVar{
				makeEnv("SCHEME", "https"),
				makeEnv("HOST", "example.com"),
				makeEnv("PORT", "8443"),
				makeEnv("URL", "${SCHEME}://${HOST}:${PORT}"),
			},
			want: []string{"https", "example.com", "8443", "https://example.com:8443"},
		},
		{
			name: "repeated and adjacent references",
			envs: []config.EnvVar{
				makeEnv("A", "hello"),
				makeEnv("B", "world"),
				makeEnv("C", "${A}${B}/${A}"),
			},
			want: []string{"hello", "world", "helloworld/hello"},
		},
		{
			name: "entire value is a reference",
			envs: []config.EnvVar{makeEnv("SOURCE", "the-value"), makeEnv("COPY", "${SOURCE}")},
			want: []string{"the-value", "the-value"},
		},
		{
			name: "undefined reference left as literal",
			envs: []config.EnvVar{
				makeEnv("DB_PASS", "secret"),
				makeEnv("CONNECTION", "host=${DB_PASS}&opts=${NOT_DEFINED}"),
			},
			want: []string{"secret", "host=secret&opts=${NOT_DEFINED}"},
		},
		{
			name: "reference to empty value",
			envs: []config.EnvVar{makeEnv("EMPTY", ""), makeEnv("URL", "prefix-${EMPTY}-suffix")},
			want: []string{"", "prefix--suffix"},
		},
		{
			name: "duplicate names use the last definition",
			envs: []config.EnvVar{
				makeEnv("PASSWORD", "first"),
				makeEnv("PASSWORD", "second"),
				makeEnv("URL", "user:${PASSWORD}@host"),
			},
			want: []string{"first", "second", "user:second@host"},
		},
		{
			name: "non-matching dollar patterns left alone",
			envs: []config.EnvVar{
				makeEnv("BARE", "$VAR stays"),
				makeEnv("EMPTY_BRACES", "${} stays"),
				makeEnv("NUMERIC_START", "${123BAD} stays"),
				makeEnv("HYPHEN", "${A-B} stays"),
				makeEnv("DOLLAR_ONLY", "just $ here"),
			},
			want: []string{"$VAR stays", "${} stays", "${123BAD} stays", "${A-B} stays", "just $ here"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := InterpolateEnvVars(tt.envs); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for i, want := range tt.want {
				if got := tt.envs[i].Value; got != want {
					t.Errorf("%s = %q, want %q", tt.envs[i].Name, got, want)
				}
			}
		})
	}
}

func TestInterpolateEnvVars_EmptyList(t *testing.T) {
	if err := InterpolateEnvVars(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := InterpolateEnvVars([]config.EnvVar{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInterpolateEnvVars_ThreeNodeCycle(t *testing.T) {
	envVars := []config.EnvVar{
		makeEnv("A", "${B}"),
		makeEnv("B", "${C}"),
		makeEnv("C", "${A}"),
	}
	err := InterpolateEnvVars(envVars)
	if err == nil {
		t.Fatal("expected error for circular dependency, got nil")
	}
}

func TestInterpolateEnvVars_CycleErrorContainsVarNames(t *testing.T) {
	envVars := []config.EnvVar{
		makeEnv("ALPHA", "${BETA}"),
		makeEnv("BETA", "${ALPHA}"),
	}
	err := InterpolateEnvVars(envVars)
	if err == nil {
		t.Fatal("expected error for circular dependency, got nil")
	}
	if !strings.Contains(err.Error(), "ALPHA") || !strings.Contains(err.Error(), "BETA") {
		t.Errorf("error should mention involved var names, got: %s", err.Error())
	}
}

func TestInterpolateEnvVars_SelfReferenceErrorContainsVarName(t *testing.T) {
	envVars := []config.EnvVar{
		makeEnv("MYSELF", "${MYSELF}-more"),
	}
	err := InterpolateEnvVars(envVars)
	if err == nil {
		t.Fatal("expected error for self-reference, got nil")
	}
	if !strings.Contains(err.Error(), "MYSELF") {
		t.Errorf("error should mention the var name, got: %s", err.Error())
	}
}
