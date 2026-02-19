---
name: skill-creator
description: Guide for creating effective Agent Skills packages with clear metadata, trigger rules, and actionable implementation instructions.
---
# Skill Creator

## Purpose
Create or improve skills that are easy for models to discover, understand, and execute safely.

## Use When
- A new capability should be packaged as a reusable skill.
- An existing skill is unclear, too broad, or low quality.

## Skill Quality Checklist
1. Single responsibility: one skill, one outcome.
2. Clear trigger conditions: when to use and when not to use.
3. Minimal required inputs.
4. Deterministic workflow with concrete steps.
5. Explicit output contract.
6. Guardrails/safety constraints.
7. References for authoritative behavior.

## Recommended Structure
- Frontmatter:
  - `name`: kebab-case skill id matching folder name
  - `description`: one-sentence capability statement
- Body:
  - Purpose
  - Use When
  - Inputs
  - Procedure
  - Output Contract
  - Guardrails
  - Examples (optional)

## Authoring Rules
- Keep instructions imperative and testable.
- Avoid ambiguous language and broad advice-only text.
- Prefer short sections and concrete field names.
- Document failure/blocked paths.

## Validation
- `name` must be lowercase kebab-case.
- Folder name must match `name`.
- `description` must be non-empty.
- Frontmatter and markdown body must both exist.

## Improvement Pass
When editing a skill:
1. Tighten scope.
2. Clarify procedure order.
3. Add missing guardrails.
4. Remove redundant prose.
5. Ensure examples match real tools.
