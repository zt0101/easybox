package web

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
)

// Under systemd the stop signal reaches Xray and the panel at once, so the panel
// often finds Xray already gone. That is a clean shutdown, not an error.
func TestStopWithXrayAlreadyGoneIsClean(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

	if err := NewServer().Stop(); err != nil {
		t.Fatalf("stop with no running Xray: %v", err)
	}
}
