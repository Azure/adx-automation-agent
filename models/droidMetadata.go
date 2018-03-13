package models

import (
	"io/ioutil"
	"log"

	yaml "gopkg.in/yaml.v2"
)

// DroidMetadata defines the data model used in metadata.yml
type DroidMetadata struct {
	Kind         string                `yaml:"kind"`
	Version      string                `yaml:"version"`
	Product      string                `yaml:"product"`
	Storage      bool                  `yaml:"storage"`
	Environments []DroidMetadataEnvDef `yaml:"environments"`
	Owners       []string              `yaml:"owners"`
}

// DroidMetadataEnvDef defines the data model of the environment variable definition in metadata.yml
type DroidMetadataEnvDef struct {
	Name  string `yaml:"name"`
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}

// ReadDroidMetadata reads the droid metadata from the metadata.yml file. Returns nil if the file is missing or the
// JSON unmarshal failed.
func ReadDroidMetadata(filePath string) *DroidMetadata {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("Fail to read %s.\n", filePath)
		return nil
	}

	var metadata DroidMetadata
	err = yaml.Unmarshal(content, &metadata)
	if err != nil {
		log.Printf("Fail to JSON unmarshal content from %s.\n", filePath)
		return nil
	}

	return &metadata
}
