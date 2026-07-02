package proxy

import (
	"strings"
	"testing"
)

func TestRouteBuilder_AddRoute(t *testing.T) {
	tests := []struct {
		name      string
		canonical string
		aliases   []string
		backends  []Backend
	}{
		{
			name:      "simple route with one backend",
			canonical: "example.com",
			aliases:   nil,
			backends:  []Backend{{IP: "10.0.0.1", Port: "8080"}},
		},
		{
			name:      "route with multiple backends",
			canonical: "example.com",
			aliases:   nil,
			backends:  []Backend{{IP: "10.0.0.1", Port: "8080"}, {IP: "10.0.0.2", Port: "8080"}},
		},
		{
			name:      "route with aliases",
			canonical: "example.com",
			aliases:   []string{"www.example.com", "app.example.com"},
			backends:  []Backend{{IP: "10.0.0.1", Port: "8080"}},
		},
		{
			name:      "case normalization",
			canonical: "EXAMPLE.COM",
			aliases:   []string{"WWW.EXAMPLE.COM"},
			backends:  []Backend{{IP: "10.0.0.1", Port: "8080"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := NewRouteBuilder()
			rb.AddRoute(tt.canonical, tt.aliases, tt.backends)

			config, err := rb.Build()
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			// Check route exists with lowercased canonical
			loweredCanonical := "example.com"
			route := config.FindRoute(loweredCanonical)
			if route == nil {
				t.Fatalf("route not found for canonical %q", loweredCanonical)
			}

			if route.Canonical != loweredCanonical {
				t.Errorf("Canonical = %q, want %q", route.Canonical, loweredCanonical)
			}

			if len(route.Backends) != len(tt.backends) {
				t.Errorf("len(Backends) = %d, want %d", len(route.Backends), len(tt.backends))
			}

			// Check aliases are lowercased and resolve to the same route
			for _, alias := range route.Aliases {
				if alias != strings.ToLower(alias) {
					t.Errorf("alias %q contains uppercase characters", alias)
				}
				if config.FindRoute(alias) != route {
					t.Errorf("FindRoute(%q) does not resolve to the canonical route", alias)
				}
			}
		})
	}
}

func TestRouteBuilder_Build_DuplicateAlias(t *testing.T) {
	rb := NewRouteBuilder()
	rb.AddRoute("app1.example.com", []string{"shared.example.com"}, []Backend{{IP: "10.0.0.1", Port: "8080"}})
	rb.AddRoute("app2.example.com", []string{"shared.example.com"}, []Backend{{IP: "10.0.0.2", Port: "8080"}})

	_, err := rb.Build()
	if err == nil {
		t.Fatal("Build() expected error for duplicate alias, got nil")
	}
	if !strings.Contains(err.Error(), "alias") {
		t.Fatalf("Build() error = %v, expected duplicate alias context", err)
	}
}

func TestRouteBuilder_Build_CanonicalAliasConflict(t *testing.T) {
	rb := NewRouteBuilder()
	rb.AddRoute("app1.example.com", []string{"app2.example.com"}, []Backend{{IP: "10.0.0.1", Port: "8080"}})
	rb.AddRoute("app2.example.com", nil, []Backend{{IP: "10.0.0.2", Port: "8080"}})

	_, err := rb.Build()
	if err == nil {
		t.Fatal("Build() expected error for canonical/alias conflict, got nil")
	}
	if !strings.Contains(err.Error(), "alias") || !strings.Contains(err.Error(), "app2.example.com") {
		t.Fatalf("Build() error = %v, expected canonical/alias conflict context", err)
	}
}

func TestRouteBuilder_SetAPIDomain(t *testing.T) {
	rb := NewRouteBuilder()
	rb.SetAPIDomain("API.EXAMPLE.COM")

	config, err := rb.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if config.APIDomain() != "api.example.com" {
		t.Errorf("APIDomain() = %q, want %q", config.APIDomain(), "api.example.com")
	}
}

func TestConfig_IsKnownHost(t *testing.T) {
	rb := NewRouteBuilder()
	rb.SetAPIDomain("api.example.com")
	rb.AddRoute("app.example.com", []string{"www.app.example.com"}, nil)

	config, err := rb.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	tests := []struct {
		host string
		want bool
	}{
		{"app.example.com", true},
		{"www.app.example.com", true},
		{"API.EXAMPLE.COM", true},
		{"unknown.example.com", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := config.IsKnownHost(tt.host); got != tt.want {
			t.Errorf("IsKnownHost(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

func TestConfig_ResolveCanonical(t *testing.T) {
	rb := NewRouteBuilder()
	rb.AddRoute("app.example.com", []string{"www.app.example.com"}, nil)

	config, err := rb.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if canonical, ok := config.ResolveCanonical("www.app.example.com"); !ok || canonical != "app.example.com" {
		t.Errorf("ResolveCanonical(alias) = %q, %v; want app.example.com, true", canonical, ok)
	}

	if _, ok := config.ResolveCanonical("unknown.example.com"); ok {
		t.Error("ResolveCanonical(unknown) = true, want false")
	}
}
