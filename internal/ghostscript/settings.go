package ghostscript

import (
	"fmt"
	"strings"
)

// formatIntArray formats an integer slice into a space-separated string.
func formatIntArray(arr []int) string {
	parts := make([]string, len(arr))
	for i, v := range arr {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, " ")
}

// BuildDistillerSettings constructs a trusted, server-defined PostScript distiller settings string.
// It sets NeverEmbed=[], the ColorACSImageDict and GrayACSImageDict with profile-specific
// QFactor and H/V sampling arrays, and preserves /Link annotations.
//
// Security invariant: This function strictly formats trusted server-defined ProfileSettings.
// No user-supplied text or snippets may be injected into this PostScript code.
func BuildDistillerSettings(s ProfileSettings) string {
	color_h := formatIntArray(s.ColorHSamples)
	color_v := formatIntArray(s.ColorVSamples)
	gray_h := formatIntArray(s.GrayHSamples)
	gray_v := formatIntArray(s.GrayVSamples)

	q_factor_str := fmt.Sprintf("%g", s.QFactor)

	return fmt.Sprintf(
		"/PreserveAnnotTypes [/Link] def << /NeverEmbed [] /ColorACSImageDict << /QFactor %s /HSamples [%s] /VSamples [%s] >> /GrayACSImageDict << /QFactor %s /HSamples [%s] /VSamples [%s] >> >> setdistillerparams",
		q_factor_str, color_h, color_v,
		q_factor_str, gray_h, gray_v,
	)
}
