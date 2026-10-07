package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_PORT", "")
	t.Setenv("JWT_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("port=%q", cfg.Port)
	}
	if cfg.RateLimit != 120 {
		t.Fatalf("rate limit=%d", cfg.RateLimit)
	}
}

func TestProductionSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_PORT", "8080")
	t.Setenv("JWT_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestProductionSecretTooShort(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_PORT", "8080")
	t.Setenv("JWT_SECRET", "short")
	if _, err := Load(); err == nil {
		t.Fatal("expected minimum secret length error")
	}
}

func TestInvalidPort(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_PORT", "abc")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadS3PathStyle(t *testing.T) {
	t.Setenv("S3_PATH_STYLE", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if !cfg.S3UsePathStyle {
		t.Fatal("S3UsePathStyle = false, want true")
	}
}

func TestProductionRequiresWebStaticDir(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_PORT", "8080")
	t.Setenv("JWT_SECRET", "12345678901234567890123456789012")
	t.Setenv("CORS_ORIGINS", "http://localhost:8080")
	t.Setenv("WEB_STATIC_DIR", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected WEB_STATIC_DIR requirement")
	}
}


func TestLocalStorageConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("STORAGE_PROVIDER", "local")
	t.Setenv("STORAGE_LOCAL_DIR", "/tmp/web-studio-assets")
	t.Setenv("STORAGE_SIGNING_SECRET", "local-signing-secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.StorageProvider != "local" || cfg.StorageLocalDir != "/tmp/web-studio-assets" || cfg.StorageSigningSecret != "local-signing-secret" {
		t.Fatalf("unexpected local storage config: %+v", cfg)
	}
}

func TestInvalidStorageProvider(t *testing.T) {
	t.Setenv("STORAGE_PROVIDER", "filesystem")
	if _, err := Load(); err == nil {
		t.Fatal("expected storage provider validation error")
	}
}

func TestProductionLocalStorageCanUseJWTSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_PORT", "8080")
	t.Setenv("JWT_SECRET", "12345678901234567890123456789012")
	t.Setenv("CORS_ORIGINS", "http://localhost:8080")
	t.Setenv("WEB_STATIC_DIR", "/app/web")
	t.Setenv("STORAGE_PROVIDER", "local")
	t.Setenv("STORAGE_SIGNING_SECRET", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.StorageSigningSecret != cfg.JWTSecret {
		t.Fatal("local storage should fall back to JWT secret when explicit signing secret is absent")
	}
}



func TestLoadDotEnvFile(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("APP_ENV", "development")
	t.Setenv("DOTENV_TEST", "")
	t.Setenv("WEB_STATIC_DIR", "")
	if err := os.WriteFile(".env", []byte("DATABASE_URL=postgres://dotenv/test\nJWT_SECRET=\"dotenv-secret\"\nDOTENV_TEST=loaded\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("DOTENV_TEST", "")
	if err := os.Unsetenv("DATABASE_URL"); err != nil { t.Fatal(err) }
	if err := os.Unsetenv("JWT_SECRET"); err != nil { t.Fatal(err) }
	if err := os.Unsetenv("DOTENV_TEST"); err != nil { t.Fatal(err) }

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://dotenv/test" {
		t.Fatalf("DatabaseURL=%q", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != "dotenv-secret" {
		t.Fatalf("JWTSecret=%q", cfg.JWTSecret)
	}
	if got := os.Getenv("DOTENV_TEST"); got != "loaded" {
		t.Fatalf("DOTENV_TEST=%q", got)
	}
}

func TestLoadDotEnvDoesNotOverrideEnvironment(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("DATABASE_URL", "postgres://environment/wins")
	if err := os.WriteFile(".env", []byte("DATABASE_URL=postgres://dotenv/loses\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://environment/wins" {
		t.Fatalf("DatabaseURL=%q", cfg.DatabaseURL)
	}
}
