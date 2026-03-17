package main

import "testing"

func TestLoadConfigUsesOnlyPortSettings(t *testing.T) {
	t.Setenv("GRPC_PORT", "19090")

	cfg := loadConfig()
	if cfg.GRPCPort != "19090" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
