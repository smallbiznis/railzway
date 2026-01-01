package events

import "go.uber.org/fx"

var Module = fx.Module("events.outbox",
	fx.Provide(NewOutbox),
)
