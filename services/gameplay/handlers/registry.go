package handlers

// ActionHandler is a function that processes a specific game action
type ActionHandler func(ctx HandlerContext, msg *ClientMessage) error

// Action handler registry
var actionHandlers = make(map[string]ActionHandler)

// RegisterActionHandler registers an action handler
func RegisterActionHandler(action string, handler ActionHandler) {
	actionHandlers[action] = handler
}

// GetActionHandler retrieves an action handler
func GetActionHandler(action string) ActionHandler {
	return actionHandlers[action]
}

// Initialize all action handlers
func init() {
	RegisterActionHandler("CARD_PLACED", HandleCardPlaced)
	RegisterActionHandler("SELECT_CARD", HandleSelectCard)
	RegisterActionHandler("DESELECT_CARD", HandleDeselectCard)
	RegisterActionHandler("JOIN_GAME", HandleJoinGame)
	RegisterActionHandler("CLICK", HandleClick)
	RegisterActionHandler("SURRENDER", HandleSurrender)
	RegisterActionHandler("RECONNECT", HandleReconnect)
}
