package devices

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pico-cs/mqtt-gateway/internal/logger"
	"golang.org/x/exp/maps"
)

// LocoSet represents a set of locos.
type LocoSet struct {
	lg      logger.Logger
	locoMap map[string]*Loco
}

// NewLocoSet creates new loco set instance.
func NewLocoSet(lg logger.Logger) *LocoSet {
	if lg == nil {
		lg = logger.Null
	}
	return &LocoSet{lg: lg, locoMap: make(map[string]*Loco)}
}

// Items returns a loco map.
func (s *LocoSet) Items() map[string]*Loco { return maps.Clone(s.locoMap) }

// Add adds a loco via a loco configuration.
func (s *LocoSet) Add(config *LocoConfig) (*Loco, error) {
	loco, err := newLoco(s.lg, config)
	if err != nil {
		return nil, err
	}
	s.locoMap[config.Name] = loco
	return loco, nil
}

// Close closes all locos.
func (s *LocoSet) Close() error {
	var lastErr error
	for _, loco := range s.locoMap {
		if err := loco.close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// ServeHTTP implements the http.Handler interface.
func (s *LocoSet) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data := locoTplData{LocoMap: s.locoMap}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	if err := locoIdxTpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
}

// A Loco represents a loco.
type Loco struct {
	lg          logger.Logger
	config      *LocoConfig
	primary     *CS
	secondaries map[string]*CS
}

// newLoco returns a new loco instance.
func newLoco(lg logger.Logger, config *LocoConfig) (*Loco, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	return &Loco{lg: lg, config: config, secondaries: map[string]*CS{}}, nil
}

func (l *Loco) name() string { return l.config.Name }

func (l *Loco) isPrimary(cs *CS) bool { return cs == l.primary }

func (l *Loco) isSecondary(cs *CS) bool { _, ok := l.secondaries[cs.name()]; return ok }

func (l *Loco) setPrimary(cs *CS) error {
	if l.primary != nil {
		return fmt.Errorf("loco %s is already assigned to primary command station %s", l.name(), cs.name())
	}
	l.primary = cs
	return nil
}

func (l *Loco) unsetPrimary(cs *CS) error {
	if l.primary != cs {
		return fmt.Errorf("loco %s is not assigned to primary command station %s", l.name(), cs.name())
	}
	l.primary = nil
	return nil
}

func (l *Loco) addSecondary(cs *CS) error {
	if _, ok := l.secondaries[cs.name()]; ok {
		return fmt.Errorf("loco %s is already assigned to secondary command station %s", l.name(), cs.name())
	}
	l.secondaries[cs.name()] = cs
	return nil
}

func (l *Loco) delSecondary(cs *CS) error {
	if _, ok := l.secondaries[cs.name()]; !ok {
		return fmt.Errorf("loco %s is not assigned to secondary command station %s", l.name(), cs.name())
	}
	delete(l.secondaries, cs.name())
	return nil
}

func (l *Loco) addr() uint { return l.config.Addr }
func (l *Loco) iterFcts(fn func(name string, no uint)) {
	for name, fct := range l.config.Fcts {
		fn(name, fct.No)
	}
}

func (l *Loco) close() error { return nil }

// ServeHTTP implements the http.Handler interface.
func (l *Loco) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	b, err := json.MarshalIndent(l.config, "", indent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Write(b)
}
