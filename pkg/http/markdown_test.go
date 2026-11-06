package http

import (
	"strings"
	"testing"
)

func TestRenderMarkdownNoXSS(t *testing.T) {
	src := "# Title\n\nhello <script>alert(1)</script> <img src=x onerror=alert(2)>\n\n" +
		"<div onclick=\"alert(3)\">block</div>\n\n[x](javascript:alert(4))\n\n**bold** [ok](https://example.com)\n"
	out := strings.ToLower(string(RenderMarkdown([]byte(src))))
	for _, bad := range []string{"<script", "onerror=", "onclick=", "javascript:"} {
		if strings.Contains(out, bad) {
			t.Errorf("rendered output contains %q:\n%s", bad, out)
		}
	}
	for _, good := range []string{"<h1", "<strong>bold</strong>", `href="https://example.com"`} {
		if !strings.Contains(out, good) {
			t.Errorf("rendered output lost %q:\n%s", good, out)
		}
	}
}
