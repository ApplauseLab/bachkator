package config

import "fmt"

type Loader func(path string, options LoadOptions) (*Project, error)

type LoaderRegistry struct {
	loaders map[string]Loader
}

func NewLoaderRegistry() *LoaderRegistry {
	return &LoaderRegistry{loaders: map[string]Loader{}}
}

func (r *LoaderRegistry) Register(family string, loader Loader) error {
	if family == "" {
		return fmt.Errorf("config loader has no family")
	}
	if loader == nil {
		return fmt.Errorf("config loader for %q is nil", family)
	}
	if _, exists := r.loaders[family]; exists {
		return fmt.Errorf("config loader for %q already registered", family)
	}
	r.loaders[family] = loader
	return nil
}

func (r *LoaderRegistry) Loader(family string) (Loader, error) {
	loader, ok := r.loaders[family]
	if !ok {
		return nil, fmt.Errorf("no config loader registered for %q", family)
	}
	return loader, nil
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
