// The hydra mod: every hook and every `$` call lives in this one file, because
// the engine follows `$` only into functions declared in the hooks module
// itself. The decisions are pure and live in logic.ts; this file asks hydra,
// reads files, and talks to the engine.
//
// Each feature degrades to doing nothing: hydra missing, slow, old or broken
// means the hook passes the event through untouched.

import { atom, read, update } from 'claude-code'
import type { EngineInterface, On, PluginOptions, Timer, ToolCallResult } from 'claude-code'

import type { HydraFailure } from '../types'
import {
  USER_ORIGINS,
  abilityBlock,
  candidatesBlock,
  commandTouchesSecrets,
  diffFailures,
  driftToast,
  failuresOf,
  holdReason,
  isRuleFile,
  isSecretPath,
  isUnder,
  matchTokenBound,
  missingLibrary,
  mentionsHydra,
  newestFirst,
  parseDollarAbility,
  parseJSON,
  remedyOf,
  ruleBlock,
  ruleModeOf,
  RULES_DELIVERY_SECTION,
  siblingHoldReason,
  splitLibraries,
  statusLine,
  stripFrontmatter,
  unknownAbilityBlock,
  worthMatching,
  type AbilityInfo,
  type AbilityMatch,
  type DoctorReport,
  type HomeSource,
  type RuleLibrary,
  type RuleMatch,
  type RuleMatchReport,
} from './logic'

type $ = EngineInterface

const LOADED = atom({ plugin: 'hydra', key: 'loaded' } as const, {} as Record<string, string[]>)
const HELD = atom({ plugin: 'hydra', key: 'held' } as const, {} as Record<string, Record<string, string>>)
const STEP = atom({ plugin: 'hydra', key: 'step' } as const, {} as Record<string, string>)
const FIRED = atom({ plugin: 'hydra', key: 'fired' } as const, [] as string[])
const FAILURES = atom({ plugin: 'hydra', key: 'failures' } as const, [] as HydraFailure[])
const DOCTOR_AT = atom({ plugin: 'hydra', key: 'doctorAt' } as const, -1)
const LAST_ABILITY = atom({ plugin: 'hydra', key: 'lastAbility' } as const, '')

// What the start probe found. Undefined until it lands, and for good when no
// hydra binary starts: every hook then passes through.
type Env = {
  binary: string
  version: string
  root: string
  home: string | undefined
  extras: string[]
  canMatch: boolean
  libraries: RuleLibrary[]
  globalHome: string | undefined
  homeSource: HomeSource
  // Set over the environment of every hydra process: HYDRA_HOME, when the
  // hydraHome option moves the global library.
  hydraEnv: Record<string, string> | undefined
}

type ToolKind = 'read' | 'write' | 'bash'

// How long each hydra call may take before the mod gives up on it and passes
// the event through. A prompt waits on ROUTE_MS and a tool call on MATCH_MS, so
// those two are short; the doctors run in the background.
const ROUTE_MS = 750
const MATCH_MS = 1500
const PROBE_MS = 2000
const COMMAND_MS = 3000
const DOCTOR_MS = 5000
// A burst of edits under a library runs the doctors once, this long after the last.
const DOCTOR_DEBOUNCE_MS = 1500
const DRIFT_TOAST_MS = 8000

function optionsOf(options: PluginOptions) {
  return {
    hydraPath: typeof options.hydraPath === 'string' && options.hydraPath.trim() ? options.hydraPath.trim() : 'hydra',
    hydraHome: typeof options.hydraHome === 'string' ? options.hydraHome.trim() : '',
    abilityRouting: options.abilityRouting !== false,
    ruleMode: ruleModeOf(options.ruleMode),
    ruleLibraries: typeof options.ruleLibraries === 'string' ? options.ruleLibraries : '',
    maxInjectChars: typeof options.maxInjectChars === 'number' && options.maxInjectChars > 0 ? options.maxInjectChars : 12000,
    doctorIntervalSec: typeof options.doctorIntervalSec === 'number' ? options.doctorIntervalSec : 300,
    trustProjectRules: options.trustProjectRules !== false,
  }
}

type Options = ReturnType<typeof optionsOf>

// Module state. register() resets all of it, and a hot reload runs register()
// again in a fresh environment, so none of it outlives a load. What must
// survive a reload (deliveries, holds, doctor verdicts) is in $.state instead.
let opt: Options = optionsOf({})
let env: Env | undefined
let abilityCache: AbilityInfo[] | undefined
const matchMemo = new Map<string, RuleMatch[]>()
let doctorTimer: Timer | undefined

// --- hydra --------------------------------------------------------------

// runHydra answers undefined for "no opinion": no binary, a spawn that
// failed or timed out. The exit code is the caller's to ignore — hydra
// prints its JSON report on a non-zero exit too.
async function runHydra($: $, args: string[], timeoutMs: number) {
  if (!env) return undefined
  try {
    return await $.process.run([env.binary, ...args], { cwd: env.root, env: env.hydraEnv, timeoutMs })
  } catch {
    return undefined
  }
}

function libraryFlags(): string[] {
  return (env?.extras ?? []).flatMap(dir => ['--library', dir])
}

async function probe($: $): Promise<void> {
  const root = await $.session.root()
  const home = await $.env.get('HOME')
  const envHydraHome = await $.env.get('HYDRA_HOME')
  const extras = splitLibraries(opt.ruleLibraries, home)
  // The global library resolves as hydra resolves it — the option, else
  // HYDRA_HOME, else ~/.hydra — by handing the option to hydra as HYDRA_HOME.
  // hydra itself then reports where it looked, so the mod never guesses.
  const hydraHome = opt.hydraHome ? splitLibraries(opt.hydraHome, home)[0] : undefined
  const hydraEnv = hydraHome ? { HYDRA_HOME: hydraHome } : undefined
  const homeSource: HomeSource = hydraHome ? 'hydraHome option' : envHydraHome?.trim() ? 'HYDRA_HOME' : 'default'
  const candidates = [opt.hydraPath, ...(home ? [`${home}/.local/bin/hydra`] : [])]

  for (const binary of candidates) {
    let version
    try {
      version = await $.process.run([binary, '--version'], { cwd: root, env: hydraEnv, timeoutMs: PROBE_MS })
    } catch {
      continue
    }
    if (version.exitCode !== 0) continue

    // One probe answers two questions: does this build have `match` (a
    // capability, not a version, so a local build works before a release),
    // and where are the libraries it reads.
    const probeArgs = ['match', '--command=hydra-mod-probe', ...extras.flatMap(d => ['--library', d]), '--json']
    const matched = await $.process.run([binary, ...probeArgs], { cwd: root, env: hydraEnv, timeoutMs: PROBE_MS }).catch(() => undefined)
    const report = matched && parseJSON<RuleMatchReport>(matched.stdout)
    const canMatch = Array.isArray(report?.libraries)
    env = {
      binary,
      version: version.stdout.trim(),
      root,
      home,
      extras,
      canMatch,
      libraries: report?.libraries ?? [],
      globalHome: report?.libraries.find(l => l.kind === 'global')?.dir.replace(/\/rules\/?$/, ''),
      homeSource,
      hydraEnv,
    }
    if (!env.globalHome) {
      const doctor = await runHydra($, ['doctor', '--global', '--json'], DOCTOR_MS)
      env.globalHome = doctor && parseJSON<DoctorReport>(doctor.stdout)?.home
    }
    if (!canMatch && opt.ruleMode !== 'off') {
      $.ui.log(`hydra: ${env.version} has no 'hydra match'; rule delivery is off until hydra is updated`)
    }
    return
  }
  $.ui.log(`hydra: no hydra binary starts (${candidates.join(', ')}); the mod passes everything through`)
}

async function abilities($: $): Promise<AbilityInfo[] | undefined> {
  if (abilityCache) return abilityCache
  const listed = await runHydra($, ['ability', 'list', '--json'], PROBE_MS)
  const parsed = listed && parseJSON<AbilityInfo[]>(listed.stdout)
  abilityCache = Array.isArray(parsed) ? parsed : undefined
  return abilityCache
}

// invalidate drops everything derived from the libraries, after one of them
// may have changed.
function invalidate(): void {
  abilityCache = undefined
  matchMemo.clear()
}

// --- status line ----------------------------------------------------------

async function refreshStatus($: $): Promise<void> {
  $.ui.status(statusLine(await read($, FIRED), await read($, FAILURES)))
}

// --- feature 1: ability routing --------------------------------------------

type Routed = { block: string; toast: string; ability: string }

async function route($: $, text: string): Promise<Routed | undefined> {
  const dollar = parseDollarAbility(text)
  if (dollar.kind === 'bare') return undefined // the router skill's picker owns it

  let phrase = text
  if (dollar.kind === 'named') phrase = dollar.name
  else if (!worthMatching(text, matchTokenBound(await abilities($)))) return undefined

  const ran = await runHydra($, ['ability', 'match', phrase, '--json'], ROUTE_MS)
  const report = ran && parseJSON<{ matches: AbilityMatch[] | null }>(ran.stdout)
  if (!report) return undefined
  let matches = report.matches ?? []

  if (dollar.kind === 'named') {
    matches = matches.filter(m => m.kind === 'name')
    if (matches.length === 0) {
      const known = (await abilities($))?.map(a => a.name) ?? []
      return {
        block: unknownAbilityBlock(dollar.name, known),
        toast: `hydra: no ability named ${dollar.name}`,
        ability: '',
      }
    }
  }

  const [only] = matches
  if (!only) return undefined
  if (matches.length === 1) {
    const body = await $.fs.read(only.path).then(stripFrontmatter, () => undefined)
    return { block: abilityBlock(only, body, opt.maxInjectChars), toast: `⚡ ability: ${only.ability}`, ability: only.ability }
  }
  const names = matches.map(m => m.ability)
  return { block: candidatesBlock(matches), toast: `⚡ abilities: ${names.join(' | ')}`, ability: names.join(' | ') }
}

// --- feature 2: rules ------------------------------------------------------

async function matchRules($: $, kind: ToolKind, subject: string): Promise<RuleMatch[]> {
  const key = `${kind === 'bash' ? 'c' : 'p'}:${subject}`
  const memo = matchMemo.get(key)
  if (memo) return memo
  const flag = kind === 'bash' ? `--command=${subject}` : `--path=${subject}`
  const ran = await runHydra($, ['match', flag, ...libraryFlags(), '--json'], MATCH_MS)
  const report = ran && parseJSON<RuleMatchReport>(ran.stdout)
  if (!report || !Array.isArray(report.matches)) return [] // no opinion: not memoized
  const matches = report.matches.filter(m => !m.always)
  matchMemo.set(key, matches)
  return matches
}

async function bodies($: $, rules: readonly RuleMatch[], subject: string): Promise<string[]> {
  return Promise.all(
    rules.map(async m => {
      const text = await $.fs.read(m.file).then(stripFrontmatter, () => undefined)
      return text === undefined
        ? ruleBlock(m, '', subject, -1) // unreadable: point at the file
        : ruleBlock(m, text, subject, opt.maxInjectChars)
    }),
  )
}

async function markLoaded($: $, loop: string, files: readonly string[]): Promise<void> {
  if (files.length === 0) return
  await update($, LOADED, all => {
    const had = all?.[loop] ?? []
    return { ...all, [loop]: [...had, ...files.filter(f => !had.includes(f))] }
  })
}

async function fire($: $, names: readonly string[]): Promise<void> {
  await update($, FIRED, list => newestFirst(list ?? [], names))
  await refreshStatus($)
}

async function withRules($: $, e: { agentId?: string }, next: () => Promise<ToolCallResult>, kind: ToolKind, subject: string): Promise<ToolCallResult> {
  const loop = e.agentId ?? 'main'
  if (kind === 'read' && env && isRuleFile(subject, env.libraries)) {
    // The model is reading a rule itself: it has the text, so it is delivered.
    await markLoaded($, loop, [subject])
  }

  const matches = await matchRules($, kind, subject)
  if (matches.length === 0) return next()
  await fire($, matches.map(m => m.rule))
  if (opt.ruleMode === 'status') return next()

  const loaded = (await read($, LOADED))[loop] ?? []
  const due = matches.filter(m => !loaded.includes(m.file) && (opt.trustProjectRules || m.library.kind !== 'project'))
  if (due.length === 0) return next()

  // The warden mod denies .env and ~/.secrets outright. Holding such a call
  // to deliver a rule would only buy a retry for warden to deny, so these are
  // never held: the rule rides the result instead, if there is one.
  const touchesSecrets = kind === 'bash' ? commandTouchesSecrets(subject) : isSecretPath(subject)
  if (opt.ruleMode === 'enforce' && kind !== 'read' && !touchesSecrets) {
    const token = (await read($, STEP))[loop]
    const held = (await read($, HELD))[loop] ?? {}
    // Held in an earlier step: the model has read the hold since, so it is delivered.
    const seen = due.filter(m => held[m.file] !== undefined && (token === undefined || held[m.file] !== token))
    const sibling = due.filter(m => token !== undefined && held[m.file] === token)
    const fresh = due.filter(m => held[m.file] === undefined)
    await markLoaded($, loop, seen.map(m => m.file))

    if (fresh.length > 0) {
      const blocks = await bodies($, fresh, subject)
      await update($, HELD, all => ({
        ...all,
        [loop]: { ...(all?.[loop] ?? {}), ...Object.fromEntries(fresh.map(m => [m.file, token ?? ''])) },
      }))
      return { deny: holdReason(blocks, fresh.map(m => m.rule), subject) }
    }
    if (sibling.length > 0) return { deny: siblingHoldReason(sibling.map(m => m.rule)) }
    return next()
  }

  const result = await next()
  if (result.deny !== undefined || result.isError) return result
  const blocks = await bodies($, due, subject)
  await markLoaded($, loop, due.map(m => m.file))
  return { ...result, context: [...(result.context ?? []), ...blocks] }
}

// touchesLibrary says whether a successful call may have changed a library
// the doctors check or the matcher reads.
function touchesLibrary(kind: ToolKind, subject: string): boolean {
  if (!env) return false
  if (kind === 'bash') return mentionsHydra(subject)
  if (kind !== 'write') return false
  const dirs = [`${env.root}/.hydra`, ...(env.globalHome ? [env.globalHome] : []), ...env.extras]
  return dirs.some(d => isUnder(subject, d))
}

async function onTool($: $, e: { agentId?: string }, next: () => Promise<ToolCallResult>, kind: ToolKind, subject: string | undefined): Promise<ToolCallResult> {
  if (!env || !subject) return next()
  const result = opt.ruleMode === 'off' || !env.canMatch ? await next() : await withRules($, e, next, kind, subject)
  if (touchesLibrary(kind, subject) && result.deny === undefined && !result.isError) {
    invalidate()
    scheduleDoctor($, DOCTOR_DEBOUNCE_MS)
  }
  return result
}

// --- feature 3: doctor -----------------------------------------------------

function scheduleDoctor($: $, ms: number): void {
  doctorTimer?.cancel()
  doctorTimer = $.clock.after(ms, () => {
    doctorTimer = undefined
    runDoctors($).catch(() => undefined)
  })
}

// runDoctors runs every doctor that applies here and acts on what changed.
// The project doctor runs only where a project library exists: elsewhere it
// says "run hydra init", which is wrong advice in a Laravel project (Boost
// owns rules there) or in the dotfiles repository.
async function runDoctors($: $): Promise<DoctorReport[] | undefined> {
  if (!env) return undefined
  const jobs: string[][] = []
  if (await $.fs.exists(`${env.root}/.hydra/rules`)) jobs.push(['doctor', '--json'])
  // A global library that is not there is reported as such, never handed to
  // the global doctors: their advice is "run hydra init", and after a relocate
  // that scaffolds an empty library at the old place.
  const libraryFound = env.globalHome !== undefined && (await $.fs.exists(env.globalHome))
  if (libraryFound && (await $.fs.exists(`${env.globalHome}/rules`))) jobs.push(['doctor', '--global', '--json'])
  if (libraryFound && (await $.fs.exists(`${env.globalHome}/abilities`))) jobs.push(['ability', 'doctor', '--json'])

  const ran = await Promise.all(jobs.map(args => runHydra($, args, DOCTOR_MS)))
  const reports = ran.map(r => r && parseJSON<DoctorReport>(r.stdout))
  // A doctor that did not answer says nothing about drift: leave the last
  // verdict standing rather than announce a recovery that did not happen.
  if (reports.some(r => r === undefined || !Array.isArray(r.checks))) return undefined
  const answered = reports as DoctorReport[]

  const failures = [...(libraryFound ? [] : [missingLibrary(env.globalHome, env.homeSource)]), ...failuresOf(answered)]
  const previous = (await read($, FAILURES)).map(f => f.key)
  const neverRan = (await read($, DOCTOR_AT)) < 0
  const change = diffFailures(neverRan ? undefined : previous, failures)

  await update($, FAILURES, () => failures)
  const now = await $.clock.now()
  await update($, DOCTOR_AT, () => now)
  if (change.appeared.length > 0) $.ui.toast(driftToast(change.appeared), { timeoutMs: DRIFT_TOAST_MS })
  else if (change.cleared) $.ui.toast('hydra: in sync')
  await refreshStatus($)
  return answered
}

// --- /hydra ------------------------------------------------------------------

async function summary($: $): Promise<string> {
  if (!env) return 'hydra: no hydra binary starts, so the mod is passing everything through.'
  const lines = [
    `${env.version} (${env.binary})`,
    `project root: ${env.root}`,
    `global library: ${env.globalHome ?? 'unknown'} (${env.homeSource === 'default' ? 'default: HYDRA_HOME is not set' : `from ${env.homeSource}`})`,
    `rule delivery: ${opt.ruleMode}${env.canMatch ? '' : " (off: this hydra has no 'match')"}`,
    `ability routing: ${opt.abilityRouting ? 'on' : 'off'}`,
  ]
  if (env.libraries.length) {
    lines.push('', 'Rule libraries')
    for (const l of env.libraries) lines.push(`  ${l.kind.padEnd(8)} ${l.dir}${l.present === false ? ' (not present)' : ''}`)
  }

  const failures = await read($, FAILURES)
  const at = await read($, DOCTOR_AT)
  lines.push('', at < 0 ? 'Doctor: not run yet' : failures.length ? 'Doctor' : 'Doctor: all checks pass')
  for (const f of failures) lines.push(`  ${f.severity === 'error' ? '✗' : '!'} ${f.scope}: ${f.name} — ${remedyOf(f)}`)

  const fired = await read($, FIRED)
  lines.push('', `Rules fired this turn: ${fired.length ? fired.join(', ') : 'none'}`)
  const loaded = await read($, LOADED)
  for (const [loop, files] of Object.entries(loaded)) {
    if (files.length) lines.push(`  delivered to ${loop}: ${files.map(f => f.slice(f.lastIndexOf('/') + 1, -3)).join(', ')}`)
  }
  const last = await read($, LAST_ABILITY)
  lines.push('', `Last ability routed: ${last || 'none'}`)
  return lines.join('\n')
}

async function matchCommand($: $, subject: string): Promise<string> {
  if (!env?.canMatch) return "This hydra has no 'match' command."
  if (!subject) return 'Usage: /hydra match <path-or-command>'
  const isPath = await $.fs.exists(subject)
  const flag = isPath ? `--path=${subject}` : `--command=${subject}`
  const ran = await runHydra($, ['match', flag, ...libraryFlags()], COMMAND_MS)
  return ran ? (ran.stdout || ran.stderr).trim() : 'hydra did not answer.'
}

export function register(on: On, options: PluginOptions): void {
  opt = optionsOf(options)
  env = undefined
  abilityCache = undefined
  matchMemo.clear()
  doctorTimer = undefined

  // --- hooks -----------------------------------------------------------------

  on('session.start', async ($, e, next) => {
    await probe($).catch(() => undefined)
    await $.command
      .register({ name: 'hydra', description: 'hydra status: doctor, rules fired, last ability', argumentHint: '[doctor | match <path-or-command>]' })
      .catch(() => undefined)
    // Never awaited: the first prompt does not wait on three doctors.
    if (env) scheduleDoctor($, 0)
    return next(e)
  })

  on('prompt.submit', async ($, e, next) => {
    await update($, FIRED, () => [])
    await refreshStatus($)
    if (!env) return next(e)

    const at = await read($, DOCTOR_AT)
    if (at >= 0 && (await $.clock.now()) - at > opt.doctorIntervalSec * 1000) scheduleDoctor($, 0)

    // An absent origin is the user's own prompt (PromptSubmitResult says so).
    const origin = e.origin?.kind ?? 'composer'
    if (!opt.abilityRouting || !USER_ORIGINS.includes(origin)) return next(e)
    const routed = await route($, e.text).catch(() => undefined)
    if (!routed) return next(e)
    await update($, LAST_ABILITY, () => routed.ability)
    const entered = await next({ ...e, context: [...(e.context ?? []), routed.block] })
    $.ui.toast(routed.toast)
    return entered
  })

  on('prompt.compose', async ($, e, next) => {
    const composed = await next(e)
    const delivers = (opt.ruleMode === 'inject' || opt.ruleMode === 'enforce') && env?.canMatch === true
    if (!delivers || e.traits.includes('bare')) return composed
    return { sections: [...composed.sections, { id: 'hydra:rules-delivery', text: RULES_DELIVERY_SECTION, scope: 'session' as const }] }
  })

  on('turn.step', async function* ($, e, next) {
    const loop = e.agentId ?? 'main'
    await update($, STEP, all => ({ ...all, [loop]: `${e.turnId}:${e.index}` }))
    return yield* next(e)
  })

  on('session.compact', async ($, e, next) => {
    const compacted = await next(e)
    // Delivered rule text may have been summarized away: deliver again.
    const loop = e.agentId ?? 'main'
    await update($, LOADED, all => ({ ...all, [loop]: [] }))
    await update($, HELD, all => ({ ...all, [loop]: {} }))
    return compacted
  })

  on('session.end', async ($, e, next) => {
    if (e.reason === 'clear') {
      await update($, LOADED, () => ({}))
      await update($, HELD, () => ({}))
      await update($, FIRED, () => [])
    }
    return next(e)
  })

  on('tool.call', { tool: 'Read' }, ($, e, next) => onTool($, e, () => next(e), 'read', e.file_path))
  on('tool.call', { tool: 'Edit' }, ($, e, next) => onTool($, e, () => next(e), 'write', e.file_path))
  on('tool.call', { tool: 'Write' }, ($, e, next) => onTool($, e, () => next(e), 'write', e.file_path))
  on('tool.call', { tool: 'NotebookEdit' }, ($, e, next) => onTool($, e, () => next(e), 'write', e.notebook_path))
  on('tool.call', { tool: 'Bash' }, ($, e, next) => onTool($, e, () => next(e), 'bash', e.command))

  on('command.run', { command: 'hydra' }, async ($, e) => {
    const args = e.args.trim()
    if (args === 'doctor') {
      await runDoctors($)
      return { text: await summary($) }
    }
    if (args === 'match' || args.startsWith('match ')) return { text: await matchCommand($, args.slice(5).trim()) }
    return { text: await summary($) }
  })
}
