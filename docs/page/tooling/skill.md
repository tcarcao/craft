# Claude Code Skill

The Craft skill for [Claude Code](https://claude.ai/code) teaches the AI assistant the full Craft grammar, modeling conventions, and event-driven choreography patterns — so it can generate, extend, explain, and validate `.craft` files without you having to describe the language each time.

## Installation

```bash
npx skills add tcarcao/craft
```

This installs the skill globally. To install at project level only, omit `-g` and run from your project directory.

## What it does

Once installed, Claude Code automatically activates the skill whenever you:

- Work with `.craft` files
- Ask it to model a domain, service, or use case
- Ask it to extend or explain an existing `.craft` file
- Mention the Craft DSL or DDD artifacts

The skill provides Claude with:

- The complete BNF grammar
- All construct types: actors, domains, services, architecture, exposures, use cases
- Event-driven choreography patterns (`notifies` / `listens`)
- Common mistakes and how to avoid them
- File conventions (kebab-case filenames, canonical ordering, `docs/` placement)
- Integration with `craft-cli` for validation and diagram generation

## Example session

```
You: Model a notification system with email and SMS delivery channels

Claude: [generates a valid .craft file with actors, domains, services,
         and event-driven use cases using notifies/listens choreography]
```

After Claude generates or edits a file, validate it:

```bash
craft-cli validate docs/notification-system.craft
```

## How it pairs with craft-cli

The skill and CLI complement each other:

| Task | Tool |
|------|------|
| Generate or extend `.craft` files | Claude Code skill |
| Validate syntax and lint rules | `craft-cli validate` |
| Produce diagrams | `craft-cli generate` |
| Inspect the parsed model | `craft-cli inspect` |

## Source

The skill lives in `.claude/skills/craft-dsl/` in this repository. It is versioned alongside the language — when the grammar evolves, the skill is updated in the same commit.
