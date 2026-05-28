# Frontend Rules (PART 16, 17)

Read: AI.md PART 16, 17

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Client-side rendering (React, Vue, Angular, etc.)
- Require JavaScript for core functionality
- Client-side routing (SPA)
- Business logic in JavaScript
- Let long strings break mobile layout
- Desktop-first CSS (use mobile-first)
- Inline CSS or JavaScript
- JavaScript alerts (use toast notifications)
- Stub templates or "coming soon" pages

## CRITICAL - ALWAYS DO
- Server-side rendering (Go templates)
- Progressive enhancement (works without JS)
- Mobile-first responsive CSS
- CSS `word-break: break-all` for long strings (IPv6, .onion, tokens)
- Full admin panel with ALL settings
- WCAG 2.1 AA accessibility
- Touch targets minimum 44x44px
- /server/about content from IDEA.md
- /server/help content from IDEA.md (real endpoints, real examples)

## LONG STRINGS (REQUIRED CSS)
```css
.long-string, .ip-address, .onion-address, .api-token, .hash {
  word-break: break-all;
  overflow-wrap: break-word;
  font-family: monospace;
}
```

## BREAKPOINTS (mobile-first)
| Target | CSS |
|--------|-----|
| Mobile (base) | No media query |
| Tablet+ | `@media (min-width: 768px)` |
| Desktop+ | `@media (min-width: 1024px)` |

---
For complete details, see AI.md PART 16, 17
