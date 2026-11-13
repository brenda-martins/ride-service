package config

import (
	"log/slog"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		wantErr    bool
		wantEnv    string
		wantLevel  slog.Level
		wantFormat string
	}{
		{
			name:       "defaults in production",
			env:        map[string]string{"DATABASE_URL": "postgres://localhost/ride"},
			wantEnv:    EnvProd,
			wantLevel:  slog.LevelInfo,
			wantFormat: FormatJSON,
		},
		{
			name:       "debug enables verbose and readable logging",
			env:        map[string]string{"DATABASE_URL": "postgres://localhost/ride", "APP_ENV": "DEBUG"},
			wantEnv:    EnvDebug,
			wantLevel:  slog.LevelDebug,
			wantFormat: FormatText,
		},
		{
			name:       "explicit variables win over the environment",
			env:        map[string]string{"DATABASE_URL": "postgres://localhost/ride", "APP_ENV": "debug", "LOG_LEVEL": "warn", "LOG_FORMAT": "json"},
			wantEnv:    EnvDebug,
			wantLevel:  slog.LevelWarn,
			wantFormat: FormatJSON,
		},
		{
			name:    "missing DATABASE_URL",
			env:     map[string]string{},
			wantErr: true,
		},
		{
			name:    "unknown APP_ENV",
			env:     map[string]string{"DATABASE_URL": "postgres://localhost/ride", "APP_ENV": "staging"},
			wantErr: true,
		},
		{
			name:    "invalid LOG_LEVEL",
			env:     map[string]string{"DATABASE_URL": "postgres://localhost/ride", "LOG_LEVEL": "verbose"},
			wantErr: true,
		},
		{
			name:    "invalid LOG_FORMAT",
			env:     map[string]string{"DATABASE_URL": "postgres://localhost/ride", "LOG_FORMAT": "yaml"},
			wantErr: true,
		},
		{
			name:    "invalid SHUTDOWN_TIMEOUT",
			env:     map[string]string{"DATABASE_URL": "postgres://localhost/ride", "SHUTDOWN_TIMEOUT": "ten seconds"},
			wantErr: true,
		},
		{
			name:    "non-positive SHUTDOWN_TIMEOUT",
			env:     map[string]string{"DATABASE_URL": "postgres://localhost/ride", "SHUTDOWN_TIMEOUT": "0s"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, tt.env)

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() returned no error: %+v", cfg)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}

			if cfg.Env != tt.wantEnv {
				t.Errorf("Env = %q, want %q", cfg.Env, tt.wantEnv)
			}
			if cfg.LogLevel != tt.wantLevel {
				t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, tt.wantLevel)
			}
			if cfg.LogFormat != tt.wantFormat {
				t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, tt.wantFormat)
			}
			if cfg.ShutdownTimeout != 10*time.Second {
				t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 10*time.Second)
			}
		})
	}
}

func TestLoadShutdownTimeoutFromEnv(t *testing.T) {
	setEnv(t, map[string]string{
		"DATABASE_URL":     "postgres://localhost/ride",
		"SHUTDOWN_TIMEOUT": "30s",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 30*time.Second)
	}
}

// setEnv sets the test variables and clears the rest, so the machine's
// environment cannot interfere with the result.
func setEnv(t *testing.T, env map[string]string) {
	t.Helper()

	for _, key := range []string{
		"APP_ENV", "HTTP_ADDR", "DATABASE_URL",
		"LOG_LEVEL", "LOG_FORMAT", "SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(key, "")
	}

	for key, value := range env {
		t.Setenv(key, value)
	}
}
