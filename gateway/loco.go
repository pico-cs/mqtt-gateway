package gateway

import (
	"sync"

	"golang.org/x/exp/maps"
)

// LocoFct represents a loco function.
type LocoFct struct {
	// loco decoder function number
	No uint
}

// A Loco represents a loco.
type Loco struct {
	mu           sync.RWMutex
	name         string
	addr         uint
	fcts         map[string]LocoFct
	primary      string
	secondaryMap map[string]bool
}

// newLoco returns a new loco instance.
func newLoco(config *LocoConfig) (*Loco, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	return &Loco{
		name:         config.Name,
		addr:         config.Addr,
		fcts:         maps.Clone(config.Fcts),
		secondaryMap: make(map[string]bool),
	}, nil
}

// Name returns the loco name.
func (l *Loco) Name() string { return l.name }

// Primary retirns the name of the primary command station.
func (l *Loco) Primary() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.primary
}

// SecondaryList returns the list of secondary command stations.
func (l *Loco) SecondaryList() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return maps.Keys(l.secondaryMap)
}

func (l *Loco) isSecondary(csName string) bool {
	_, ok := l.secondaryMap[csName]
	return ok
}

func (l *Loco) addSecondary(csName string) {
	l.secondaryMap[csName] = true
}
