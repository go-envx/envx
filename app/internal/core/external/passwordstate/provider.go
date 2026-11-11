// Package passwordstate resolves passwordstate:// references with the
// Passwordstate standard REST API.
package passwordstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-envx/envx/app/internal/core/external"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
)

// defaultField is the password record field read when a reference names none.
const defaultField = "Password"

// maxResponseBytes bounds the response body read from the API.
const maxResponseBytes = 1 << 20

// passwordID matches a numeric password ID.
var passwordID = regexp.MustCompile(`^\d+$`)

// Options locates the Passwordstate API and its credentials.
type Options struct {
	// URL is the HTTPS base URL of the Passwordstate site.
	URL string
	// APIKeyEnv names the host variable holding the API key.
	APIKeyEnv string
	// Client sends API requests; nil uses a client that refuses redirects.
	Client external.HTTPClient
}

// Provider fetches passwordstate://<password-id>[/<field>] references.
type Provider struct {
	// client sends API requests.
	client external.HTTPClient
	// baseURL is the validated site URL.
	baseURL string
	// apiKey authenticates requests.
	apiKey string
	// configErr reports invalid configuration or missing credentials on fetch.
	configErr error
}

// apiError is one entry of a Passwordstate error response.
type apiError struct {
	// Errors lists the reported problems.
	Errors []struct {
		// Message describes the problem.
		Message string `json:"message"`
	} `json:"errors"`
}

// New constructs a provider that reads its API key from host. Configuration
// problems surface on Fetch, so construction never fails.
func New(host hostenv.Snapshot, opts Options) *Provider {
	client := opts.Client
	if client == nil {
		// Refusing redirects keeps the APIKey header from following one.
		client = &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	p := &Provider{client: client, baseURL: opts.URL}
	parsed, err := url.Parse(opts.URL)
	switch {
	case opts.URL == "":
		p.configErr = errors.New("passwordstate url is not configured")
	case err != nil || parsed.Scheme != "https" || parsed.Host == "":
		p.configErr = fmt.Errorf("passwordstate url %q must be an https URL", opts.URL)
	case opts.APIKeyEnv == "":
		p.configErr = errors.New("passwordstate api-key-env is not configured")
	default:
		key, _ := host.Get(opts.APIKeyEnv)
		if key == "" {
			p.configErr = fmt.Errorf("%s is not set", opts.APIKeyEnv)
		}
		p.apiKey = key
	}
	if p.configErr != nil {
		p.configErr = fmt.Errorf("%w: %w", external.ErrProviderUnavailable, p.configErr)
	}
	return p
}

// Scheme returns "passwordstate".
func (p *Provider) Scheme() string {
	return external.SchemePasswordstate
}

// Fetch reads one reference path: a password ID, optionally followed by a
// field name such as "UserName".
func (p *Provider) Fetch(ctx context.Context, path string) (string, error) {
	id, field, hasField := strings.Cut(path, "/")
	if !passwordID.MatchString(id) || (hasField && (field == "" ||
		strings.Contains(field, "/"))) {
		return "", fmt.Errorf(
			"%w: want passwordstate://<password-id>[/<field>]",
			external.ErrInvalidReference,
		)
	}
	if !hasField {
		field = defaultField
	}
	if p.configErr != nil {
		return "", p.configErr
	}
	record, err := p.password(ctx, id)
	if err != nil {
		return "", err
	}
	for name, raw := range record {
		if !strings.EqualFold(name, field) {
			continue
		}
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return "", fmt.Errorf(
				"%w: field %q of password %s is not text",
				external.ErrReferenceUnresolved, field, id,
			)
		}
		return text, nil
	}
	return "", fmt.Errorf(
		"%w: password %s has no field %q", external.ErrReferenceUnresolved, id, field,
	)
}

// password requests one password record. Errors never quote the record, which
// holds secret values.
func (p *Provider) password(
	ctx context.Context, id string,
) (map[string]json.RawMessage, error) {
	endpoint, err := url.JoinPath(p.baseURL, "api", "passwords", id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", external.ErrProviderUnavailable, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", external.ErrProviderUnavailable, err)
	}
	req.Header.Set("APIKey", p.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: passwordstate request failed: %w", external.ErrProviderUnavailable, err,
		)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf(
			"%w: reading passwordstate response: %w", external.ErrReferenceUnresolved, err,
		)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"%w: GET password %s: HTTP %d: %s",
			external.ErrReferenceUnresolved, id, resp.StatusCode, errorSummary(body),
		)
	}
	var records []map[string]json.RawMessage
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf(
			"%w: GET password %s: unexpected response format",
			external.ErrReferenceUnresolved, id,
		)
	}
	if len(records) != 1 {
		return nil, fmt.Errorf(
			"%w: GET password %s: want 1 record, got %d",
			external.ErrReferenceUnresolved, id, len(records),
		)
	}
	return records[0], nil
}

// errorSummary returns the first message of an API error body, truncated. A
// body of any other shape is not quoted.
func errorSummary(body []byte) string {
	var errs []apiError
	if err := json.Unmarshal(body, &errs); err != nil ||
		len(errs) == 0 || len(errs[0].Errors) == 0 {
		return external.SummarizeStderr("")
	}
	return external.SummarizeStderr(errs[0].Errors[0].Message)
}
