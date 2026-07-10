package scenario

import (
	"encoding/json"
	"os"
)

func LoadFile(path string) (Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, err
	}
	var definition Definition
	if err := json.Unmarshal(data, &definition); err != nil {
		return Definition{}, err
	}
	if definition.Name == "" {
		definition.Name = path
	}
	return definition, nil
}

func LoadBootstrapFile(path string) (apiBootstrap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return apiBootstrap{}, err
	}
	var wrapper struct {
		Bootstrap apiBootstrap `json:"bootstrap"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Bootstrap.Routes) > 0 {
		return wrapper.Bootstrap, nil
	}
	var bootstrap apiBootstrap
	if err := json.Unmarshal(data, &bootstrap); err != nil {
		return apiBootstrap{}, err
	}
	return bootstrap, nil
}
