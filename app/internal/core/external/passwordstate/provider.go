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
	"strconv"
	"strings"

	"github.com/go-envx/envx/app/internal/core/external"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
)

// defaultField is the password record field read when a reference names none.
const defaultField = "Password"

// maxResponseBytes bounds the response body read from the API.
const maxResponseBytes = 1 << 20

// passwordID matches a numeric password ID; any other first segment is a title.
var passwordID = regexp.MustCompile(`^\d+$`)

// Options locates the Passwordstate API and its credentials.
type Options struct {
	// URL is the HTTPS base URL of the Passwordstate site.
	URL string
	// APIKeyEnv names the host variable holding the API key.
	APIKeyEnv string
	// ListID is the password list searched by title; zero disables titles.
	ListID int
	// Field is read when a reference names none; empty means "Password".
	Field string
	// Client sends API requests; nil uses a client that refuses redirects.
	Client external.HTTPClient
}

// Provider fetches passwordstate://<password-id-or-title>[/<field>]
// references. It caches the list listing for its lifetime, which is one
// operation.
type Provider struct {
	// client sends API requests.
	client external.HTTPClient
	// baseURL is the validated site URL.
	baseURL string
	// apiKey authenticates requests.
	apiKey string
	// listID is the password list searched by title; zero disables titles.
	listID int
	// field is read when a reference names none.
	field string
	// listing caches the records of the list once listed.
	listing []record
	// configErr reports invalid configuration or missing credentials on fetch.
	configErr error
}

// record is one password record keyed by API field name.
type record map[string]json.RawMessage

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
	p := &Provider{
		client:  client,
		baseURL: opts.URL,
		listID:  opts.ListID,
		field:   opts.Field,
	}
	if p.field == "" {
		p.field = defaultField
	}
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

// Fetch reads one reference path: a password ID or a title within the
// configured list, optionally followed by a field name such as "UserName". A
// title is percent-decoded once, so "%2F" stands for a slash.
func (p *Provider) Fetch(ctx context.Context, path string) (string, error) {
	target, field, hasField := strings.Cut(path, "/")
	title, titleErr := url.PathUnescape(target)
	if titleErr != nil || title == "" ||
		(hasField && (field == "" || strings.Contains(field, "/"))) {
		return "", fmt.Errorf(
			"%w: want passwordstate://<password-id-or-title>[/<field>]",
			external.ErrInvalidReference,
		)
	}
	byID := passwordID.MatchString(target)
	if !byID && p.listID == 0 {
		return "", fmt.Errorf(
			"%w: title references require list-id", external.ErrInvalidReference,
		)
	}
	if !hasField {
		field = p.field
	}
	if p.configErr != nil {
		return "", p.configErr
	}
	if byID {
		found, err := p.password(ctx, target)
		if err != nil {
			return "", err
		}
		return found.text(field, "password "+target)
	}
	found, err := p.passwordByTitle(ctx, title)
	if err != nil {
		return "", err
	}
	return found.text(field, fmt.Sprintf("password %q", title))
}

// password requests one password record by ID.
func (p *Provider) password(ctx context.Context, id string) (record, error) {
	endpoint, err := url.JoinPath(p.baseURL, "api", "passwords", id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", external.ErrProviderUnavailable, err)
	}
	records, err := p.get(ctx, endpoint, "password "+id)
	if err != nil {
		return nil, err
	}
	if len(records) != 1 {
		return nil, fmt.Errorf(
			"%w: GET password %s: want 1 record, got %d",
			external.ErrReferenceUnresolved, id, len(records),
		)
	}
	return records[0], nil
}

// passwordByTitle selects the single record titled title in the configured
// list, listing the list once. Titles match exactly.
func (p *Provider) passwordByTitle(ctx context.Context, title string) (record, error) {
	list := strconv.Itoa(p.listID)
	if p.listing == nil {
		endpoint, err := url.JoinPath(p.baseURL, "api", "passwords", list)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", external.ErrProviderUnavailable, err)
		}
		listed, err := p.get(ctx, endpoint+"?QueryAll", "password list "+list)
		if err != nil {
			return nil, err
		}
		p.listing = listed
	}
	var matches []record
	for _, r := range p.listing {
		var got string
		if json.Unmarshal(r["Title"], &got) == nil && got == title {
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf(
			"%w: no password titled %q in list %s",
			external.ErrReferenceUnresolved, title, list,
		)
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, len(matches))
		for i, r := range matches {
			ids[i] = r.id()
		}
		return nil, fmt.Errorf(
			"%w: %d passwords titled %q in list %s; "+
				"reference one by ID, such as passwordstate://%s (IDs: %s)",
			external.ErrReferenceUnresolved, len(matches), title, list,
			ids[0], strings.Join(ids, ", "),
		)
	}
}

// get requests endpoint and decodes its record array; what names the target in
// errors. Errors never quote the body, which holds secret values.
func (p *Provider) get(ctx context.Context, endpoint, what string) ([]record, error) {
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
			"%w: GET %s: HTTP %d: %s",
			external.ErrReferenceUnresolved, what, resp.StatusCode, errorSummary(body),
		)
	}
	var records []record
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf(
			"%w: GET %s: unexpected response format",
			external.ErrReferenceUnresolved, what,
		)
	}
	return records, nil
}

// text returns the field matching name case-insensitively; what names the
// record in errors.
func (r record) text(name, what string) (string, error) {
	for key, raw := range r {
		if !strings.EqualFold(key, name) {
			continue
		}
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return "", fmt.Errorf(
				"%w: field %q of %s is not text",
				external.ErrReferenceUnresolved, name, what,
			)
		}
		return text, nil
	}
	return "", fmt.Errorf(
		"%w: %s has no field %q", external.ErrReferenceUnresolved, what, name,
	)
}

// id returns the record's password ID, or "?" when it has none.
func (r record) id() string {
	var id json.Number
	if err := json.Unmarshal(r["PasswordID"], &id); err != nil {
		return "?"
	}
	return id.String()
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
