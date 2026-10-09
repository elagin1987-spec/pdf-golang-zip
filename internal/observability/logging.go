package observability

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
)

func Logger() *slog.Logger { return slog.New(slog.NewJSONHandler(os.Stdout, nil)) }
func RequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
