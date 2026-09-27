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

