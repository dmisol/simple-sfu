package defs

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Conf struct {
	Port     string   `yaml:"port"`
	Hosts    []string `yaml:"hosts,omitempty"`
	Redirect string   `yaml:"redirect,omitempty"`
	Folder   string   `yaml:"folder"`

	*AnimConf `yaml:"anim,omitempty"`
}

type AnimConf struct {
	FPS    int    `yaml:"fps,omitempty"`
	W      int    `yaml:"width,omitempty"`
	H      int    `yaml:"height,omitempty"`
	Static string `yaml:"static,omitempty"`
}

func ReadConf(name string) (c *Conf, err error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return
	}

	c = &Conf{}
	err = yaml.Unmarshal(b, c)
	return
}
