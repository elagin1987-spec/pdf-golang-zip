package ghostscript

import (
	"strings"
	"testing"
)

func TestBuildDistillerSettings(t *testing.T) {
	t.Run("balanced profile settings string", func(t *testing.T) {
		settings, err := GetProfileSettings(ProfileBalanced)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := BuildDistillerSettings(settings)
		expected := "/PreserveAnnotTypes [/Link] def << /NeverEmbed [] /ColorACSImageDict << /QFactor 0.4 /HSamples [1 1 1 1] /VSamples [1 1 1 1] >> /GrayACSImageDict << /QFactor 0.4 /HSamples [1 1 1 1] /VSamples [1 1 1 1] >> >> setdistillerparams"

		if got != expected {
			t.Fatalf("PostScript distiller settings mismatch:\nexpected: %s\ngot:      %s", expected, got)
		}
	})

	t.Run("compact profile settings string", func(t *testing.T) {
		settings, err := GetProfileSettings(ProfileCompact)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := BuildDistillerSettings(settings)
		expected := "/PreserveAnnotTypes [/Link] def << /NeverEmbed [] /ColorACSImageDict << /QFactor 0.76 /HSamples [2 1 1 2] /VSamples [2 1 1 2] >> /GrayACSImageDict << /QFactor 0.76 /HSamples [1 1 1 1] /VSamples [1 1 1 1] >> >> setdistillerparams"

		if got != expected {
			t.Fatalf("PostScript distiller settings mismatch:\nexpected: %s\ngot:      %s", expected, got)
		}
	})

	t.Run("invariants check", func(t *testing.T) {
		settings, err := GetProfileSettings(ProfileBalanced)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ps := BuildDistillerSettings(settings)
		if !strings.Contains(ps, "/PreserveAnnotTypes [/Link] def") {
			t.Fatalf("missing /PreserveAnnotTypes [/Link] def")
		}
		if !strings.Contains(ps, "/NeverEmbed []") {
			t.Fatalf("missing /NeverEmbed []")
		}
		if !strings.Contains(ps, "setdistillerparams") {
			t.Fatalf("missing setdistillerparams")
		}
	})
}
