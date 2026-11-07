package main

import (
	"bufio"
	"os"
	"strings"
)

type config struct {
	Port          string // PORT, default 8081
	DatabasePath  string // DATABASE_PATH, default data/evcal.sqlite
	PublicBaseURL string // PUBLIC_BASE_URL, used for pairing links and the Secure cookie flag
}

// loadConfig reads the environment, after filling unset variables from a
// .env file in the working directory if there is one. The server and the CLI
// commands share it, so run both from the same directory.
func loadConfig() config {
	loadDotEnv(".env")
	c := config{
		Port:          env("PORT", "8081"),
		DatabasePath:  env("DATABASE_PATH", "data/evcal.sqlite"),
		PublicBaseURL: os.Getenv("PUBLIC_BASE_URL"),
	}
	if c.PublicBaseURL == "" {
		c.PublicBaseURL = "http://localhost:" + c.Port
	}
	c.PublicBaseURL = strings.TrimRight(c.PublicBaseURL, "/")
	return c
}

func (c config) secureCookies() bool { return strings.HasPrefix(c.PublicBaseURL, "https://") }

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(line, "#") {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if _, set := os.LookupEnv(key); !set {
			os.Setenv(key, value)
		}
	}
}
