package rpcclient

type Registry struct {
	endpoints map[string]string
}

func NewRegistry(endpoints map[string]string) *Registry {
	copyEndpoints := make(map[string]string, len(endpoints))
	for name, endpoint := range endpoints {
		copyEndpoints[name] = endpoint
	}
	return &Registry{endpoints: copyEndpoints}
}

func (r *Registry) Endpoint(name string) (string, bool) {
	endpoint, ok := r.endpoints[name]
	return endpoint, ok
}
