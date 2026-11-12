package providers

import (
	"fmt"
	"sync"
	"time"
)

// map[string]*time.Location
var locations sync.Map

func LoadLocation(name string) (*time.Location, error) {
	if loc, ok := locations.Load(name); ok {
		return loc.(*time.Location), nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("unknown time zone %q: %w", name, err)
	}
	locations.Store(name, loc)
	return loc, nil
}
