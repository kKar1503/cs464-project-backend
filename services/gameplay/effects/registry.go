package effects

import (
	"encoding/json"
	"fmt"
)

// EffectFactory creates an Ability from a trigger type and JSON params.
type EffectFactory func(trigger string, params json.RawMessage) (Ability, error)

var registry = map[string]EffectFactory{
	"stats_change":        NewStatsChange,
	"deal_damage":         NewDealDamage,
	"destroy":             NewDestroy,
	"bounce":              NewBounce,
	"transform":           NewTransform,
	"summon_units":        NewSummonUnits,
	"set_hp":              NewSetHP,
	"elixir_overflow":     NewElixirOverflow,
	"reset_attack":        NewResetAttack,
	"double_attack_speed": NewDoubleAttackSpeed,
	"random_target":       NewRandomTarget,
	"skip_front_row":      NewSkipFrontRow,
	"reflect":             NewReflect,
	"shield":              NewShield,
	"self_damage":         NewSelfDamage,
	"damage_leader":       NewDamageLeader,
	"destroy_random":      NewDestroyRandom,
}

// ResolveAbility turns a DB row into a live Ability.
func ResolveAbility(def AbilityDefinition) (Ability, error) {
	factory, ok := registry[def.EffectType]
	if !ok {
		return nil, fmt.Errorf("unknown effect type: %s", def.EffectType)
	}
	return factory(def.TriggerType, def.Params)
}

// ResolveAllAbilities resolves a slice of ability definitions into live abilities.
func ResolveAllAbilities(defs []AbilityDefinition) ([]Ability, error) {
	abilities := make([]Ability, 0, len(defs))
	for _, def := range defs {
		a, err := ResolveAbility(def)
		if err != nil {
			return nil, fmt.Errorf("resolving ability for effect %s: %w", def.EffectType, err)
		}
		abilities = append(abilities, a)
	}
	return abilities, nil
}
