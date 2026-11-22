package main

import (
	"strings"
	"testing"
)

func TestPublicPagesUseLocalFontsAndPinnedChart(t *testing.T) {
	for name, page := range map[string]string{"parent": ortuPortalHTML, "exam": ujianOnlineHTML} {
		if !strings.Contains(page, `href="/fonts.css?v=1"`) {
			t.Fatalf("%s must load the local font stylesheet", name)
		}
		for _, host := range []string{"fonts.googleapis.com", "fonts.gstatic.com", "cdn.jsdelivr.net"} {
			if strings.Contains(page, host) {
				t.Fatalf("%s loads an unpinned third-party asset from %s", name, host)
			}
		}
	}
	if !strings.Contains(ortuPortalHTML, `src="/vendor/chart.js-4.5.1/chart.umd.min.js" integrity="sha384-`) {
		t.Fatal("parent portal must use the bundled, integrity-checked chart script")
	}
}

func TestProductionCSPRejectsExternalFontAndChartHosts(t *testing.T) {
	policy := productionContentSecurityPolicy("test-nonce")
	for _, host := range []string{"fonts.googleapis.com", "fonts.gstatic.com", "cdn.jsdelivr.net"} {
		if strings.Contains(policy, host) {
			t.Fatalf("CSP still permits removed asset host %s", host)
		}
	}
	if !strings.Contains(policy, "font-src 'self';") || !strings.Contains(policy, "https://challenges.cloudflare.com") {
		t.Fatal("CSP must restrict fonts to self while preserving the anti-bot integration")
	}
}
