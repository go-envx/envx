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

// numeric matches a password or list ID.
var numeric = regexp.MustCompile(`^\d+$`)

// Options locates the Passwordstate API and its credentials.
type Options struct {
	// URL is the HTTPS base URL of the Passwordstate site.
	URL string
	// APIKeyEnv names the host variable holding the API key.
	APIKeyEnv string
	// ListIDs maps an environment to the list of its short references.
	ListIDs map[string]int
	// Field is read when a reference names none; empty means "Password".
	Field string
	// Client sends API requests; nil uses a client that refuses redirects.
	Client external.HTTPClient
}

// Provider fetches passwordstate://<password-id>, <list-id>/<title>, or
// <title> references with an optional #<field>. It caches list listings for
// its lifetime, which is one operation.
type Provider struct {
	// client sends API requests.
	client external.HTTPClient
	// baseURL is the validated site URL.
	baseURL string
	// apiKey authenticates requests.
	apiKey string
	// listIDs maps an environment to the list of its short references.
	listIDs map[string]int
	// field is read when a reference names none.
	field string
	// listings caches each listed list's records by list ID.
	listings map[string][]record
	// configErr reports invalid configuration or missing credentials on fetch.
	configErr error
}

// record is one password record keyed by API field name.
type record map[string]json.RawMessage

// reference is one parsed qualified reference.
type reference struct {
	// id is the password ID; empty for a title reference.
	id string
	// list is the list searched by title; empty for an ID reference.
	list string
	// title is the decoded title; empty for an ID reference.
	title string
	// field is the record field to read.
	field string
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
	p := &Provider{
		client:   client,
		baseURL:  opts.URL,
		listIDs:  opts.ListIDs,
		field:    opts.Field,
		listings: make(map[string][]record),
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

// Qualify accepts a password ID, "<list-id>/<title>", or a short "<title>" in
// the environment's configured list, each with an optional "#<field>".
func (p *Provider) Qualify(
	ref external.Reference, environment string,
) (external.Reference, error) {
	if !strings.Contains(ref.Path, "/") && !numeric.MatchString(ref.Path) {
		list, ok := p.listIDs[environment]
		if !ok {
			return external.Reference{}, fmt.Errorf(
				"%w: no passwordstate list-id configured for environment %q",
				external.ErrInvalidReference, environment,
			)
		}
		ref.Path = strconv.Itoa(list) + "/" + ref.Path
	}
	if _, err := p.parse(ref); err != nil {
		return external.Reference{}, err
	}
	return ref, nil
}

// CheckAvailable reports invalid configuration or a missing API key.
func (p *Provider) CheckAvailable() error {
	return p.configErr
}

// Fetch reads one qualified reference: a password ID or a title within a list.
// A title is percent-decoded once, so "%2F" stands for a slash.
func (p *Provider) Fetch(ctx context.Context, ref external.Reference) (string, error) {
	target, err := p.parse(ref)
	if err != nil {
		return "", err
	}
	if p.configErr != nil {
		return "", p.configErr
	}
	if target.id != "" {
		found, err := p.password(ctx, target.id)
		if err != nil {
			return "", err
		}
		return found.text(target.field, "password "+target.id)
	}
	found, err := p.passwordByTitle(ctx, target.list, target.title)
	if err != nil {
		return "", err
	}
	return found.text(target.field, fmt.Sprintf("password %q", target.title))
}

// parse splits a qualified reference into a password ID, or a list and decoded
// title, applying the default field.
func (p *Provider) parse(ref external.Reference) (reference, error) {
	field := ref.Field
	if field == "" {
		field = p.field
	}
	list, rawTitle, byTitle := strings.Cut(ref.Path, "/")
	if !byTitle && numeric.MatchString(ref.Path) {
		return reference{id: ref.Path, field: field}, nil
	}
	if !byTitle || !numeric.MatchString(list) {
		return reference{}, fmt.Errorf(
			"%w: want passwordstate://<password-id>, "+
				"passwordstate://<list-id>/<title>, or passwordstate://<title>",
			external.ErrInvalidReference,
		)
	}
	title, err := url.PathUnescape(rawTitle)
	if err != nil {
		return reference{}, fmt.Errorf(
			"%w: invalid percent-encoding; write a literal %% as %%25",
			external.ErrInvalidReference,
		)
	}
	return reference{list: list, title: title, field: field}, nil
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

// passwordByTitle selects the single record titled title in list, listing each
// list once. Titles match exactly.
func (p *Provider) passwordByTitle(
	ctx context.Context, list, title string,
) (record, error) {
	listing, ok := p.listings[list]
	if !ok {
		endpoint, err := url.JoinPath(p.baseURL, "api", "passwords", list)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", external.ErrProviderUnavailable, err)
		}
		listing, err = p.get(ctx, endpoint+"?QueryAll", "password list "+list)
		if err != nil {
			return nil, err
		}
		p.listings[list] = listing
	}
	var matches []record
	for _, r := range listing {
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
