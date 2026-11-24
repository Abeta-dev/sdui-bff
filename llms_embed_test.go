package sduibff_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	sduibff "github.com/abeta-dev/sdui-bff"
)

func TestEmbeddedLLMsDocsMatchDiskAndSpec(t *testing.T) {
	t.Parallel()

	t.Run("llms.txt matches disk and llmstxt.org format", func(t *testing.T) {
		if len(sduibff.LLMsTxt) == 0 {
			t.Fatal("LLMsTxt is empty")
		}
		diskBytes, err := os.ReadFile("llms.txt")
		if err != nil {
			t.Fatalf("read llms.txt from disk: %v", err)
		}
		if !bytes.Equal(sduibff.LLMsTxt, diskBytes) {
			t.Fatal("embedded LLMsTxt does not match repository root llms.txt")
		}

		content := string(sduibff.LLMsTxt)
		for _, want := range []string{
			"# Abeta SDUI Domain BFF",
			"\n> ",
			"https://raw.githubusercontent.com/Abeta-dev/sdui-bff/main/README.md",
			"https://raw.githubusercontent.com/Abeta-dev/sdui-bff/main/AGENTS.md",
			"https://raw.githubusercontent.com/Abeta-dev/sdui-bff/main/docs/architecture.md",
			"https://raw.githubusercontent.com/Abeta-dev/sdui-bff/main/docs/verticals-guide.md",
			"https://raw.githubusercontent.com/Abeta-dev/sdui-bff/main/llms-full.txt",
			"https://raw.githubusercontent.com/Abeta-dev/sdui-bff/main/skills/abeta-sdui-bff/SKILL.md",
			"https://raw.githubusercontent.com/Abeta-dev/sdui-bff/main/agents/bff-engineer.md",
			"https://raw.githubusercontent.com/Abeta-dev/sdui-bff/main/docker-compose.yaml",
			"https://raw.githubusercontent.com/Abeta-dev/kmp/main/llms.txt",
			"https://raw.githubusercontent.com/Abeta-dev/layout-engine/main/llms.txt",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("llms.txt missing expected substring %q", want)
			}
		}
	})

	t.Run("llms-full.txt matches disk and documents platform and verticals", func(t *testing.T) {
		if len(sduibff.LLMsFullTxt) == 0 {
			t.Fatal("LLMsFullTxt is empty")
		}
		if len(sduibff.LLMsFullTxt) <= len(sduibff.LLMsTxt) {
			t.Fatalf("expected LLMsFullTxt (%d bytes) to be larger than LLMsTxt (%d bytes)", len(sduibff.LLMsFullTxt), len(sduibff.LLMsTxt))
		}
		diskBytes, err := os.ReadFile("llms-full.txt")
		if err != nil {
			t.Fatalf("read llms-full.txt from disk: %v", err)
		}
		if !bytes.Equal(sduibff.LLMsFullTxt, diskBytes) {
			t.Fatal("embedded LLMsFullTxt does not match repository root llms-full.txt")
		}

		content := string(sduibff.LLMsFullTxt)
		for _, want := range []string{
			"Abeta-dev/kmp",
			"Abeta-dev/layout-engine",
			"Abeta-dev/sdui-bff",
			"LAYOUT_ENGINE_URL",
			"X-SDUI-Source",
			"TrackingContract.properties",
			"Abeta Fresh",
			"Abeta Bites",
			"Abeta Pulse",
			"Abeta Mart",
			"Abeta Spark",
			"Abeta Trade",
			"Abeta Pay",
			"v0.4.0",
			"github.com/abeta-dev/kmp/bff@v0.6.0",
			"29 Wire Atoms",
			"31 Compose UI Atoms",
			"36 ComposedNodeKinds",
			"POST /api/v1/analytics/events",
			"GET /api/v1/analytics/summary",
			"EnsureComponentIDs",
			"CI Quality Gate",
			"--bff-url=http://localhost:8085",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("llms-full.txt missing expected substring %q", want)
			}
		}
	})
}
