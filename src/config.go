package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed config.yaml
var configFile []byte

type updaterConfig struct {
	Type        string     `yaml:"type"`
	Home        [2]float64 `yaml:"home"`
	Work        [2]float64 `yaml:"work"`
	MaxDistance float64    `yaml:"max_distance"`
	Margin      float64    `yaml:"margin"`
	Name        string     `yaml:"name"`
	Start       string     `yaml:"start"`
	End         string     `yaml:"end"`
	Timezone    string     `yaml:"timezone"`
	Types       []string   `yaml:"types"`
	Template    string     `yaml:"template"`
}

type rawConfig struct {
	Users map[int64][]updaterConfig `yaml:"users"`
}

type challengeData struct {
	Day      int
	Distance float64
	Duration string
}

type Config struct {
	users map[int64][]Updater
}

func loadConfig() Config {
	return parseConfig(configFile)
}

func parseConfig(data []byte) Config {
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		panic(fmt.Sprintf("invalid config.yaml: %v", err))
	}

	users := make(map[int64][]Updater)
	for userID, entries := range raw.Users {
		var updaters []Updater
		for _, entry := range entries {
			switch entry.Type {
			case "commute":
				types := make([]Type, len(entry.Types))
				for i, t := range entry.Types {
					types[i] = Type(t)
				}
				updaters = append(updaters, Commute(entry.Home, entry.Work, entry.MaxDistance, entry.Margin, types...))

			case "challenge":
				tmpl, err := template.New(entry.Name).Parse(entry.Template)
				if err != nil {
					panic(fmt.Sprintf("invalid template for challenge %q: %v", entry.Name, err))
				}

				formatter := func(day int, distance float64, duration time.Duration) string {
					data := challengeData{
						Day:      day,
						Distance: distance,
						Duration: durationString(duration),
					}
					var buf bytes.Buffer
					if err := tmpl.Execute(&buf, data); err != nil {
						panic(fmt.Sprintf("template execution error for challenge %q: %v", entry.Name, err))
					}
					return buf.String()
				}

				types := make([]Type, len(entry.Types))
				for i, t := range entry.Types {
					types[i] = Type(t)
				}
				updaters = append(updaters, Challenge(formatter, entry.Start, entry.End, entry.Timezone, types...))

			default:
				panic(fmt.Sprintf("unknown updater type %q for user %d", entry.Type, userID))
			}
		}
		users[userID] = updaters
	}

	return Config{users: users}
}

func (c Config) UpdatersForUser(userID int64) ([]Updater, bool) {
	updaters, ok := c.users[userID]
	return updaters, ok
}
