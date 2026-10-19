package cmd

import (
	"github.com/spf13/pflag"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runconfig"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

// HTTP client flags. Like the other settings that mirror a hitspec.yaml key
// they default to "not set": the HITSPEC_* variable, hitspec.yaml and the
// built-in default fill in what the command line leaves out.
var (
	cacertFlag                 string
	certFlag                   string
	keyFlag                    string
	http1Flag                  bool
	cookiesFlag                bool
	maxResponseBytesFlag       int64
	redirectForwardHeadersFlag bool
)

func registerHTTPFlags(f *pflag.FlagSet) {
	f.StringVar(&cacertFlag, "cacert", "", "PEM CA bundle to trust in addition to the system roots (env: "+runconfig.EnvCACert+"; default: hitspec.yaml cacert)")
	f.StringVar(&certFlag, "cert", "", "PEM client certificate for mutual TLS; needs --key (env: "+runconfig.EnvCert+"; default: hitspec.yaml cert)")
	f.StringVar(&keyFlag, "key", "", "PEM private key for --cert (env: "+runconfig.EnvKey+"; default: hitspec.yaml key)")
	f.BoolVar(&http1Flag, "http1.1", false, "Do not negotiate HTTP/2 (env: "+runconfig.EnvHTTP1+"; default: hitspec.yaml http1, else false)")
	f.BoolVar(&cookiesFlag, "cookies", false, "Keep a cookie jar: Set-Cookie from one response is sent on later requests of the run (env: "+runconfig.EnvCookies+"; default: hitspec.yaml cookies, else false)")
	f.Int64Var(&maxResponseBytesFlag, "max-response-bytes", 0, "Largest response body to read, in bytes; -1 for no limit (env: "+runconfig.EnvMaxResponseBytes+"; default: hitspec.yaml maxResponseBytes, else 104857600)")
	f.BoolVar(&redirectForwardHeadersFlag, "redirect-forward-headers", false, "Keep your request headers (API keys, Authorization) on redirects to another origin (env: "+runconfig.EnvRedirectForwardHeaders+"; default: hitspec.yaml redirectForwardHeaders, else false)")
}

// applyHTTPFlags layers the HTTP client flags onto o. A bool flag overrides
// when it is true or was passed explicitly, so --cookies=false can beat
// cookies: true in hitspec.yaml.
func applyHTTPFlags(flags *pflag.FlagSet, o *runconfig.Overrides) {
	o.CACert, o.Cert, o.Key = cacertFlag, certFlag, keyFlag
	if http1Flag || flags.Changed("http1.1") {
		o.HTTP1 = &http1Flag
	}
	if cookiesFlag || flags.Changed("cookies") {
		o.Cookies = &cookiesFlag
	}
	if redirectForwardHeadersFlag || flags.Changed("redirect-forward-headers") {
		o.RedirectForwardHeaders = &redirectForwardHeadersFlag
	}
	if flags.Changed("max-response-bytes") {
		o.MaxResponseBytes = &maxResponseBytesFlag
	}
}

// checkClientOptions fails with a configuration error (exit 3) when the HTTP
// client options cannot be applied: a missing or invalid CA, certificate or key
// file, or a malformed proxy URL.
func checkClientOptions(opts []http.ClientOption) error {
	if _, err := http.NewClientE(opts...); err != nil {
		return newExitError(ExitConfigError, err.Error())
	}
	return nil
}
