// Pure helpers for the hydra mod. Nothing here touches `$` or `on`: the engine
// follows `$` only into functions declared in the hooks module itself, so
// everything that calls the engine lives in register.ts and this file stays
// plain data in, data out — which is also what makes it easy to test.

export type RuleMode = 'off' | 'status' | 'inject' | 'enforce'

export const RULE_MODES: readonly RuleMode[] = ['off', 'status', 'inject', 'enforce']

export function ruleModeOf(value: unknown): RuleMode {
  return RULE_MODES.includes(value as RuleMode) ? (value as RuleMode) : 'enforce'
}

// ---------------------------------------------------------------------------
// hydra's JSON, as far as the mod reads it

export type AbilityMatch = {
  ability: string
  kind: 'name' | 'trigger'
  trigger?: string
  description?: string
  path: string
}

export type AbilityInfo = {
  name: string
  description: string
  triggers?: string[] | null
  path: string
}

export type RuleLibrary = { kind: string; dir: string; present?: boolean }

export type RuleMatch = {
  rule: string
  title: string
  file: string
  library: RuleLibrary
  always: boolean
  matched: { kind: 'path' | 'command'; pattern: string; subject: string }[]
}

export type RuleMatchReport = {
  libraries: RuleLibrary[]
  matches: RuleMatch[]
  errors?: { file: string; error: string }[]
}

export type DoctorCheck = {
  name: string
  ok: boolean
  severity: 'error' | 'warning'
  detail?: string
  fix?: string[]
}

export type DoctorReport = {
  scope: string
  home: string
  ok: boolean
  initialized?: boolean
  checks: DoctorCheck[]
}

// parseJSON reads a hydra report. hydra prints JSON on a non-zero exit too
// (match exits 1 when nothing matched), so the exit code is never consulted:
// a report that parses is an answer, anything else is "no opinion".
export function parseJSON<T>(stdout: string): T | undefined {
  try {
    const value = JSON.parse(stdout) as unknown
    return value !== null && typeof value === 'object' ? (value as T) : undefined
  } catch {
    return undefined
  }
}

// ---------------------------------------------------------------------------
// Feature 1: ability routing

// A user typed it, through some client of theirs. Notifications, schedules,
// peers and channels never route: a notification that happens to read
// "ship it" must not start a workflow.
export const USER_ORIGINS: readonly string[] = ['composer', 'bridge', 'sdk']

export type DollarAbility =
  | { kind: 'none' }
  | { kind: 'bare' }
  | { kind: 'named'; name: string; rest: string }

// parseDollarAbility recognizes `$ability [name] [task...]`. A bare `$ability`
// is left to the router skill, which owns the picker.
export function parseDollarAbility(text: string): DollarAbility {
  const m = /^\s*\$ability(?:\s+([A-Za-z0-9][A-Za-z0-9-]*))?(?:\s+([\s\S]*))?\s*$/.exec(text)
  if (!m) return { kind: 'none' }
  if (!m[1]) return m[2]?.trim() ? { kind: 'none' } : { kind: 'bare' }
  return { kind: 'named', name: m[1].toLowerCase(), rest: (m[2] ?? '').trim() }
}

export function tokenCount(text: string): number {
  return (text.toLowerCase().match(/[\p{L}\p{N}]+/gu) ?? []).length
}

// FILLER_TOKENS is generous headroom for the politeness hydra strips
// ("could you go ahead and ... for me please"). The bound only decides whether
// asking hydra is worth a process; hydra still decides every prompt it sees.
const FILLER_TOKENS = 8

// matchTokenBound is the most tokens a prompt can have and still be an exact
// name or trigger invocation. Undefined when the catalog is unknown.
export function matchTokenBound(abilities: readonly AbilityInfo[] | undefined): number | undefined {
  if (!abilities) return undefined
  let longest = 0
  for (const a of abilities) {
    longest = Math.max(longest, tokenCount(a.name.replace(/-/g, ' ')))
    for (const t of a.triggers ?? []) longest = Math.max(longest, tokenCount(t))
  }
  return longest + FILLER_TOKENS
}

// worthMatching is the cheap gate in front of `hydra ability match`. A trigger
// fires only when it is the whole request, so a multi-line prompt, or one with
// more words than any trigger plus filler, can never match.
export function worthMatching(text: string, bound: number | undefined): boolean {
  const trimmed = text.trim()
  if (trimmed === '' || trimmed.includes('\n') || trimmed.length > 300) return false
  return bound === undefined || tokenCount(trimmed) <= bound
}

export function stripFrontmatter(markdown: string): string {
  return markdown.replace(/^---\r?\n[\s\S]*?\r?\n---\r?\n?/, '').replace(/^\s*\n/, '')
}

function attr(value: string): string {
  return value.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;')
}

function matchedBy(m: AbilityMatch): string {
  return m.kind === 'name' ? 'exact ability name' : `trigger: ${m.trigger ?? ''}`
}

// abilityBlock is what the model reads beside a prompt hydra matched to one
// ability. The body is inlined so the model cannot skip reading it; the path
// stays so relative references still resolve.
export function abilityBlock(m: AbilityMatch, body: string | undefined, maxChars: number): string {
  const head =
    `<hydra-ability name="${attr(m.ability)}" matched-by="${attr(matchedBy(m))}" path="${attr(m.path)}">\n` +
    'hydra matched this request to the ability above deterministically: treat it as an explicit invocation.\n'
  if (body === undefined || body.length > maxChars) {
    return (
      head +
      'Read that ABILITY.md completely before acting, and resolve its relative references from its directory.\n' +
      '</hydra-ability>'
    )
  }
  return (
    head +
    'The complete ABILITY.md follows and counts as read: do not open it again. Resolve its relative\n' +
    "references from that file's directory.\n\n" +
    body.trimEnd() +
    '\n</hydra-ability>'
  )
}

// candidatesBlock lists overlapping trigger matches. hydra cannot see the
// conversation that settles which one was meant, so the model picks.
export function candidatesBlock(matches: readonly AbilityMatch[]): string {
  const rows = matches.map(m => `- ${m.ability} (${matchedBy(m)}): ${m.description ?? ''}\n  ${m.path}`)
  return (
    '<hydra-abilities>\n' +
    'hydra matched this request to several abilities by trigger. Pick the one whose description fits what\n' +
    'the user is doing, say which you picked, and read its ABILITY.md completely before acting.\n\n' +
    rows.join('\n') +
    '\n</hydra-abilities>'
  )
}

export function unknownAbilityBlock(name: string, known: readonly string[]): string {
  return (
    `<hydra-abilities>\nThe user asked for $ability ${name}, but no ability has that name. ` +
    'Tell them so and list the exact names available:\n' +
    (known.length ? known.map(n => `- ${n}`).join('\n') : '(none installed)') +
    '\n</hydra-abilities>'
  )
}

// ---------------------------------------------------------------------------
// Feature 2: rules

// isSecretPath names subjects the warden mod denies outright (.env files and
// ~/.secrets). Holding such a call to deliver a rule only to watch warden deny
// the retry costs a round trip for nothing, so these are never held; the rule
// still shows on the status line and rides a result that is not denied.
export function isSecretPath(path: string): boolean {
  return /(^|\/)\.env(\.[^/]*)?$|(^|\/)\.secrets$/.test(path)
}

export function commandTouchesSecrets(command: string): boolean {
  return /(^|[\s'"=/<>])\.env(\.[\w.-]*)?($|[\s'";|&)>])|(^|[\s'"=/<>])\.secrets($|[\s'";|&)>])/.test(command)
}

// mentionsHydra spots a hydra invocation in a shell command (sync, add, new,
// relocate ...), after which the library may have changed.
export function mentionsHydra(command: string): boolean {
  return /(^|[^A-Za-z0-9_-])hydra($|[^A-Za-z0-9_-])/.test(command)
}

export function ruleBlock(m: RuleMatch, body: string, subject: string, maxChars: number): string {
  const how = m.matched.map(h => `${h.kind} "${h.pattern}"`).join(', ')
  const head =
    `<hydra-rule name="${attr(m.rule)}" title="${attr(m.title)}" source="${attr(`${m.library.kind}:${m.library.dir}`)}" path="${attr(m.file)}">\n` +
    `This rule applies to ${subject} (matched by ${how}).\n`
  if (body.length > maxChars) {
    return head + 'It is too long to inline here: read the file above completely before going further.\n</hydra-rule>'
  }
  return head + 'Its complete text follows and counts as read: do not open the file again.\n\n' + body.trimEnd() + '\n</hydra-rule>'
}

export function holdReason(blocks: readonly string[], names: readonly string[], subject: string): string {
  const which = names.length === 1 ? `rule ${names[0]} applies` : `rules ${names.join(', ')} apply`
  return (
    blocks.join('\n\n') +
    `\n\nThis call was held once, not refused: ${which} to ${subject} and had not been read in this session. ` +
    'The rule text above now counts as read. Re-issue the call, adjusted if the rule requires it.'
  )
}

export function siblingHoldReason(names: readonly string[]): string {
  return (
    `Held once: ${names.join(', ')} ${names.length === 1 ? 'applies' : 'apply'} here, and its text is on a sibling ` +
    'call\'s result in this same step. Read it there, then re-issue this call.'
  )
}

// The one system-prompt note the mod adds. It is static text, so it never
// costs the prompt cache more than once.
export const RULES_DELIVERY_SECTION =
  'hydra mod: path and command rule matches are detected automatically. When a <hydra-rule> block ' +
  "appears (as a tool result's note, or as the reason a call was held), it is that rule's complete text " +
  'and counts as read. You still judge trigger/situation matches yourself, and still grep the rules ' +
  'directories as instructed.'

// libraryFileOf says whether a path is a rule file in one of the libraries, so
// a model reading the rule itself marks it delivered.
export function isRuleFile(path: string, libraries: readonly RuleLibrary[]): boolean {
  if (!path.endsWith('.md') || /\/(index|README)\.md$/.test(path)) return false
  const dir = path.slice(0, path.lastIndexOf('/'))
  return libraries.some(l => l.dir.replace(/\/+$/, '') === dir)
}

export function splitLibraries(value: string, home: string | undefined): string[] {
  return value
    .split(':')
    .map(s => s.trim())
    .filter(Boolean)
    .map(s => (s === '~' || s.startsWith('~/')) && home ? home + s.slice(1) : s)
}

export function isUnder(path: string, dir: string): boolean {
  const d = dir.replace(/\/+$/, '')
  return d !== '' && (path === d || path.startsWith(d + '/'))
}

// ---------------------------------------------------------------------------
// Feature 3: doctor

// fix is the hydra command that clears a failure. hint replaces it when the
// remedy is not a command (a library that is not where hydra looks).
export type Failure = { key: string; scope: string; name: string; severity: 'error' | 'warning'; fix: string; hint?: string }

export function remedyOf(f: Failure): string {
  return f.hint ?? `run ${f.fix}`
}

// Where the global library came from, as the mod resolved it.
export type HomeSource = 'hydraHome option' | 'HYDRA_HOME' | 'default'

// missingLibrary is the failure for a global library that is not there. It is
// reported, never "fixed" with hydra init: after a relocate, an init in a shell
// that lost HYDRA_HOME would scaffold an empty library at the old place.
export function missingLibrary(home: string | undefined, source: HomeSource): Failure {
  const where = home ?? '~/.hydra'
  const hint =
    source === 'default'
      ? 'set HYDRA_HOME to your hydra library'
      : source === 'HYDRA_HOME'
        ? `point HYDRA_HOME at your hydra library (${where} does not exist)`
        : `point the hydraHome option at your hydra library (${where} does not exist)`
  return { key: 'global|library present', scope: 'global', name: `global library not found at ${where}`, severity: 'warning', fix: '', hint }
}

// remedyFor reads the remedy off a check. hydra 0.3 states a command as argv;
// older builds only say it in prose, and their global report's prose omits
// --global, which is corrected here so the command shown is the one that works.
// A check whose remedy is no command — set HYDRA_HOME, because the managed
// block names another library — is passed on as a hint, word for word.
export function remedyFor(scope: string, check: DoctorCheck): { fix: string; hint?: string } {
  if (check.fix?.length) return { fix: check.fix.join(' ') }
  // Older hydra's prose opens with the command, or puts it after an em dash
  // ("gemini support was removed — run '...'"); a run '...' anywhere else is an
  // alternative inside advice, not the fix.
  const m = /^(?:.*— )?run '([^']+)'/.exec(check.detail ?? '')
  if (!m?.[1]) return { fix: '', hint: check.detail || 'see hydra doctor' }
  let cmd = m[1]
  if (scope === 'global' && /^hydra (sync|init)$/.test(cmd)) cmd += ' --global'
  return { fix: cmd }
}

export function failuresOf(reports: readonly DoctorReport[]): Failure[] {
  const out: Failure[] = []
  for (const r of reports) {
    for (const c of r.checks) {
      if (c.ok) continue
      out.push({ key: `${r.scope}|${c.name}`, scope: r.scope, name: c.name, severity: c.severity, ...remedyFor(r.scope, c) })
    }
  }
  return out
}

export type DoctorChange = { appeared: Failure[]; cleared: boolean }

// diffFailures acts only on change: a failure already reported stays quiet.
export function diffFailures(previous: readonly string[] | undefined, current: readonly Failure[]): DoctorChange {
  const before = new Set(previous ?? [])
  return {
    appeared: current.filter(f => !before.has(f.key)),
    cleared: before.size > 0 && current.length === 0,
  }
}

export function uniqueRemedies(failures: readonly Failure[]): string[] {
  return [...new Set(failures.map(remedyOf))]
}

export function driftToast(appeared: readonly Failure[]): string {
  const first = appeared[0]
  if (!first) return ''
  const what = appeared.length === 1 ? `${first.scope}: ${first.name}` : `${appeared.length} checks failing`
  return `hydra: ${what} — ${uniqueRemedies(appeared).join('; ')}`
}

// ---------------------------------------------------------------------------
// The one status line both features share

export function statusLine(fired: readonly string[], failures: readonly Failure[]): string | undefined {
  const parts: string[] = []
  if (fired.length) {
    const shown = fired.slice(0, 3).join(', ')
    parts.push(`${fired.length === 1 ? 'rule' : 'rules'} matched: ${shown}${fired.length > 3 ? ` +${fired.length - 3}` : ''}`)
  }
  if (failures.some(f => f.severity === 'error')) parts.push('✗ hydra doctor')
  else if (failures.length) parts.push(`! ${uniqueRemedies(failures).join('; ')}`)
  // Claude Code already labels the band with the plugin name, so no prefix of our own.
  return parts.length ? parts.join(' · ') : undefined
}

// newestFirst adds names to a fired list without duplicates, newest first.
export function newestFirst(list: readonly string[], names: readonly string[]): string[] {
  const out = [...names]
  for (const n of list) if (!out.includes(n)) out.push(n)
  return out
}
