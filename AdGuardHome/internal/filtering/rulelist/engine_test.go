package rulelist_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/AdguardTeam/AdGuardHome/internal/filtering/rulelist"
	"github.com/AdguardTeam/golibs/testutil"
	"github.com/AdguardTeam/urlfilter"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_Refresh(t *testing.T) {
	cacheDir := t.TempDir()

	fileURL, srvURL := newFilterLocations(t, cacheDir, testRuleTextBlocked, testRuleTextBlocked2)

	fileFlt := newFilter(t, fileURL, "File Filter")
	httpFlt := newFilter(t, srvURL, "HTTP Filter")

	eng := rulelist.NewEngine(&rulelist.EngineConfig{
		Name:    "Engine",
		Filters: []*rulelist.Filter{fileFlt, httpFlt},
	})
	require.NotNil(t, eng)
	testutil.CleanupAndRequireSuccess(t, eng.Close)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)

	buf := make([]byte, rulelist.DefaultRuleBufSize)
	cli := &http.Client{
		Timeout: testTimeout,
	}

	err := eng.Refresh(ctx, buf, cli, cacheDir, rulelist.DefaultMaxRuleListSize)
	require.NoError(t, err)

	fltReq := &urlfilter.DNSRequest{
		Hostname: "blocked.example",
		Answer:   false,
		DNSType:  dns.TypeA,
	}

	fltRes, hasMatched := eng.FilterRequest(fltReq)
	assert.True(t, hasMatched)

	require.NotNil(t, fltRes)

	fltReq = &urlfilter.DNSRequest{
		Hostname: "blocked-2.example",
		Answer:   false,
		DNSType:  dns.TypeA,
	}

	fltRes, hasMatched = eng.FilterRequest(fltReq)
	assert.True(t, hasMatched)

	require.NotNil(t, fltRes)
}
