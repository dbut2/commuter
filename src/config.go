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
	Type         string     `yaml:"type"`
	Home         [2]float64 `yaml:"home"`
	Work         [2]float64 `yaml:"work"`
	MaxDistance  float64    `yaml:"max_distance"`
	Margin       float64    `yaml:"margin"`
	Name         string     `yaml:"name"`
	Start        string     `yaml:"start"`
	End          string     `yaml:"end"`
	Types        []string   `yaml:"types"`
	Template     string     `yaml:"template"`
	ParkrunnerID int64      `yaml:"parkrunner_id"`
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
	users   map[int64][]Updater
	pending map[int64][]PendingUpdater
}

func loadConfig() Config {
	return parseConfig(configFile)
}

func toTypes(raw []string) []Type {
	types := make([]Type, len(raw))
	for i, t := range raw {
		types[i] = Type(t)
	}
	return types
}

func parseConfig(data []byte) Config {
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		panic(fmt.Sprintf("invalid config.yaml: %v", err))
	}

	users := make(map[int64][]Updater)
	pending := make(map[int64][]PendingUpdater)
	for userID, entries := range raw.Users {
		var updaters []Updater
		var pendingUpdaters []PendingUpdater
		for _, entry := range entries {
			switch entry.Type {
			case "commute":
				updaters = append(updaters, Commute(entry.Home, entry.Work, entry.MaxDistance, entry.Margin, toTypes(entry.Types)...))

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

				updaters = append(updaters, Challenge(formatter, entry.Start, entry.End, toTypes(entry.Types)...))

			case "parkrun":
				pendingUpdaters = append(pendingUpdaters, NewParkrun(userID, entry.ParkrunnerID, toTypes(entry.Types)...))

			default:
				panic(fmt.Sprintf("unknown updater type %q for user %d", entry.Type, userID))
			}
		}
		users[userID] = updaters
		pending[userID] = pendingUpdaters
	}

	return Config{users: users, pending: pending}
}

func (c Config) UpdatersForUser(userID int64) ([]Updater, bool) {
	updaters, ok := c.users[userID]
	return updaters, ok
}

func (c Config) PendingUpdatersForUser(userID int64) ([]PendingUpdater, bool) {
	pending, ok := c.pending[userID]
	return pending, ok
}

func (c Config) PendingUpdater(ownerID int64, typ string) (PendingUpdater, bool) {
	for _, p := range c.pending[ownerID] {
		if p.Type() == typ {
			return p, true
		}
	}
	return nil, false
}
