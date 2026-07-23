package dpay

import "github.com/dpayglobal/dpay-go-sdk/internal/wire"

// Fields is an ordered map for request fields the API accepts as free-form
// objects: billing address, shipping address and product entries. Order matters
// because the SDK serializes fields in insertion order.
type Fields struct {
	body *wire.Body
}

// NewFields returns an empty Fields.
func NewFields() *Fields {
	return &Fields{body: wire.NewBody()}
}

// Set stores value under key and returns the receiver so calls can be chained.
func (f *Fields) Set(key string, value any) *Fields {
	f.body.Set(key, value)
	return f
}

// MarshalJSON serializes the fields in insertion order.
func (f *Fields) MarshalJSON() ([]byte, error) {
	return f.body.MarshalJSON()
}
