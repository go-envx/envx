---
name: markdown-editor
description: Use when writing, editing, or reviewing Markdown files, especially when reflowing prose or preserving Markdown structure.
---

## Best Practices

Apply these best practices when creating or modifying Markdown files:

- Keep each prose paragraph and list item on one physical line unless a hard line break is intentional. Do not create phantom line breaks: soft-wrap prose only when the document's conventions explicitly require it.

  ```md
  <!-- BAD: This is an example of a phantom line break -->
  This paragraph is split across
  multiple physical lines without an intentional
  hard line break.

  <!-- GOOD: This is an example of a properly formatted paragraph without phantom line breaks -->
  This paragraph stays on one physical line unless an intentional hard line break is needed even though it is somewhat long and extends beyond 100 characters.
  ```

- Preserve blank lines between block elements so headings, lists, tables, quotes, and code fences render correctly.

- Use headings in a logical hierarchy and keep their text concise and descriptive.

- Use fenced code blocks with a language identifier when the content is code.

- Use tables for genuinely tabular data and keep each row structurally complete.

- Write link text that describes its destination and keep URLs out of prose when a meaningful label works.

- Avoid trailing whitespace, including the two spaces that create an accidental hard line break.
