package effects

// Ability is the core polymorphic interface.
// Each effect_type in the DB becomes a struct that implements this.
type Ability interface {
	TriggerType() string
	Execute(ctx *EffectContext) []EffectEvent
}

// TargetModifier is implemented by abilities that change attack targeting
// (random_target, skip_front_row). The game loop checks for this before
// resolving the default target.
type TargetModifier interface {
	ModifyTarget(ctx *EffectContext) *TargetOverride
}
