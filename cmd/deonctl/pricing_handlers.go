package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/deon7769/deonclaw/internal/store"
	usagepkg "github.com/deon7769/deonclaw/internal/usage"
)

func runPricingValidate(configPath string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := usagepkg.LoadPricingConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "pricing validate failed: %v\n", err)
		return 1
	}
	if err := usagepkg.ValidatePricingConfig(cfg); err != nil {
		fmt.Fprintf(stderr, "pricing validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "pricing validate: ok")
	return 0
}

func runPricingSync(storePath, configPath string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := usagepkg.LoadPricingConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "pricing sync failed: %v\n", err)
		return 1
	}
	if err := usagepkg.ValidatePricingConfig(cfg); err != nil {
		fmt.Fprintf(stderr, "pricing sync failed: %v\n", err)
		return 1
	}
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "pricing sync failed: %v\n", err)
		return 1
	}
	defer db.Close()
	prices := make([]usagepkg.ModelPrice, 0, len(cfg.ModelPrices))
	for _, price := range cfg.ModelPrices {
		prices = append(prices, price)
	}
	if err := db.SyncModelPrices(context.Background(), prices); err != nil {
		fmt.Fprintf(stderr, "pricing sync failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "pricing sync: ok")
	return 0
}

func runPricingList(storePath string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "pricing list failed: %v\n", err)
		return 1
	}
	defer db.Close()
	prices, err := db.ListModelPrices(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "pricing list failed: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(prices)
	return 0
}

func runPricingShow(storePath, id string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "pricing show failed: %v\n", err)
		return 1
	}
	defer db.Close()
	price, err := db.ModelPrice(context.Background(), id)
	if err != nil {
		fmt.Fprintf(stderr, "pricing show failed: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(price)
	return 0
}
