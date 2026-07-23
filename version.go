package dpay

import "runtime"

// Version is the SDK version reported in the User-Agent header.
const Version = "0.1.0"

func userAgent() string {
	return "dpay-go-sdk/" + Version + " go/" + runtime.Version()
}
