package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"pdf-golang-zip/internal/compression"
	"pdf-golang-zip/internal/qpdf"
)

type Config struct {
	Auto                                                            compression.AutoSettings
	ListenAddr, TempDir, WorkerPath, QPDFPath, GSPath               string
	MaxFileBytes, MaxRequestBytes, MaxJobBytes                      int64
	MaxWorkers, MaxPages                                            int
	ProcessingTimeout, ReadHeaderTimeout, ReadTimeout, WriteTimeout time.Duration
	EnableLossy, AllowSpecialTypes                                  bool
}

func Default() Config {
	return Config{
		Auto:              compression.DefaultAutoSettings(),
		ListenAddr:        ":8080",
		TempDir:           "/work/jobs",
		WorkerPath:        "/usr/local/bin/pdfworker",
		QPDFPath:          "qpdf",
		GSPath:            "gs",
		MaxFileBytes:      300 << 20,
		MaxRequestBytes:   310 << 20,
		MaxJobBytes:       2048 << 20,
		MaxWorkers:        2,
		MaxPages:          qpdf.MaxDocumentPages,
		ProcessingTimeout: 120 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      180 * time.Second,
		EnableLossy:       true,
		AllowSpecialTypes: true,
	}
}

func Load() (Config, error) {
	c := Default()
	for k, p := range map[string]*string{
		"LISTEN_ADDR": &c.ListenAddr,
		"TEMP_DIR":    &c.TempDir,
		"WORKER_PATH": &c.WorkerPath,
		"QPDF_PATH":   &c.QPDFPath,
		"GS_PATH":     &c.GSPath,
	} {
		if v, ok := os.LookupEnv(k); ok {
			if v == "" {
				return c, fmt.Errorf("%s cannot be empty", k)
			}
			*p = v
		}
	}
	for k, p := range map[string]*int64{
		"MAX_FILE_BYTES":    &c.MaxFileBytes,
		"MAX_REQUEST_BYTES": &c.MaxRequestBytes,
		"MAX_JOB_BYTES":     &c.MaxJobBytes,
	} {
		if v, ok := os.LookupEnv(k); ok {
			n, e := strconv.ParseInt(v, 10, 64)
			if e != nil || n <= 0 {
				return c, fmt.Errorf("invalid %s", k)
			}
			*p = n
		}
	}
	for k, p := range map[string]*int{
		"MAX_WORKERS": &c.MaxWorkers,
		"MAX_PAGES":   &c.MaxPages,
	} {
		if v, ok := os.LookupEnv(k); ok {
			n, e := strconv.Atoi(v)
			if e != nil || n < 1 {
				return c, fmt.Errorf("invalid %s", k)
			}
			*p = n
		}
	}
	for k, p := range map[string]*time.Duration{
		"PROCESSING_TIMEOUT":  &c.ProcessingTimeout,
		"READ_HEADER_TIMEOUT": &c.ReadHeaderTimeout,
		"READ_TIMEOUT":        &c.ReadTimeout,
		"WRITE_TIMEOUT":       &c.WriteTimeout,
	} {
		if v, ok := os.LookupEnv(k); ok {
			d, e := time.ParseDuration(v)
			if e != nil || d <= 0 {
				return c, fmt.Errorf("invalid %s", k)
			}
			*p = d
		}
	}
	for key, target := range map[string]*int{"AUTO_JPEG_QUALITY": &c.Auto.JPEGQuality, "AUTO_MIN_SAVED_PERCENT": &c.Auto.MinSavedPercent, "AUTO_DPI": &c.Auto.DPI} {
		if value, ok := os.LookupEnv(key); ok {
			n, err := strconv.Atoi(value)
			if err != nil {
				return c, fmt.Errorf("invalid %s", key)
			}
			*target = n
		}
	}
	if err := c.Auto.Validate(); err != nil {
		return c, err
	}
	if v, ok := os.LookupEnv("ENABLE_LOSSY"); ok {
		b, e := strconv.ParseBool(v)
		if e != nil {
			return c, e
		}
		c.EnableLossy = b
	}
	if v, ok := os.LookupEnv("ALLOW_SPECIAL_TYPES"); ok {
		b, e := strconv.ParseBool(v)
		if e != nil {
			return c, e
		}
		c.AllowSpecialTypes = b
	}
	if c.MaxWorkers > 2 || c.MaxPages > qpdf.MaxDocumentPages || c.MaxFileBytes > 300<<20 ||
		c.MaxRequestBytes < c.MaxFileBytes || c.MaxRequestBytes > 310<<20 ||
		c.MaxJobBytes > 2048<<20 {
		return c, fmt.Errorf("limits exceed bounds or request limit is below file limit")
	}
	return c, nil
}
