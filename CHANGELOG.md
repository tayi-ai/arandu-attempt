# Changelog

Everything worth knowing about a release of Arandu Attempt is recorded here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

A published module version is immutable: Go serves it from the proxy forever, so
a release is corrected by another release and never by moving a tag.

## [Unreleased]

### Added

- `answer`: extracts the final answer of a math solution (`####`, `\boxed{}`, last
  number), parses integers, decimals and fractions to exact rationals, and
  verifies a response against a reference as `math.numeric_exact_rational.v1`.
  It refuses rather than guesses: percent, scientific notation, radicals and
  ambiguous separators are errors.
- `comparison`: paired transitions of a candidate against its base, VGR, RR
  and NVG as exact fractions, exact two-sided McNemar and sign tests, and
  seeded paired bootstrap intervals.

### Changed

- **Breaking.** `Attempt` embeds the non-generic `model.Model`, and its table is
  declared once beside it with `model.NewTable`. `Attempts` takes a `model.DB`
  and returns the generated `*AttemptQuery`; `Get` returns `AttemptCollection`
  and `New` replaces `NewInstance(nil, false)`. The fields and methods
  `model.Model[Attempt]` promoted onto `Attempt` are gone, and `Exists` is a
  method. `UPGRADE.md` names every symbol.
- Requires Hesape `v0.48.0` and Framework `v0.50.2`; `arandu.mod.toml` declares
  `framework = ">= 0.50"`. The store route reads `name` from the body of the
  `POST` only, as Hesape now reads every `POST`.
- Routes, the migration `20260823_0001_create_attempts`, the actions
  `AttemptView`, `AttemptList`, `AttemptCreate`, `AttemptUpdate` and
  `AttemptDelete`, policy decisions and tenant scoping are unchanged.
