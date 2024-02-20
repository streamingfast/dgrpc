package pbacme

import "time"

func (x *GetPingRequest) GetResponseDelay() time.Duration {
	if x != nil {
		return time.Duration(x.ResponseDelayInMillis) * time.Millisecond
	}
	return time.Duration(0)
}

func (x *StreamPingRequest) GetTerminatesAfter() time.Duration {
	if x != nil {
		return time.Duration(x.TerminatesAfterMillis) * time.Millisecond
	}
	return time.Duration(0)
}

func (x *StreamPingRequest) GetResponseDelay() time.Duration {
	if x != nil {
		return time.Duration(x.ResponseDelayInMillis) * time.Millisecond
	}
	return time.Duration(0)
}
