package config

import (
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

const configKeyDelimiter = "."

func LoadFile(path string) (Config, error) {
	loader := koanf.New(configKeyDelimiter)
	if err := loader.Load(structs.Provider(Defaults(), "yaml"), nil); err != nil {
		return Config{}, fmt.Errorf("load config defaults: %w", err)
	}

	if path != "" {
		if err := loadYAMLFile(loader, path); err != nil {
			return Config{}, err
		}
	}

	if err := loadEnv(loader); err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := loader.UnmarshalWithConf("", &cfg, configUnmarshalConf()); err != nil {
		return Config{}, normalizeDecodeError(err)
	}
	return cfg, nil
}

func loadYAMLFile(loader *koanf.Koanf, path string) error {
	yamlLoader := koanf.New(configKeyDelimiter)
	if err := yamlLoader.Load(file.Provider(path), yaml.Parser()); err != nil {
		return fmt.Errorf("load config file %q: %w", path, err)
	}
	if err := rejectUnknownYAMLKeys(yamlLoader.Keys()); err != nil {
		return err
	}
	if err := loader.Load(file.Provider(path), yaml.Parser()); err != nil {
		return fmt.Errorf("load config file %q: %w", path, err)
	}
	return nil
}

func loadEnv(loader *koanf.Koanf) error {
	if err := validateConfigEnv(); err != nil {
		return err
	}

	provider := env.Provider(configKeyDelimiter, env.Opt{
		Prefix: "PROTORADAR_",
		TransformFunc: func(name string, value string) (string, any) {
			path := envPathForName(name)
			if path == "" {
				return "", nil
			}
			return path, value
		},
	})
	if err := loader.Load(provider, nil); err != nil {
		return fmt.Errorf("load config env: %w", err)
	}
	return nil
}

func normalizeDecodeError(err error) error {
	message := err.Error()
	for _, binding := range envBindings() {
		if !strings.Contains(message, "'"+binding.path+"'") && !strings.Contains(message, binding.path) {
			continue
		}
		switch binding.kind {
		case envBool:
			return fmt.Errorf("%s must be a boolean", binding.path)
		case envInt, envInt64:
			return fmt.Errorf("%s must be an integer", binding.path)
		case envDuration:
			return fmt.Errorf("%s must be a valid duration", binding.path)
		}
	}
	return fmt.Errorf("decode config: %w", err)
}

func configUnmarshalConf() koanf.UnmarshalConf {
	return koanf.UnmarshalConf{
		Tag: "yaml",
		DecoderConfig: &mapstructure.DecoderConfig{
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToTimeDurationHookFunc(),
			),
			WeaklyTypedInput: true,
		},
	}
}
