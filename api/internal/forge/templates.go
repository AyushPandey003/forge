package forge

import (
	"fmt"
	"html"
	"strings"
)

// Starter documents for artifact slots. They are Markdown: Forge's editor
// stores Markdown as the source of truth and renders it to self-contained
// HTML for each version (see RenderMarkdownDoc).
var templates = map[string]string{
	"problem_statement": `# Problem statement

## Who is affected
Describe the users or teams who feel this problem.

## What is the problem
State the problem in one or two sentences. Avoid describing a solution.

## Evidence
- Data, metrics, support tickets
- Quotes from user interviews
- Linked incidents (IRCA)

## Impact
What happens if we do nothing? Quantify where possible.

## Out of scope
What this problem statement deliberately does not cover.
`,
	"prd": `# PRD

## Summary
One paragraph: the problem (link the approved problem statement) and the proposed solution.

## Goals and non-goals
**Goals**
- …

**Non-goals**
- …

## User stories

### Story 1: _Title_
As a _user_, I want _capability_ so that _outcome_.

**Acceptance criteria**
- [ ] Given _context_, when _action_, then _result_
- [ ] Another criterion

### Story 2: _Title_
As a _user_, I want _capability_ so that _outcome_.

**Acceptance criteria**
- [ ] Another criterion

## UX
Link designs (generate them with Forge's design studio) and prototypes.

## Success metrics
| Metric | Baseline | Target |
|---|---|---|
|  |  |  |

## Open questions
- …
`,
	"impl_doc": `# Implementation doc

## High-level design (HLD)
Components, responsibilities and how they interact.

` + "```mermaid\nflowchart LR\n  Client --> API --> DB[(Database)]\n```" + `

## Low-level design (LLD)
### Data model
### APIs
### Key algorithms / flows

## Non-functional requirements
| Area | Requirement |
|---|---|
| Performance |  |
| Scalability |  |
| Security & privacy |  |
| Reliability |  |
| Observability |  |
| Accessibility |  |

## Rollout plan
Feature flags, migration steps, rollback.

## Risks
- …
`,
	"user_interview": `# User interview

**Participant:**  
**Role / segment:**  
**Date:**  
**Interviewer:**  

## Goals of this interview
- …

## Notes

## Key quotes
> “…”

## Insights
- …
`,
	"interview_plan": `# User interview plan

## What we want to learn
- …

## Participants
| Name | Segment | Date | Status |
|---|---|---|---|
|  |  |  | To schedule |

## Script
1. …
`,
	"incident": `# Incident / IRCA

**Incident ID:**  
**Date:**  
**Severity:**  

## Summary

## Root cause

## Customer impact

## How it relates to this problem
`,
	"learnings": `# Learnings

## What went well
- …

## What was harder than expected
- …

## What we would do differently
- …
`,
	"release_notes": `# Release notes

## What's new
- …

## Improvements
- …

## Fixes
- …

## Known issues
- …
`,
	"launch_notes": `# Launch notes

## Audience

## Key message

## Channels and dates
| Channel | Owner | Date |
|---|---|---|
|  |  |  |

## FAQ
`,
	"csat": `# CSAT

## Survey
**How satisfied are you with _feature_?** (1–5)

**What is one thing we should improve?**

## Results
| Period | Responses | Avg score | CSAT % |
|---|---|---|---|
|  |  |  |  |

## Themes
- …
`,
}

// Template returns the starter Markdown for a template key ("" if none).
func Template(key, solutionTitle string) string {
	t, ok := templates[key]
	if !ok {
		return "# " + solutionTitle + "\n\n"
	}
	return t
}

// RenderMarkdownDoc wraps rendered Markdown HTML in a self-contained document
// styled with Katalyst tokens, and embeds the Markdown source so the editor
// can round-trip it.
func RenderMarkdownDoc(title, bodyHTML, markdown string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s</title><meta name="forge:kind" content="markdown">
<link rel="preconnect" href="https://fonts.googleapis.com"><link href="https://fonts.googleapis.com/css2?family=Figtree:wght@400;500;600;700&display=swap" rel="stylesheet">
<style>%s</style></head><body><main class="doc">%s</main>
<script type="text/markdown" id="forge-source">%s</script>
<script>%s</script></body></html>`,
		html.EscapeString(title), docCSS, bodyHTML, strings.ReplaceAll(markdown, "</script", "<\\/script"), mermaidBoot)
	return []byte(b.String())
}

// Katalyst-flavoured document typography (monochrome, hairline borders).
const docCSS = `
:root{--background:oklch(1 0 0);--foreground:oklch(.145 0 0);--muted:oklch(.97 0 0);--muted-foreground:oklch(.556 0 0);--border:oklch(.922 0 0);--radius:.45rem}
@media (prefers-color-scheme:dark){:root{--background:oklch(.145 0 0);--foreground:oklch(.985 0 0);--muted:oklch(.269 0 0);--muted-foreground:oklch(.715 0 0);--border:oklch(1 0 0/10%)}}
*{box-sizing:border-box}html{background:var(--background);color:var(--foreground)}
body{margin:0;font:15px/1.65 Figtree,ui-sans-serif,system-ui,sans-serif;-webkit-font-smoothing:antialiased}
.doc{max-width:760px;margin:0 auto;padding:56px 24px 96px}
h1{font-size:30px;line-height:1.2;font-weight:600;letter-spacing:-.02em;margin:0 0 24px}
h2{font-size:20px;font-weight:600;margin:40px 0 12px;padding-top:8px}
h3{font-size:16px;font-weight:600;margin:28px 0 8px}
p,ul,ol,table,blockquote,pre{margin:0 0 14px}
a{color:inherit;text-underline-offset:3px}
code{font:13px ui-monospace,SFMono-Regular,Menlo,monospace;background:var(--muted);padding:2px 5px;border-radius:calc(var(--radius)*.6)}
pre{background:var(--muted);padding:14px;border-radius:var(--radius);overflow:auto}pre code{background:none;padding:0}
blockquote{border-left:2px solid var(--border);padding-left:14px;color:var(--muted-foreground)}
table{border-collapse:collapse;width:100%;font-size:14px}th,td{border:1px solid var(--border);padding:8px 10px;text-align:left;vertical-align:top}th{background:var(--muted);font-weight:500}
hr{border:0;border-top:1px solid var(--border);margin:32px 0}
img{max-width:100%;border-radius:var(--radius);border:1px solid var(--border)}
li>input[type=checkbox]{margin-right:6px}ul.contains-task-list{list-style:none;padding-left:4px}
.mermaid{background:var(--muted);border-radius:var(--radius);padding:12px;text-align:center}
`

// Lazily load Mermaid only when a diagram is present.
const mermaidBoot = `(function(){var n=document.querySelectorAll('pre code.language-mermaid');if(!n.length)return;n.forEach(function(c){var d=document.createElement('div');d.className='mermaid';d.textContent=c.textContent;c.parentNode.replaceWith(d)});var s=document.createElement('script');s.src='https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js';s.onload=function(){mermaid.initialize({startOnLoad:false,theme:matchMedia('(prefers-color-scheme: dark)').matches?'dark':'neutral'});mermaid.run()};document.head.appendChild(s)})();`

// ExtractMarkdownSource returns the embedded Markdown of a Forge doc, if any.
func ExtractMarkdownSource(doc []byte) (string, bool) {
	s := string(doc)
	const open = `<script type="text/markdown" id="forge-source">`
	i := strings.Index(s, open)
	if i < 0 {
		return "", false
	}
	rest := s[i+len(open):]
	j := strings.Index(rest, "</script>")
	if j < 0 {
		return "", false
	}
	return strings.ReplaceAll(rest[:j], "<\\/script", "</script"), true
}
