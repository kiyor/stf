package http

import (
	"bytes"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// MarkdownRenderer handles rendering of markdown files to HTML
type MarkdownRenderer struct{}

// markdownTemplate is the HTML template for rendering markdown
const markdownTemplate = `<!DOCTYPE html>
<html lang="en" data-theme="{{.Theme}}">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        :root {
            --bg-color: #ffffff;
            --text-color: #24292e;
            --link-color: #0366d6;
            --code-bg: #f6f8fa;
            --border-color: #e1e4e8;
            --blockquote-color: #6a737d;
            --table-border: #dfe2e5;
            --table-alt-bg: #f6f8fa;
            --hr-color: #e1e4e8;
        }

        [data-theme="dark"] {
            --bg-color: #0d1117;
            --text-color: #c9d1d9;
            --link-color: #58a6ff;
            --code-bg: #161b22;
            --border-color: #30363d;
            --blockquote-color: #8b949e;
            --table-border: #30363d;
            --table-alt-bg: #161b22;
            --hr-color: #21262d;
        }

        * {
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Noto Sans', Helvetica, Arial, sans-serif;
            font-size: 16px;
            line-height: 1.6;
            color: var(--text-color);
            background-color: var(--bg-color);
            margin: 0;
            padding: 0;
        }

        .container {
            max-width: 900px;
            margin: 0 auto;
            padding: 32px;
        }

        .theme-toggle {
            position: fixed;
            top: 16px;
            right: 16px;
            background: var(--code-bg);
            border: 1px solid var(--border-color);
            border-radius: 6px;
            padding: 8px 12px;
            cursor: pointer;
            color: var(--text-color);
            font-size: 14px;
            z-index: 1000;
        }

        .theme-toggle:hover {
            background: var(--border-color);
        }

        h1, h2, h3, h4, h5, h6 {
            margin-top: 24px;
            margin-bottom: 16px;
            font-weight: 600;
            line-height: 1.25;
        }

        h1 { font-size: 2em; padding-bottom: 0.3em; border-bottom: 1px solid var(--border-color); }
        h2 { font-size: 1.5em; padding-bottom: 0.3em; border-bottom: 1px solid var(--border-color); }
        h3 { font-size: 1.25em; }
        h4 { font-size: 1em; }
        h5 { font-size: 0.875em; }
        h6 { font-size: 0.85em; color: var(--blockquote-color); }

        p {
            margin-top: 0;
            margin-bottom: 16px;
        }

        a {
            color: var(--link-color);
            text-decoration: none;
        }

        a:hover {
            text-decoration: underline;
        }

        code {
            font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
            font-size: 85%;
            background-color: var(--code-bg);
            border-radius: 6px;
            padding: 0.2em 0.4em;
        }

        pre {
            font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
            font-size: 85%;
            background-color: var(--code-bg);
            border-radius: 6px;
            padding: 16px;
            overflow: auto;
            line-height: 1.45;
        }

        pre code {
            background-color: transparent;
            padding: 0;
            border-radius: 0;
        }

        blockquote {
            margin: 0 0 16px 0;
            padding: 0 1em;
            color: var(--blockquote-color);
            border-left: 0.25em solid var(--border-color);
        }

        ul, ol {
            margin-top: 0;
            margin-bottom: 16px;
            padding-left: 2em;
        }

        li + li {
            margin-top: 0.25em;
        }

        table {
            border-spacing: 0;
            border-collapse: collapse;
            margin-top: 0;
            margin-bottom: 16px;
            width: 100%;
            overflow: auto;
        }

        table th, table td {
            padding: 6px 13px;
            border: 1px solid var(--table-border);
        }

        table th {
            font-weight: 600;
        }

        table tr {
            background-color: var(--bg-color);
        }

        table tr:nth-child(2n) {
            background-color: var(--table-alt-bg);
        }

        hr {
            height: 0.25em;
            padding: 0;
            margin: 24px 0;
            background-color: var(--hr-color);
            border: 0;
        }

        img {
            max-width: 100%;
            box-sizing: content-box;
        }

        .task-list-item {
            list-style-type: none;
        }

        .task-list-item input {
            margin: 0 0.2em 0.25em -1.4em;
            vertical-align: middle;
        }

        .raw-link {
            position: fixed;
            top: 16px;
            right: 100px;
            background: var(--code-bg);
            border: 1px solid var(--border-color);
            border-radius: 6px;
            padding: 8px 12px;
            color: var(--text-color);
            font-size: 14px;
            text-decoration: none;
            z-index: 1000;
        }

        .raw-link:hover {
            background: var(--border-color);
            text-decoration: none;
        }
    </style>
</head>
<body>
    <a href="?raw=1" class="raw-link">Raw</a>
    <button class="theme-toggle" onclick="toggleTheme()">
        <span id="theme-icon">{{if eq .Theme "dark"}}☀️ Light{{else}}🌙 Dark{{end}}</span>
    </button>
    <div class="container">
        {{.Content}}
    </div>
    <script>
        function getPreferredTheme() {
            const saved = localStorage.getItem('md-theme');
            if (saved) return saved;
            return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
        }

        function setTheme(theme) {
            document.documentElement.setAttribute('data-theme', theme);
            localStorage.setItem('md-theme', theme);
            const icon = document.getElementById('theme-icon');
            icon.textContent = theme === 'dark' ? '☀️ Light' : '🌙 Dark';
        }

        function toggleTheme() {
            const current = document.documentElement.getAttribute('data-theme');
            setTheme(current === 'dark' ? 'light' : 'dark');
        }

        // Apply saved theme on load
        const urlParams = new URLSearchParams(window.location.search);
        const themeParam = urlParams.get('theme');
        if (themeParam === 'dark' || themeParam === 'light') {
            setTheme(themeParam);
        } else {
            setTheme(getPreferredTheme());
        }
    </script>
</body>
</html>`

type markdownData struct {
	Title   string
	Content template.HTML
	Theme   string
}

// IsMarkdownFile checks if the file is a markdown file
func IsMarkdownFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".md" || ext == ".markdown"
}

// RenderMarkdown converts markdown content to HTML
func RenderMarkdown(content []byte) []byte {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.Footnotes | parser.Strikethrough | parser.Tables | parser.FencedCode | parser.Autolink
	p := parser.NewWithExtensions(extensions)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return markdown.ToHTML(content, p, renderer)
}

// ServeMarkdown serves a markdown file as rendered HTML
func ServeMarkdown(w http.ResponseWriter, r *http.Request, f File, name string, modtime time.Time) {
	// Check if raw content is requested
	if r.URL.Query().Get("raw") == "1" {
		// Serve raw markdown content
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		setLastModified(w, modtime)
		io.Copy(w, f)
		return
	}

	// Read the entire file
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, f); err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	// Convert markdown to HTML
	htmlContent := RenderMarkdown(buf.Bytes())

	// Determine theme from query parameter or default
	theme := r.URL.Query().Get("theme")
	if theme != "dark" && theme != "light" {
		theme = "light" // default theme, will be overridden by JS
	}

	// Extract title from filename
	title := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))

	// Parse and execute template
	tmpl, err := template.New("markdown").Parse(markdownTemplate)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	data := markdownData{
		Title:   title,
		Content: template.HTML(htmlContent),
		Theme:   theme,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	setLastModified(w, modtime)

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template execution error", http.StatusInternalServerError)
		return
	}
}
