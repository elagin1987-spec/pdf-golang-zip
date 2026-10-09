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
}
