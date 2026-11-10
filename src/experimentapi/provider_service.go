package experimentapi

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/providers"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1/tradingcopilotv1connect"
)

// providerServer implements tradingcopilotv1connect.ProviderServiceHandler over the providers service.
type providerServer struct {
	svc *providers.Service
}

var _ tradingcopilotv1connect.ProviderServiceHandler = providerServer{}

// providerError maps the providers errors to Connect codes.
func providerError(err error) error {
	var verr providers.ValidationError
	switch {
	case errors.Is(err, providers.ErrUnknownProvider):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, providers.ErrStoreRequired), errors.Is(err, providers.ErrUpstream):
		return connect.NewError(connect.CodeUnavailable, err)
	case errors.As(err, &verr):
		return invalidArgument(verr.Msg)
	default:
		return internal(err)
	}
}

func (p providerServer) ListProviders(ctx context.Context, _ *connect.Request[v1.ListProvidersRequest]) (*connect.Response[v1.ListProvidersResponse], error) {
	views, err := p.svc.List(ctx)
	if err != nil {
		return nil, providerError(err)
	}
	out := &v1.ListProvidersResponse{}
	for _, v := range views {
		out.Providers = append(out.Providers, providerViewProto(v))
	}
	return connect.NewResponse(out), nil
}

func (p providerServer) GetProvider(ctx context.Context, req *connect.Request[v1.GetProviderRequest]) (*connect.Response[v1.GetProviderResponse], error) {
	v, err := p.svc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, providerError(err)
	}
	return connect.NewResponse(&v1.GetProviderResponse{Provider: providerViewProto(v)}), nil
}

func (p providerServer) SaveProviderSettings(ctx context.Context, req *connect.Request[v1.SaveProviderSettingsRequest]) (*connect.Response[v1.SaveProviderSettingsResponse], error) {
	v, err := p.svc.Save(ctx, req.Msg.GetId(), req.Msg.GetConfig(), req.Msg.GetSecrets())
	if err != nil {
		return nil, providerError(err)
	}
	return connect.NewResponse(&v1.SaveProviderSettingsResponse{Provider: providerViewProto(v)}), nil
}

func (p providerServer) ActivateProvider(ctx context.Context, req *connect.Request[v1.ActivateProviderRequest]) (*connect.Response[v1.ActivateProviderResponse], error) {
	v, err := p.svc.Activate(ctx, req.Msg.GetId())
	if err != nil {
		return nil, providerError(err)
	}
	return connect.NewResponse(&v1.ActivateProviderResponse{Provider: providerViewProto(v)}), nil
}

func (p providerServer) DeactivateProvider(ctx context.Context, req *connect.Request[v1.DeactivateProviderRequest]) (*connect.Response[v1.DeactivateProviderResponse], error) {
	v, err := p.svc.Deactivate(ctx, req.Msg.GetId())
	if err != nil {
		return nil, providerError(err)
	}
	return connect.NewResponse(&v1.DeactivateProviderResponse{Provider: providerViewProto(v)}), nil
}

func (p providerServer) ListProviderPairs(ctx context.Context, req *connect.Request[v1.ListProviderPairsRequest]) (*connect.Response[v1.ListProviderPairsResponse], error) {
	pairs, err := p.svc.Pairs(ctx, req.Msg.GetId())
	if err != nil {
		return nil, providerError(err)
	}
	out := &v1.ListProviderPairsResponse{Provider: req.Msg.GetId()}
	for _, pr := range pairs {
		out.Pairs = append(out.Pairs, &v1.ProviderPair{Base: pr.Base, Quote: pr.Quote})
	}
	return connect.NewResponse(out), nil
}

func providerViewProto(v providers.View) *v1.ProviderView {
	out := &v1.ProviderView{
		Id:                   v.ID,
		Label:                v.Label,
		Description:          v.Description,
		Activated:            v.Activated,
		Status:               v.Status,
		Error:                v.Error,
		Config:               v.Config,
		SecretKeysConfigured: v.SecretKeysConfigured,
		UpdatedAtMs:          v.UpdatedAtMs,
	}
	for _, f := range v.Fields {
		out.Fields = append(out.Fields, &v1.ProviderField{
			Key: f.Key, Label: f.Label, Kind: f.Kind, Description: f.Description,
			Placeholder: f.Placeholder, Default: f.Default,
		})
	}
	if v.Progress != nil {
		out.Progress = &v1.ProviderProgress{Label: v.Progress.Label}
		if v.Progress.Done != nil {
			done := int32(*v.Progress.Done)
			out.Progress.Done = &done
		}
		if v.Progress.Total != nil {
			total := int32(*v.Progress.Total)
			out.Progress.Total = &total
		}
	}
	return out
}
