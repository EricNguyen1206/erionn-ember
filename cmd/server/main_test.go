package main

import "testing"

func TestLoadConfigUsesOnlyPortSettings(t *testing.T) {
	t.Setenv("HTTP_PORT", "18080")
	t.Setenv("GRPC_PORT", "19090")

	cfg := loadConfig()
	if cfg.HTTPPort != "18080" || cfg.GRPCPort != "19090" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
