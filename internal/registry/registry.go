package registry

type Registry[K comparable, V any] struct {
	values map[K]V
}

func New[K comparable, V any]() *Registry[K, V] {
	return &Registry[K, V]{}
}

func (r *Registry[K, V]) Register(key K, value V, duplicate func(K) error) error {
	if r.values == nil {
		r.values = map[K]V{}
	}
	if _, exists := r.values[key]; exists {
		return duplicate(key)
	}
	r.values[key] = value
	return nil
}

func (r *Registry[K, V]) Get(key K, missing func(K) error) (V, error) {
	value, ok := r.values[key]
	if !ok {
		var zero V
		return zero, missing(key)
	}
	return value, nil
}
