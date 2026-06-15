package config

import (
	"fmt"

	typedregistry "github.com/applauselab/bachkator/internal/registry"
)

type Loader func(path string, options LoadOptions) (*Project, error)

type LoaderRegistry struct {
	loaders typedregistry.Registry[string, Loader]
}

func NewLoaderRegistry() *LoaderRegistry {
	return &LoaderRegistry{}
}

func (r *LoaderRegistry) Register(family string, loader Loader) error {
	if family == "" {
		return fmt.Errorf("config loader has no family")
	}
	if loader == nil {
		return fmt.Errorf("config loader for %q is nil", family)
	}
	return r.loaders.Register(family, loader, duplicateLoaderError)
}

func (r *LoaderRegistry) Loader(family string) (Loader, error) {
	return r.loaders.Get(family, missingLoaderError)
}

func duplicateLoaderError(family string) error {
	return fmt.Errorf("config loader for %q already registered", family)
}

func missingLoaderError(family string) error {
	return fmt.Errorf("no config loader registered for %q", family)
}

func BuiltinLoaderRegistry() *LoaderRegistry {
	registry := NewLoaderRegistry()
	mustRegisterLoader(registry, "bachfile", LoadWithOptions)
	return registry
}

func mustRegisterLoader(registry *LoaderRegistry, family string, loader Loader) {
	if err := registry.Register(family, loader); err != nil {
		panic(err)
	}
}
