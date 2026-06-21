package qmclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// API is a typed client for the Quartermaster control and data planes.
type API struct {
	client *ClientWithResponses
}

// Raw returns the underlying generated client.
func (a *API) Raw() *ClientWithResponses {
	return a.client
}

func (a *API) Health(ctx context.Context) (*HealthResponse, error) {
	resp, err := a.client.HealthzWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		return resp.JSON200, nil
	default:
		return nil, fmt.Errorf("quartermaster: health check returned %d", resp.StatusCode())
	}
}

func (a *API) OpenIDConfiguration(ctx context.Context) (*DiscoveryDocument, error) {
	resp, err := a.client.OpenidConfigurationWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("quartermaster: openid configuration returned %d", resp.StatusCode())
	}
	return resp.JSON200, nil
}

func (a *API) CAChain(ctx context.Context) (string, error) {
	resp, err := a.client.CaChainWithResponse(ctx)
	if err != nil {
		return "", err
	}
	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("quartermaster: ca chain returned %d", resp.StatusCode())
	}
	return string(resp.Body), nil
}

func (a *API) JWKS(ctx context.Context) (json.RawMessage, error) {
	resp, err := a.client.JwksWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("quartermaster: jwks returned %d", resp.StatusCode())
	}
	return json.RawMessage(resp.Body), nil
}

func (a *API) ListBillets(ctx context.Context) ([]BilletListItem, error) {
	resp, err := a.client.ListBilletsWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		if resp.JSON200 == nil {
			return nil, nil
		}
		return *resp.JSON200, nil
	case 401, 403:
		return nil, errorFromResponse(resp.StatusCode(), firstErrorBody(resp.JSON401, resp.JSON403))
	default:
		return nil, fmt.Errorf("quartermaster: list billets returned %d", resp.StatusCode())
	}
}

func (a *API) GetAdminBillet(ctx context.Context, name string) (*BilletWithPolicies, error) {
	resp, err := a.client.GetAdminBilletWithResponse(ctx, name)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		return resp.JSON200, nil
	case 401, 403, 404:
		return nil, errorFromResponse(resp.StatusCode(), firstErrorBody(resp.JSON401, resp.JSON403, resp.JSON404))
	default:
		return nil, fmt.Errorf("quartermaster: get billet returned %d", resp.StatusCode())
	}
}

func (a *API) CreateBillet(ctx context.Context, req CreateBilletRequest) (*AdminBilletMetadataResponse, error) {
	resp, err := a.client.CreateBilletWithResponse(ctx, req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 201:
		return resp.JSON201, nil
	case 401, 403:
		return nil, errorFromResponse(resp.StatusCode(), firstErrorBody(resp.JSON401, resp.JSON403))
	default:
		return nil, fmt.Errorf("quartermaster: create billet returned %d", resp.StatusCode())
	}
}

func (a *API) UpdateBillet(ctx context.Context, name string, req UpdateBilletRequest) (*AdminBilletMetadataResponse, error) {
	resp, err := a.client.UpdateBilletWithResponse(ctx, name, req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		return resp.JSON200, nil
	case 401, 403, 404:
		return nil, errorFromResponse(resp.StatusCode(), firstErrorBody(resp.JSON401, resp.JSON403, resp.JSON404))
	default:
		return nil, fmt.Errorf("quartermaster: update billet returned %d", resp.StatusCode())
	}
}

func (a *API) DeleteBillet(ctx context.Context, name string) error {
	resp, err := a.client.DeleteBilletWithResponse(ctx, name)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 204:
		return nil
	case 401, 403, 404:
		return errorFromResponse(resp.StatusCode(), firstErrorBody(resp.JSON401, resp.JSON403, resp.JSON404))
	default:
		return fmt.Errorf("quartermaster: delete billet returned %d", resp.StatusCode())
	}
}

func (a *API) ListPolicies(ctx context.Context, billetName string) ([]PolicyResponse, error) {
	resp, err := a.client.ListPoliciesWithResponse(ctx, billetName)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		if resp.JSON200 == nil {
			return nil, nil
		}
		return *resp.JSON200, nil
	case 401, 403:
		return nil, errorFromResponse(resp.StatusCode(), firstErrorBody(resp.JSON401, resp.JSON403))
	default:
		return nil, fmt.Errorf("quartermaster: list policies returned %d", resp.StatusCode())
	}
}

func (a *API) GetPolicy(ctx context.Context, billetName, id string) (*PolicyResponse, error) {
	resp, err := a.client.GetPolicyWithResponse(ctx, billetName, id)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		return resp.JSON200, nil
	case 401, 403, 404:
		return nil, errorFromResponse(resp.StatusCode(), firstErrorBody(resp.JSON401, resp.JSON403, resp.JSON404))
	default:
		return nil, fmt.Errorf("quartermaster: get policy returned %d", resp.StatusCode())
	}
}

func (a *API) CreatePolicy(ctx context.Context, billetName string, req CreatePolicyRequest) (*PolicyResponse, error) {
	resp, err := a.client.CreatePolicyWithResponse(ctx, billetName, req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 201:
		return resp.JSON201, nil
	case 401, 403, 404:
		return nil, errorFromResponse(resp.StatusCode(), firstErrorBody(resp.JSON401, resp.JSON403, resp.JSON404))
	default:
		return nil, fmt.Errorf("quartermaster: create policy returned %d", resp.StatusCode())
	}
}

func (a *API) UpdatePolicy(ctx context.Context, billetName, id string, req UpdatePolicyRequest) (*PolicyResponse, error) {
	resp, err := a.client.UpdatePolicyWithResponse(ctx, billetName, id, req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		return resp.JSON200, nil
	case 401, 403, 404:
		return nil, errorFromResponse(resp.StatusCode(), firstErrorBody(resp.JSON401, resp.JSON403, resp.JSON404))
	default:
		return nil, fmt.Errorf("quartermaster: update policy returned %d", resp.StatusCode())
	}
}

func (a *API) DeletePolicy(ctx context.Context, billetName, id string) error {
	resp, err := a.client.DeletePolicyWithResponse(ctx, billetName, id)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 204:
		return nil
	case 401, 403, 404:
		return errorFromResponse(resp.StatusCode(), firstErrorBody(resp.JSON401, resp.JSON403, resp.JSON404))
	default:
		return fmt.Errorf("quartermaster: delete policy returned %d", resp.StatusCode())
	}
}

func (a *API) GetBilletMetadata(ctx context.Context, name string) (*BilletMetadataResponse, error) {
	resp, err := a.client.GetBilletMetadataWithResponse(ctx, name)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		return resp.JSON200, nil
	case 401, 403, 404:
		return nil, errorFromResponse(resp.StatusCode(), firstErrorBody(resp.JSON401, resp.JSON403, resp.JSON404))
	default:
		return nil, fmt.Errorf("quartermaster: get billet metadata returned %d", resp.StatusCode())
	}
}

func (a *API) DiscoverBillets(ctx context.Context, form BilletDiscoveryForm) (*BilletDiscoveryResponse, error) {
	resp, err := a.client.BilletDiscoveryWithFormdataBodyWithResponse(ctx, form)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		return resp.JSON200, nil
	case 400, 401:
		return nil, errorFromResponse(resp.StatusCode(), firstErrorBody(resp.JSON400, resp.JSON401))
	default:
		return nil, fmt.Errorf("quartermaster: billet discovery returned %d", resp.StatusCode())
	}
}

func (a *API) ExchangeToken(ctx context.Context, form TokenExchangeForm) (*TokenExchangeResponse, error) {
	resp, err := a.client.TokenExchangeWithFormdataBodyWithResponse(ctx, form)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		return resp.JSON200, nil
	case 400, 401:
		return nil, errorFromResponse(resp.StatusCode(), firstErrorBody(resp.JSON400, resp.JSON401))
	default:
		return nil, fmt.Errorf("quartermaster: token exchange returned %d", resp.StatusCode())
	}
}

// MarshalJSON formats a value for logging or debugging.
func MarshalJSON(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// MustReadBody reads and closes an io.ReadCloser, used by tests.
func MustReadBody(rc io.ReadCloser) ([]byte, error) {
	defer rc.Close()
	return io.ReadAll(rc)
}
