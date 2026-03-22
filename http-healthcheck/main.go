package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	timeout  time.Duration
	method   string
	insecure bool
	expect   string
)

var rootCmd = &cobra.Command{
	Use:   "http-healthcheck [flags] <url> [url...]",
	Short: "Check HTTP endpoints and match response status codes",
	Long: `Runs HTTP requests against each URL and succeeds only when the response status
is one of the codes given by --expect (default 200).

Flags can appear before or after URLs. Use --timeout or -t (not -timeout; pflag parses
single-dash long words as shorthand letters).

Boolean --insecure / -k: omit the flag for normal TLS verification (default). Use -k
or --insecure alone to skip verification. -k true and -k false (two tokens) are
accepted the same as -k=true / -k=false. You can also use -k=false / --insecure=false.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.MinimumNArgs(1),
	RunE:          run,
}

func init() {
	// Use --timeout (or -t). A single -timeout is parsed as multiple shorthand flags by pflag.
	rootCmd.Flags().DurationVarP(&timeout, "timeout", "t", 10*time.Second, "per-request timeout")
	rootCmd.Flags().StringVarP(&method, "method", "m", http.MethodGet, "HTTP method (GET, HEAD, ...)")
	rootCmd.Flags().BoolVarP(&insecure, "insecure", "k", false, "skip TLS verification (default: verify; use -k alone to enable, omit flag or use --insecure=false to verify)")
	rootCmd.Flags().StringVarP(&expect, "expect", "e", "200", "comma-separated acceptable status codes (e.g. 200,204)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(_ *cobra.Command, args []string) error {
	// pflag bool uses NoOptDefVal "true" for -k, so "-k true|false" does not bind the
	// second word to -k; it becomes a positional arg. Normalize -k true / -k false.
	insecure, args := fixInsecureBoolShorthand(insecure, args)
	if len(args) == 0 {
		return fmt.Errorf("at least one URL is required (after handling -k true/false, no URLs remain)")
	}

	okCodes, err := parseExpect(expect)
	if err != nil {
		return fmt.Errorf("invalid --expect: %w", err)
	}

	client := newHTTPClient(timeout, insecure)
	ctx := context.Background()
	var hadFailure bool
	for _, raw := range args {
		raw = strings.TrimSpace(raw)
		if err := validateHTTPURL(raw); err != nil {
			return fmt.Errorf("invalid url %q: %w", raw, err)
		}
		code, err := checkURL(ctx, client, method, raw, okCodes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", raw, err)
			hadFailure = true
			continue
		}
		fmt.Printf("OK %s -> %d\n", raw, code)
	}
	if hadFailure {
		return fmt.Errorf("one or more checks failed")
	}
	return nil
}

// fixInsecureBoolShorthand maps `-k false` / `-k true` (two argv tokens) to the right bool.
// Without this, pflag sets -k to true via NoOptDefVal and leaves "true"/"false" as a URL arg.
func fixInsecureBoolShorthand(insecure bool, args []string) (bool, []string) {
	if !insecure || len(args) == 0 {
		return insecure, args
	}
	switch args[0] {
	case "false":
		return false, args[1:]
	case "true":
		return true, args[1:]
	default:
		return insecure, args
	}
}

func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "http", "https":
	default:
		if u.Scheme == "" {
			return fmt.Errorf("must start with http:// or https://")
		}
		return fmt.Errorf("unsupported scheme %q (use http or https)", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("missing host")
	}
	return nil
}

func parseExpect(s string) (map[int]struct{}, error) {
	parts := strings.Split(s, ",")
	out := make(map[int]struct{}, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("not an integer: %q", p)
		}
		if n < 100 || n > 599 {
			return nil, fmt.Errorf("invalid HTTP status: %d", n)
		}
		out[n] = struct{}{}
	}
	if len(out) == 0 {
		return nil, errors.New("no status codes in -expect")
	}
	return out, nil
}

func newHTTPClient(timeout time.Duration, insecure bool) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = &tls.Config{}
		}
		tr.TLSClientConfig.InsecureSkipVerify = true
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: tr,
	}
}

func checkURL(ctx context.Context, client *http.Client, method, url string, okCodes map[int]struct{}) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if _, ok := okCodes[resp.StatusCode]; !ok {
		return resp.StatusCode, fmt.Errorf("status %d not in expected set", resp.StatusCode)
	}
	return resp.StatusCode, nil
}
