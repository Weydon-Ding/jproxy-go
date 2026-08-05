package runtime

import "jproxy-go/internal/store/sqlite"

type preparedPublisher interface {
	publishPrepared(sqlite.Snapshot)
}

// PublishPrepared swaps a snapshot already validated in the writing
// transaction. It deliberately performs neither I/O nor validation.
func PublishPrepared(provider Provider, snapshot sqlite.Snapshot) bool {
	publisher, ok := provider.(preparedPublisher)
	if !ok {
		return false
	}
	publisher.publishPrepared(snapshot)
	return true
}
