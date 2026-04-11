---
name: regex-vs-llm-structured-text
description: Decision framework for choosing between regex and LLM when parsing structured text — start with regex, add LLM only for low-confidence edge cases.
---

# Regex vs LLM for Structured Text

## Decision Rule

```
Is the text format consistent and repeating?
├── Yes (>90% follows a pattern) → Regex first
│   ├── Regex handles 95%+ → Done, no LLM needed
│   └── Regex handles <95% → Add LLM for edge cases only
└── No (free-form, highly variable) → LLM directly
```

## Pipeline Pattern

1. **Regex parser** — extract structure (handles 95-98% of cases)
2. **Confidence scorer** — flag extractions with missing fields, short text, few matches
3. **LLM validator** — fix only low-confidence items (cheapest model sufficient)

## Key Principles

- **Always start with regex** — even imperfect regex gives a baseline. Cheaper, faster, deterministic.
- **Confidence threshold** — score each extraction, only send items below threshold to LLM (~2-5% typically)
- **Cheapest LLM for validation** — Haiku-class models suffice for "is this extraction correct?"
- **Immutable data** — return new objects from cleaning/validation, never mutate parsed items
- **Measure** — track regex success rate and LLM call count to know if the pipeline is healthy

## When NOT to Use Regex

- Free-form text with no repeating structure
- Text where meaning depends on context, not position
- Highly variable formatting with many edge cases (>10% failure rate)

## Applies To

Importing CSV/Excel data, parsing structured emails, processing uploaded documents, extracting data from HTML/PDF, any repeating-pattern text where cost matters.
