package cfenv

// An App holds information about the current app running on Cloud Foundry
type App struct {
	ID              string   `json:"instance_id"`      // id of the app
	Index           int      `json:"instance_index"`   // index of the app
	Name            string   `json:"name"`             // name of the app
	Host            string   `json:"host"`             // host of the app
	Port            int      `json:"port"`             // port of the app
	Version         string   `json:"version"`          // version of the app
	ApplicationURIs []string `json:"application_uris"` // application uri of the app
	SpaceID         string   `json:"space_id"`         // id of the space
	SpaceName       string   `json:"space_name"`       // name of the space
	Home            string   // root folder for the deployed app
	MemoryLimit     string   // maximum amount of memory that each instance of the application can consume
	WorkingDir      string   // present working directory, where the buildpack that processed the application ran
	TempDir         string   // directory location where temporary and staging files are stored
	User            string   // user account under which the DEA runs
	Services        Services // services bound to the app
}
