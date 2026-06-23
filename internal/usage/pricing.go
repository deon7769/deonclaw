package usage

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ModelPrice struct {
	ID                            string `yaml:"-" json:"id"`
	Provider                      string `yaml:"provider" json:"provider"`
	Model                         string `yaml:"model" json:"model"`
	Currency                      string `yaml:"currency" json:"currency"`
	InputMicroUSDPerMillion       int64  `yaml:"input_microusd_per_million" json:"input_microusd_per_million"`
	OutputMicroUSDPerMillion      int64  `yaml:"output_microusd_per_million" json:"output_microusd_per_million"`
	CachedInputMicroUSDPerMillion int64  `yaml:"cached_input_microusd_per_million" json:"cached_input_microusd_per_million"`
	EffectiveAt                   string `yaml:"effective_at" json:"effective_at"`
}

type PricingConfig struct {
	ModelPrices map[string]ModelPrice `yaml:"model_prices"`
}

func LoadPricingConfig(path string) (PricingConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PricingConfig{}, fmt.Errorf("read pricing config %q: %w", path, err)
	}
	return ParsePricingConfig(data)
}

func ParsePricingConfig(data []byte) (PricingConfig, error) {
	var cfg PricingConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return PricingConfig{}, fmt.Errorf("parse pricing yaml: %w", err)
	}
	if cfg.ModelPrices == nil {
		cfg.ModelPrices = map[string]ModelPrice{}
	}
	for id, price := range cfg.ModelPrices {
		price.ID = strings.TrimSpace(id)
		cfg.ModelPrices[id] = price
	}
	return cfg, nil
}

func ValidatePricingConfig(cfg PricingConfig) error {
	var errs []error
	for id, price := range cfg.ModelPrices {
		if strings.TrimSpace(id) == "" {
			errs = append(errs, errors.New("model price id is required"))
			continue
		}
		if strings.TrimSpace(price.Provider) == "" {
			errs = append(errs, fmt.Errorf("model price %q provider is required", id))
		}
		if strings.TrimSpace(price.Model) == "" {
			errs = append(errs, fmt.Errorf("model price %q model is required", id))
		}
		currency := strings.TrimSpace(strings.ToUpper(price.Currency))
		if currency != "" && currency != "USD" {
			errs = append(errs, fmt.Errorf("model price %q currency %q is not supported in MVP", id, price.Currency))
		}
		if price.InputMicroUSDPerMillion < 0 || price.OutputMicroUSDPerMillion < 0 || price.CachedInputMicroUSDPerMillion < 0 {
			errs = append(errs, fmt.Errorf("model price %q rates must be non-negative", id))
		}
		if strings.TrimSpace(price.EffectiveAt) == "" {
			errs = append(errs, fmt.Errorf("model price %q effective_at is required", id))
		} else if _, err := time.Parse("2006-01-02", price.EffectiveAt); err != nil {
			errs = append(errs, fmt.Errorf("model price %q effective_at: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func LookupModelPrice(cfg PricingConfig, modelProfile string, at time.Time) (ModelPrice, error) {
	price, ok := cfg.ModelPrices[modelProfile]
	if !ok {
		return ModelPrice{}, fmt.Errorf("unknown model profile %q", modelProfile)
	}
	effective, err := time.Parse("2006-01-02", price.EffectiveAt)
	if err != nil {
		return ModelPrice{}, fmt.Errorf("model profile %q effective_at: %w", modelProfile, err)
	}
	if at.Before(effective) {
		return ModelPrice{}, fmt.Errorf("model profile %q price not effective at %s", modelProfile, at.Format(time.RFC3339))
	}
	price.ID = modelProfile
	return price, nil
}
