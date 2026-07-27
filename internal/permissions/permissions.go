package permissions

type Filesystem struct {
	Read  []string `json:"read,omitempty" yaml:"read,omitempty"`
	Write []string `json:"write,omitempty" yaml:"write,omitempty"`
}

type Network struct {
	Local    bool `json:"local" yaml:"local"`
	Internet bool `json:"internet" yaml:"internet"`
}

type Set struct {
	Filesystem      Filesystem `json:"filesystem" yaml:"filesystem"`
	Network         Network    `json:"network" yaml:"network"`
	StartOnLogin    bool       `json:"startOnLogin" yaml:"startOnLogin"`
	RunInBackground bool       `json:"runInBackground" yaml:"runInBackground"`
}

type Statement struct {
	ID          string
	Description string
	Granted     bool
}

func Describe(value Set) []Statement {
	statements := []Statement{
		{ID: "network.local", Description: "Accept connections from your local network", Granted: value.Network.Local},
		{ID: "network.internet", Description: "Access internet services", Granted: value.Network.Internet},
		{ID: "startup.login", Description: "Start after you log in", Granted: value.StartOnLogin},
		{ID: "process.background", Description: "Run in the background", Granted: value.RunInBackground},
	}
	if len(value.Filesystem.Read) > 0 {
		statements = append(statements, Statement{
			ID:          "filesystem.read",
			Description: "Read files in the folder you select",
			Granted:     true,
		})
	}
	if len(value.Filesystem.Write) > 0 {
		statements = append(statements, Statement{
			ID:          "filesystem.write",
			Description: "Write files into the folder you select",
			Granted:     true,
		})
	}
	return statements
}
