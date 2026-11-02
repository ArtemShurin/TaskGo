package db

import "testing"

func TestDSNFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "defaults",
			env:  map[string]string{},
			want: "postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable",
		},
		{
			name: "DATABASE_URL has priority",
			env: map[string]string{
				"DATABASE_URL": "postgres://user:pass@db:5433/other?sslmode=disable",
				"DB_HOST":      "ignored",
			},
			want: "postgres://user:pass@db:5433/other?sslmode=disable",
		},
		{
			name: "separate variables",
			env: map[string]string{
				"DB_HOST": "db.local",
				"DB_PORT": "6543",
				"DB_USER": "app",
				"DB_PASS": "secret",
				"DB_NAME": "ledger",
			},
			want: "postgres://app:secret@db.local:6543/ledger?sslmode=disable",
		},
		{
			name: "special characters in password are escaped",
			env:  map[string]string{"DB_PASS": "p@ss:w/rd"},
			want: "postgres://postgres:p%40ss%3Aw%2Frd@localhost:5432/cashapp?sslmode=disable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{"DATABASE_URL", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASS", "DB_NAME"} {
				t.Setenv(key, tc.env[key])
			}

			if got := DSNFromEnv(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRedact(t *testing.T) {
	got := Redact("postgres://postgres:secret@localhost:5432/cashapp?sslmode=disable")
	want := "postgres://postgres:xxxxx@localhost:5432/cashapp?sslmode=disable"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
