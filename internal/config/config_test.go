package config

import "testing"

func TestConfigValidation(t *testing.T) {
	for _, v := range []string{"0", "3", "bad"} {
		t.Setenv("MAX_WORKERS", v)
		if _, err := Load(); err == nil {
			t.Fatal("accepted", v)
		}
	}
	t.Setenv("MAX_WORKERS", "2")
	t.Setenv("ENABLE_LOSSY", "false")
	c, err := Load()
	if err != nil || c.EnableLossy {
		t.Fatal(c, err)
	}
	t.Setenv("PROCESSING_TIMEOUT", "-1s")
	if _, err := Load(); err == nil {
		t.Fatal("negative timeout")
	}
	t.Setenv("PROCESSING_TIMEOUT", "120s")
	t.Setenv("ALLOW_SPECIAL_TYPES", "false")
	c2, err := Load()
	if err != nil || c2.AllowSpecialTypes {
		t.Fatal("expected AllowSpecialTypes false", err)
	}
	t.Setenv("MAX_FILE_BYTES", "350000000")
	if _, err := Load(); err == nil {
		t.Fatal("accepted MAX_FILE_BYTES above bound")
	}
}

func TestPageLimitBound(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{{"552", true}, {"1000", true}, {"1001", false}} {
		t.Setenv("MAX_PAGES", tc.value)
		_, err := Load()
		if (err == nil) != tc.valid {
			t.Fatalf("MAX_PAGES=%s: %v", tc.value, err)
		}
	}
}
