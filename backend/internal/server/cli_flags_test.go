package server

import (
	"reflect"
	"testing"
)

func TestParseGlobalFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantClean []string
		wantDB    string
		wantKeep  bool
	}{
		{
			name:      "no flags",
			args:      []string{"cli", "--reset-admin"},
			wantClean: []string{"cli", "--reset-admin"},
			wantDB:    "",
			wantKeep:  false,
		},
		{
			name:      "db flag space separated",
			args:      []string{"cli", "--db", "/custom/path.db", "--list-admins"},
			wantClean: []string{"cli", "--list-admins"},
			wantDB:    "/custom/path.db",
			wantKeep:  false,
		},
		{
			name:      "db flag equals separated",
			args:      []string{"--db=/custom/path.db", "cli"},
			wantClean: []string{"cli"},
			wantDB:    "/custom/path.db",
			wantKeep:  false,
		},
		{
			name:      "acme-style meme flag",
			args:      []string{"cli", "--yes-I-know-root-db-mode-enough-go-ahead-please", "--reset-admin"},
			wantClean: []string{"cli", "--reset-admin"},
			wantDB:    "",
			wantKeep:  true,
		},
		{
			name:      "literal acme dns meme flag",
			args:      []string{"--yes-I-know-dns-manual-mode-enough-go-ahead-please", "cli"},
			wantClean: []string{"cli"},
			wantDB:    "",
			wantKeep:  true,
		},
		{
			name:      "short keep-root-db and no-chown",
			args:      []string{"cli", "--keep-root-db", "--db", "/data/gw.db"},
			wantClean: []string{"cli"},
			wantDB:    "/data/gw.db",
			wantKeep:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClean, gotDB, gotKeep := parseGlobalFlags(tt.args)
			if !reflect.DeepEqual(gotClean, tt.wantClean) {
				t.Errorf("parseGlobalFlags() clean = %v, want %v", gotClean, tt.wantClean)
			}
			if gotDB != tt.wantDB {
				t.Errorf("parseGlobalFlags() db = %v, want %v", gotDB, tt.wantDB)
			}
			if gotKeep != tt.wantKeep {
				t.Errorf("parseGlobalFlags() keep = %v, want %v", gotKeep, tt.wantKeep)
			}
		})
	}
}
