// Package cfenv provides information about the current app deployed on Cloud Foundry, including any bound service(s).
package cfenv

import (
	"encoding/json"

	"github.com/mitchellh/mapstructure"
)

// New creates a new App with the provided environment.
func New(env map[string]string) (*App, error) {
	var app App
	appVar := env["VCAP_APPLICATION"]
	if err := json.Unmarshal([]byte(appVar), &app); err != nil {
		return nil, err
	}
	app.Home = env["HOME"]
	app.MemoryLimit = env["MEMORY_LIMIT"]
	app.WorkingDir = env["PWD"]
	app.TempDir = env["TMPDIR"]
	app.User = env["USER"]
	var rawServices map[string]interface{}
	servicesVar := env["VCAP_SERVICES"]
	if err := json.Unmarshal([]byte(servicesVar), &rawServices); err != nil {
		return nil, err
	}

	services := make(map[string][]Service)
	for k, v := range rawServices {
		var serviceInstances []Service
		if err := mapstructure.WeakDecode(v, &serviceInstances); err != nil {
			return nil, err
		}
		services[k] = serviceInstances
	}
	app.Services = services
	return &app, nil
}

// Current creates a new App with the current environment.
func Current() (*App, error) {
	return New(CurrentEnv())
}
