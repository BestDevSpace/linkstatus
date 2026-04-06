package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// Limits to keep background monitor light on CPU and syscalls (config is clamped on load).
const (
	MinProbeInterval = 3 * time.Second  // avoid sub-second polling storms
	MaxProbeInterval = 30 * time.Minute //
	MinProbeTimeout  = 400 * time.Millisecond
	MaxProbeTimeout  = 10 * time.Second
	probeCycleSlack  = 250 * time.Millisecond // interval must exceed one probe wave
)

type Config struct {
	ProbeInterval    time.Duration      `mapstructure:"probe_interval"`
	ProbeTimeout     time.Duration      `mapstructure:"probe_timeout"`
	PingTargets      []string           `mapstructure:"ping_targets"`
	DNSTargets       []string           `mapstructure:"dns_targets"`
	DNSDomain        string             `mapstructure:"dns_domain"`
	RatingThresholds map[string]float64 `mapstructure:"rating_thresholds"`
}

func defaultConfig() *Config {
	return &Config{
		// Shorter defaults: link-up / internet-down often waits full probe timeouts; poll more often.
		ProbeInterval: 5 * time.Second,
		ProbeTimeout:  1200 * time.Millisecond,
		PingTargets:   []string{"8.8.8.8", "1.1.1.1", "9.9.9.9"},
		DNSTargets:    []string{"8.8.8.8:53", "1.1.1.1:53", "9.9.9.9:53"},
		DNSDomain:     "google.com",
		RatingThresholds: map[string]float64{
			"5": 20,
			"4": 50,
			"3": 100,
			"2": 300,
		},
	}
}

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get home directory: %w", err)
	}
	return filepath.Join(home, ".linkstatus"), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func Load() (*Config, error) {
	cfgPath, err := ConfigPath()
	if err != nil {
		return defaultConfig(), nil
	}

	v := viper.New()
	v.SetConfigFile(cfgPath)
	v.SetDefault("probe_interval", "5s")
	v.SetDefault("probe_timeout", "1200ms")
	v.SetDefault("ping_targets", []string{"8.8.8.8", "1.1.1.1", "9.9.9.9"})
	v.SetDefault("dns_targets", []string{"8.8.8.8:53", "1.1.1.1:53", "9.9.9.9:53"})
	v.SetDefault("dns_domain", "google.com")
	v.SetDefault("rating_thresholds", map[string]float64{"5": 20, "4": 50, "3": 100, "2": 300})

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return defaultConfig(), nil
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return defaultConfig(), nil
	}
	normalizeProbeSettings(cfg)
	return cfg, nil
}

func normalizeProbeSettings(cfg *Config) {
	d := defaultConfig()
	if cfg.ProbeInterval <= 0 {
		cfg.ProbeInterval = d.ProbeInterval
	}
	if cfg.ProbeTimeout <= 0 {
		cfg.ProbeTimeout = d.ProbeTimeout
	}
	if cfg.ProbeTimeout < MinProbeTimeout {
		cfg.ProbeTimeout = MinProbeTimeout
	}
	if cfg.ProbeTimeout > MaxProbeTimeout {
		cfg.ProbeTimeout = MaxProbeTimeout
	}
	if cfg.ProbeInterval < MinProbeInterval {
		cfg.ProbeInterval = MinProbeInterval
	}
	if cfg.ProbeInterval > MaxProbeInterval {
		cfg.ProbeInterval = MaxProbeInterval
	}
	// One cycle runs ICMP then maybe DNS (parallel per target); keep interval > timeout so ticks do not overlap work.
	if cfg.ProbeInterval < cfg.ProbeTimeout+probeCycleSlack {
		cfg.ProbeInterval = cfg.ProbeTimeout + probeCycleSlack
	}
}

func Save(cfg *Config) error {
	cfgPath, err := ConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(cfgPath)
	v.Set("probe_interval", cfg.ProbeInterval.String())
	v.Set("probe_timeout", cfg.ProbeTimeout.String())
	v.Set("ping_targets", cfg.PingTargets)
	v.Set("dns_targets", cfg.DNSTargets)
	v.Set("dns_domain", cfg.DNSDomain)
	v.Set("rating_thresholds", cfg.RatingThresholds)

	return v.WriteConfig()
}

func Reset() error {
	return Save(defaultConfig())
}
