package collectors

import nut "github.com/robbiet480/go.nut"

// NutUPS abstracts an individual UPS device and its properties.
type NutUPS interface {
	GetName() string
	GetDescription() string
	GetMaster() bool
	GetNumberOfLogins() int
	GetClients() []string
	GetCommands() []NutCommand
	GetVariables() []NutVariable
}

// realNutUPS wraps the concrete nut.UPS so it implements our NutUPS interface.
type realNutUPS struct {
	ups nut.UPS
}

func (r *realNutUPS) GetName() string        { return r.ups.Name }
func (r *realNutUPS) GetDescription() string { return r.ups.Description }
func (r *realNutUPS) GetMaster() bool        { return r.ups.Master }
func (r *realNutUPS) GetNumberOfLogins() int { return r.ups.NumberOfLogins }
func (r *realNutUPS) GetClients() []string   { return r.ups.Clients }

func (r *realNutUPS) GetCommands() []NutCommand {
	cmds := make([]NutCommand, len(r.ups.Commands))
	for i, c := range r.ups.Commands {
		cmds[i] = NutCommand{Name: c.Name, Description: c.Description}
	}
	return cmds
}

func (r *realNutUPS) GetVariables() []NutVariable {
	vars := make([]NutVariable, len(r.ups.Variables))
	for i, v := range r.ups.Variables {
		vars[i] = NutVariable{
			Name:          v.Name,
			Value:         v.Value,
			Type:          v.Type,
			Description:   v.Description,
			Writeable:     v.Writeable,
			MaximumLength: v.MaximumLength,
			OriginalType:  v.OriginalType,
		}
	}
	return vars
}

// NutVariable mirrors nut.Variable but decouples it from the third-party type.
type NutVariable struct {
	Name          string
	Value         interface{}
	Type          string
	Description   string
	Writeable     bool
	MaximumLength int
	OriginalType  string
}

// NutCommand mirrors nut.Command.
type NutCommand struct {
	Name        string
	Description string
}
