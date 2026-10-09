package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr, TempDir, WorkerPath, QPDFPath, GSPath               string
	MaxFileBytes, MaxRequestBytes, MaxJobBytes                      int64
	MaxWorkers, MaxPages                                            int
	ProcessingTimeout, ReadHeaderTimeout, ReadTimeout, WriteTimeout time.Duration
	EnableLossy                                                     bool
}

func Default() Config {
	return Config{ListenAddr: ":8080", TempDir: "/work/jobs", WorkerPath: "/usr/local/bin/pdfworker", QPDFPath: "qpdf", GSPath: "gs",
		MaxFileBytes: 50 << 20, MaxRequestBytes: 51 << 20, MaxJobBytes: 512 << 20, MaxWorkers: 2, MaxPages: 500,
		ProcessingTimeout: 120 * time.Second, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 60 * time.Second, WriteTimeout: 180 * time.Second, EnableLossy: true}
}

func Load() (Config, error) {
	c := Default()
	for k, p := range map[string]*string{"LISTEN_ADDR": &c.ListenAddr, "TEMP_DIR": &c.TempDir, "WORKER_PATH": &c.WorkerPath, "QPDF_PATH": &c.QPDFPath, "GS_PATH": &c.GSPath} {
		if v, ok := os.LookupEnv(k); ok {
			if v == "" {
				return c, fmt.Errorf("%s cannot be empty", k)
			}
			*p = v
		}
	}
	for k, p := range map[string]*int64{"MAX_FILE_BYTES": &c.MaxFileBytes, "MAX_REQUEST_BYTES": &c.MaxRequestBytes, "MAX_JOB_BYTES": &c.MaxJobBytes} {
		if v, ok := os.LookupEnv(k); ok {
			n, e := strconv.ParseInt(v, 10, 64)
			if e != nil || n <= 0 {
				return c, fmt.Errorf("invalid %s", k)
			}
			*p = n
		}
	}
	for k, p := range map[string]*int{"MAX_WORKERS": &c.MaxWorkers, "MAX_PAGES": &c.MaxPages} {
		if v, ok := os.LookupEnv(k); ok {
			n, e := strconv.Atoi(v)
			if e != nil || n < 1 {
				return c, fmt.Errorf("invalid %s", k)
			}
			*p = n
		}
	}
	for k, p := range map[string]*time.Duration{"PROCESSING_TIMEOUT": &c.ProcessingTimeout, "READ_HEADER_TIMEOUT": &c.ReadHeaderTimeout, "READ_TIMEOUT": &c.ReadTimeout, "WRITE_TIMEOUT": &c.WriteTimeout} {
		if v, ok := os.LookupEnv(k); ok {
			d, e := time.ParseDuration(v)
			if e != nil || d <= 0 {
				return c, fmt.Errorf("invalid %s", k)
			}
			*p = d
		}
	}
	if v, ok := os.LookupEnv("ENABLE_LOSSY"); ok {
		b, e := strconv.ParseBool(v)
		if e != nil {
			return c, e
		}
		c.EnableLossy = b
	}
	if c.MaxWorkers > 2 || c.MaxPages > 500 || c.MaxFileBytes > 50<<20 || c.MaxRequestBytes < c.MaxFileBytes || c.MaxRequestBytes > 51<<20 || c.MaxJobBytes > 512<<20 {
		return c, fmt.Errorf("limits exceed GS-v1 bounds or request limit is below file limit")
	}
	return c, nil
}
