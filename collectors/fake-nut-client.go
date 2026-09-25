package collectors

import nut "github.com/robbiet480/go.nut"

// --- Expanded Fakes/Mocks ---

type fakeNutClient struct {
	getUPSListFunc func() ([]NutUPS, error)
}

func (f *fakeNutClient) GetUPSList() ([]NutUPS, error) { return f.getUPSListFunc() }

func (f *fakeNutClient) Disconnect() (bool, error) {
	return true, nil
}

func (f *fakeNutClient) Authenticate(username string, password string) (bool, error) {
	return true, nil
}

func (f *fakeNutClient) GetUnderlyingClient() *nut.Client {
	return nil
}

func (f *fakeNutClient) NewUPS(name string) (NutUPS, error) {
	return fakeNutUPS{}, nil
}

type fakeNutUPS struct {
	name        string
	variables   []NutVariable
	commands    []NutCommand
	clients     []string
	description string
	master      bool
	logins      int
}

func (f fakeNutUPS) GetName() string             { return f.name }
func (f fakeNutUPS) GetVariables() []NutVariable { return f.variables }
func (f fakeNutUPS) GetCommands() []NutCommand   { return f.commands }
func (f fakeNutUPS) GetClients() []string        { return f.clients }
func (f fakeNutUPS) GetDescription() string      { return f.description }
func (f fakeNutUPS) GetMaster() bool             { return f.master }
func (f fakeNutUPS) GetNumberOfLogins() int      { return f.logins }
