package transmission

import (
	"context"
	"net/http"
)

type mutationStartKey struct{}

func mutationStart(ctx context.Context) chan<- error {
	start, _ := ctx.Value(mutationStartKey{}).(chan<- error)
	return start
}

type signalingRoundTripper struct{ next http.RoundTripper }

func (transport signalingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if started := mutationStart(request.Context()); started != nil {
		select {
		case started <- nil:
		default:
		}
	}
	return transport.next.RoundTrip(request)
}
