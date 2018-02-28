package models

// DroidMetadata defines the data model used in metadata.yml
type DroidMetadata struct {
	Kind         string                `yaml:"kind"`
	Version      string                `yaml:"version"`
	Product      string                `yaml:"product"`
	Storage      bool                  `yaml:"storage"`
	Environments []DroidMetadataEnvDef `yaml:"environments"`
}

// DroidMetadataEnvDef defines the data model of the environment variable definition in metadata.yml
type DroidMetadataEnvDef struct {
	Name  string `yaml:"name"`
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}
