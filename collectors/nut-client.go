package collectors

import nut "github.com/robbiet480/go.nut"

// NutClient abstracts the connection lifecycle and top-level commands.
type NutClient interface {
	GetUPSList() ([]NutUPS, error)
	Authenticate(username string, password string) (bool, error)
	Disconnect() (bool, error)

	// Temporary escape hatch for incremental refactoring
	GetUnderlyingClient() *nut.Client

	NewUPS(name string) (NutUPS, error)
}

// realNutClient wraps the concrete nut.Client so it implements our NutClient interface.
type realNutClient struct {
	client *nut.Client
}

func (r *realNutClient) GetUPSList() ([]NutUPS, error) {
	list, err := r.client.GetUPSList()
	if err != nil {
		return nil, err
	}
	// Convert concrete slice to interface slice
	ifaces := make([]NutUPS, len(list))
	for i, u := range list {
		ifaces[i] = &realNutUPS{ups: u}
	}
	return ifaces, nil
}

func (r *realNutClient) Authenticate(username, password string) (bool, error) {
	return r.client.Authenticate(username, password)
}

func (r *realNutClient) Disconnect() (bool, error) {
	return r.client.Disconnect()
}

func (r *realNutClient) GetUnderlyingClient() *nut.Client {
	return r.client
}

func (r *realNutClient) NewUPS(name string) (NutUPS, error) {
	// Since r.client is a *nut.Client, it compiles perfectly here!
	u, err := nut.NewUPS(name, r.client)
	if err != nil {
		return nil, err
	}
	return &realNutUPS{ups: u}, nil
}
