package cache

import "testing"

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr bool
	}{
		{
			name: "defaults",
			env:  map[string]string{},
			want: Config{Addr: "localhost:6379", DB: 0},
		},
		{
			name: "all variables",
			env:  map[string]string{"REDIS_ADDR": "redis:6380", "REDIS_DB": "3", "REDIS_PASSWORD": "secret"},
			want: Config{Addr: "redis:6380", Password: "secret", DB: 3},
		},
		{
			name:    "non-numeric db",
			env:     map[string]string{"REDIS_DB": "abc"},
			wantErr: true,
		},
		{
			name:    "negative db",
			env:     map[string]string{"REDIS_DB": "-1"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{"REDIS_ADDR", "REDIS_DB", "REDIS_PASSWORD"} {
				t.Setenv(key, tc.env[key])
			}

			got, err := ConfigFromEnv()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got config %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
