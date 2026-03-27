# Layouts

The `sections` field in your config controls how many status lines are rendered and what appears on each one. Prism supports both single-line and multi-line layouts.

## Single-line Layout

Pass a flat array of section names to display everything on one line:

```json
{
  "sections": ["dir", "model", "context", "usage", "git"]
}
```

Sections are separated by dim separator characters and render left-to-right in the order listed.

## Multi-line Layout

Pass a nested array — each inner array becomes a separate status line:

```json
{
  "sections": [
    ["dir", "model", "context", "usage", "git"],
    ["supabase", "vercel"],
    ["spotify"]
  ]
}
```

Lines where every section returns an empty string are hidden entirely, so you can include optional plugin sections without worrying about blank lines appearing when those plugins are inactive.

## Section Ordering

Sections render left-to-right in the order they are listed. Place the most important or most frequently changing information first so it is immediately visible.

## Plugin Sections

Any registered plugin name can be used as a section name. Built-in sections (`dir`, `model`, `context`, `linesChanged`) are rendered directly by Prism. Plugin sections (`git`, `spotify`, `usage`, and so on) are executed via the plugin system and run in parallel.

## Tips

- Put frequently-changing information (context, usage) on the first line where it is always visible.
- Put ambient or secondary information (spotify, system metrics) on lower lines to keep the primary line focused.
- Plugins that return empty strings are silently hidden — you do not need to conditionally include them.
- Lines where all sections return empty are removed automatically, keeping the output clean.
