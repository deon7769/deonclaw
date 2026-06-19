package retrievalcontext

import "context"

// ProviderTransportRequest carries metadata for a future outbound provider call.
// Task 22.38 skeleton only — no payload text is included.
type ProviderTransportRequest struct {
	ProviderPayloadSHA256 string
	DispatchConfigSHA256  string
	ApprovalSHA256        string
}

// ProviderTransportResponse is the result of a provider transport delivery attempt.
type ProviderTransportResponse struct {
	Delivered bool
}

// ProviderTransport is the outbound boundary for provider API calls.
// Real implementations are not enabled in Task 22.38.
type ProviderTransport interface {
	Deliver(ctx context.Context, req ProviderTransportRequest) (ProviderTransportResponse, error)
}

// BlockedProviderTransport is a no-op transport that never delivers payloads.
type BlockedProviderTransport struct{}

func (BlockedProviderTransport) Deliver(context.Context, ProviderTransportRequest) (ProviderTransportResponse, error) {
	return ProviderTransportResponse{Delivered: false}, ErrProviderTransportNotEnabled
}
