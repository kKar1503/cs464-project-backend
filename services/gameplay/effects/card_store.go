package effects

import "sync"

// CardDefinitionStore holds all card definitions loaded at startup.
// Used by transform and summon_units effects to look up card templates.
type CardDefinitionStore struct {
	mu          sync.RWMutex
	definitions map[int]*CardDefinition
}

// NewCardDefinitionStore creates a new store from a slice of definitions.
func NewCardDefinitionStore(defs []*CardDefinition) *CardDefinitionStore {
	m := make(map[int]*CardDefinition, len(defs))
	for _, d := range defs {
		m[d.CardID] = d
	}
	return &CardDefinitionStore{definitions: m}
}

// Get returns a card definition by ID, or nil if not found.
func (s *CardDefinitionStore) Get(cardID int) *CardDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.definitions[cardID]
}

// NewCardInstance creates a new CardInstance from a definition with resolved abilities.
func NewCardInstance(def *CardDefinition, instanceID int) (*CardInstance, error) {
	abilities, err := ResolveAllAbilities(def.Abilities)
	if err != nil {
		return nil, err
	}
	return &CardInstance{
		InstanceID:           instanceID,
		Definition:           def,
		CurrentAtk:           def.BaseAtk,
		CurrentHP:            def.BaseHP,
		MaxHP:                def.BaseHP,
		ChargeTicksRemaining: ChargeTicksTotal,
		ChargeTicksTotal:     ChargeTicksTotal,
		IsCharging:           true,
		Abilities:            abilities,
	}, nil
}

const ChargeTicksTotal = 40 // 10 seconds at 4 ticks/sec
