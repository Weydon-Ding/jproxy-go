package runtime

import "errors"

var ErrTransmissionRevisionStale = errors.New("transmission configuration revision is stale")

// TransmissionMutationAdmission atomically verifies the Transmission
// configuration revision and begins a mutation request.
type TransmissionMutationAdmission interface {
	AdmitTransmissionMutation(uint64, func()) error
}

// AdmitTransmissionMutation holds the publication lock only while it compares
// the revision and synchronously starts the request. Response I/O must happen
// after the callback returns, outside this lock.
func (p *provider) AdmitTransmissionMutation(revision uint64, begin func()) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.value.Load().TransmissionRevision != revision {
		return ErrTransmissionRevisionStale
	}
	begin()
	return nil
}
