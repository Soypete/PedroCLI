package database

import (
	"testing"
)

func TestParseDatabaseURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    *Config
		wantErr bool
	}{
		{
			name: "supabase pooler URL",
			url:  "postgresql://postgres.abcdefgh:mypassword@aws-0-us-west-1.pooler.supabase.com:6543/postgres?sslmode=require",
			want: &Config{
				Host:     "aws-0-us-west-1.pooler.supabase.com",
				Port:     6543,
				User:     "postgres.abcdefgh",
				Password: "mypassword",
				Database: "postgres",
				SSLMode:  "require",
			},
		},
		{
			name: "supabase direct URL",
			url:  "postgresql://postgres:mypassword@db.abcdefgh.supabase.co:5432/postgres?sslmode=require",
			want: &Config{
				Host:     "db.abcdefgh.supabase.co",
				Port:     5432,
				User:     "postgres",
				Password: "mypassword",
				Database: "postgres",
				SSLMode:  "require",
			},
		},
		{
			name: "standard postgres URL",
			url:  "postgres://pedrocli:pedrocli@localhost:5432/pedrocli?sslmode=disable",
			want: &Config{
				Host:     "localhost",
				Port:     5432,
				User:     "pedrocli",
				Password: "pedrocli",
				Database: "pedrocli",
				SSLMode:  "disable",
			},
		},
		{
			name: "URL without sslmode defaults to require",
			url:  "postgres://user:pass@host:5432/db",
			want: &Config{
				Host:     "host",
				Port:     5432,
				User:     "user",
				Password: "pass",
				Database: "db",
				SSLMode:  "require",
			},
		},
		{
			name: "URL without port defaults to 5432",
			url:  "postgres://user:pass@host/db",
			want: &Config{
				Host:     "host",
				Port:     5432,
				User:     "user",
				Password: "pass",
				Database: "db",
				SSLMode:  "require",
			},
		},
		{
			name:    "invalid scheme",
			url:     "mysql://user:pass@host:3306/db",
			wantErr: true,
		},
		{
			name:    "invalid URL",
			url:     "://not-a-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDatabaseURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDatabaseURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Host != tt.want.Host {
				t.Errorf("Host = %q, want %q", got.Host, tt.want.Host)
			}
			if got.Port != tt.want.Port {
				t.Errorf("Port = %d, want %d", got.Port, tt.want.Port)
			}
			if got.User != tt.want.User {
				t.Errorf("User = %q, want %q", got.User, tt.want.User)
			}
			if got.Password != tt.want.Password {
				t.Errorf("Password = %q, want %q", got.Password, tt.want.Password)
			}
			if got.Database != tt.want.Database {
				t.Errorf("Database = %q, want %q", got.Database, tt.want.Database)
			}
			if got.SSLMode != tt.want.SSLMode {
				t.Errorf("SSLMode = %q, want %q", got.SSLMode, tt.want.SSLMode)
			}
		})
	}
}
