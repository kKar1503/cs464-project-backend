package effects

// FireTrigger fires all abilities on the source card matching the given trigger type.
func FireTrigger(trigger string, source *CardInstance, ctx *EffectContext) []EffectEvent {
	var events []EffectEvent
	for _, ability := range source.Abilities {
		if ability.TriggerType() == trigger {
			events = append(events, ability.Execute(ctx)...)
		}
	}
	return events
}

// GetTargetModifier returns the first TargetModifier ability on the card, or nil.
func GetTargetModifier(card *CardInstance) TargetModifier {
	for _, ability := range card.Abilities {
		if tm, ok := ability.(TargetModifier); ok {
			return tm
		}
	}
	return nil
}
