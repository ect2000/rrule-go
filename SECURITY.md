# Security policy

## Supported versions

Fixes go into the latest v1.8.x release. Older releases do not get backports.

## What to report privately

Report these privately, not in a public issue:

- a panic, hang or runaway CPU or memory use on a rule that `NewRRule`, `StrToRRule` or `StrToRRuleSet`
  accepts;
- anything else that lets untrusted rule text harm a program that uses this library.

Wrong occurrences without a crash or hang are ordinary bugs. Open a normal issue for those.

## How to report

Use GitHub's private reporting:
[Report a vulnerability](https://github.com/kentonium3/rrule-go/security/advisories/new).

Include the rule, the DTSTART and its time zone, the library and Go versions, and what happened.

I aim to reply within a week. Once a fix is released, I publish an advisory and credit you unless you
ask me not to.
