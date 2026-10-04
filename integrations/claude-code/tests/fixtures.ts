// A fake world beneath the hydra mod: a hydra binary answering by argv, a
// filesystem, and the UI sinks, every call recorded for the test to read.

import type { On } from 'claude-code'
import { mock } from 'claude-code/testing'

import type { AbilityInfo, AbilityMatch, DoctorReport, RuleMatch } from '../hooks/logic'

export const ROOT = '/repo'
export const HOME = '/home/u'
export const GLOBAL = `${HOME}/.hydra`
export const DOTFILES = `${HOME}/AI/dotfiles/rules`

export type World = {
  // What the fake hydra knows. Tests change these between calls.
  abilities: AbilityInfo[]
  abilityMatches: Record<string, AbilityMatch[]>
  rules: { match: RuleMatch; paths?: string[]; commands?: string[] }[]
  doctors: { project?: DoctorReport; global?: DoctorReport; ability?: DoctorReport }
  hasMatch: boolean
  hydraMissing: boolean
  // HYDRA_HOME as the session's environment has it.
  envHydraHome: string | undefined
  files: Record<string, string>
  // What the mod did.
  runs: string[][]
  // HYDRA_HOME each hydra process was given, '' when none.
  runEnv: string[]
  toasts: string[]
  statuses: (string | undefined)[]
  logs: string[]
  clock: ReturnType<typeof mock.clock>
}

// The hydra commands that change something on disk.
const WRITING = ['init', 'sync', 'add', 'new', 'relocate', 'self-update']

export function writes(args: readonly string[]): boolean {
  const verb = args[0] === 'ability' ? args[1] : args[0]
  return verb !== undefined && WRITING.includes(verb)
}

const ok = (stdout: string, exitCode = 0) => ({
  value: { exitCode, stdout, stderr: '', isStdoutTruncated: false, isStderrTruncated: false },
})

export function healthy(scope: string, home: string): DoctorReport {
  return { scope, home, ok: true, initialized: true, checks: [{ name: 'index.md is current', ok: true, severity: 'warning' }] }
}

export function stale(scope: string, home: string, fix: string[]): DoctorReport {
  return {
    scope,
    home,
    ok: true,
    initialized: true,
    checks: [{ name: 'index.md is current', ok: false, severity: 'warning', detail: `run '${fix.join(' ')}'`, fix }],
  }
}

export function rule(name: string, kind: 'project' | 'global' | 'extra', pattern: string, subject: string, matchKind: 'path' | 'command' = 'path'): RuleMatch {
  const dir = kind === 'project' ? `${ROOT}/.hydra/rules` : kind === 'global' ? `${GLOBAL}/rules` : DOTFILES
  return {
    rule: name,
    title: name,
    file: `${dir}/${name}.md`,
    library: { kind, dir, present: true },
    always: false,
    matched: [{ kind: matchKind, pattern, subject }],
  }
}

export function worldOf(on: On, start: Partial<World> = {}): World {
  const w: World = {
    abilities: [],
    abilityMatches: {},
    rules: [],
    doctors: { global: healthy('global', GLOBAL), ability: healthy('global abilities', GLOBAL) },
    hasMatch: true,
    hydraMissing: false,
    envHydraHome: undefined,
    files: {},
    runs: [],
    runEnv: [],
    toasts: [],
    statuses: [],
    logs: [],
    clock: mock.clock(on),
    ...start,
  }
  w.doctors = { global: healthy('global', GLOBAL), ability: healthy('global abilities', GLOBAL), ...start.doctors }

  on('session.start', ($, e) => ({ cwd: e.cwd }))
  on('session.root', () => ({ value: ROOT }))
  on('env.get', ($, e) => ({ value: e.name === 'HOME' ? HOME : e.name === 'HYDRA_HOME' ? w.envHydraHome : undefined }))
  on('command.register', ($, e) => ({ value: { command: e.name } }))
  on('ui.toast', ($, e) => {
    w.toasts.push(e.text)
    return { value: undefined }
  })
  on('ui.status', ($, e) => {
    w.statuses.push(e.text)
    return { value: undefined }
  })
  on('ui.log', ($, e) => {
    w.logs.push(e.text)
    return { value: undefined }
  })
  on('fs.read', ($, e) => {
    const text = w.files[e.path]
    if (text === undefined) throw new Error(`ENOENT: ${e.path}`)
    return { value: text }
  })
  on('fs.exists', ($, e) => ({
    value: Object.keys(w.files).some(f => f === e.path || f.startsWith(e.path + '/')),
  }))

  on('process.run', ($, e) => {
    const [binary, ...args] = e.argv
    if (w.hydraMissing || binary !== 'hydra') throw new Error(`cannot start ${binary}`)
    w.runs.push(args)
    // The mod only ever reads: it never syncs, initializes, adds, relocates or
    // updates anything. A writing command here fails the test that caused it.
    if (writes(args)) throw new Error(`the mod ran a writing hydra command: hydra ${args.join(' ')}`)
    w.runEnv.push(e.init?.env?.HYDRA_HOME ?? '')
    // hydra resolves its global library the way the real one does.
    const library = e.init?.env?.HYDRA_HOME ?? w.envHydraHome ?? GLOBAL
    const line = args.join(' ')
    if (line === '--version') return ok('hydra 0.3.0-test\n')

    if (args[0] === 'match') {
      if (!w.hasMatch) return ok('', 1)
      const libraries = [
        { kind: 'project', dir: `${ROOT}/.hydra/rules`, present: true },
        { kind: 'global', dir: `${library}/rules`, present: Object.keys(w.files).some(f => f.startsWith(`${library}/rules/`)) },
        ...args.flatMap((a, i) => (a === '--library' ? [{ kind: 'extra', dir: args[i + 1] ?? '', present: true }] : [])),
      ]
      const path = args.find(a => a.startsWith('--path='))?.slice(7)
      const command = args.find(a => a.startsWith('--command='))?.slice(10)
      const matches = w.rules
        .filter(r => (path !== undefined && r.paths?.includes(path)) || (command !== undefined && r.commands?.some(c => command.includes(c))))
        .map(r => r.match)
      if (!args.includes('--json')) return ok(matches.length ? `${matches.length} rule(s) match\n` : 'No rule matches.\n', matches.length ? 0 : 1)
      return ok(JSON.stringify({ libraries, matches, errors: [] }), matches.length ? 0 : 1)
    }
    if (line === 'ability list --json') return ok(JSON.stringify(w.abilities))
    if (args[0] === 'ability' && args[1] === 'match') {
      const phrase = args[2] ?? ''
      // hydra compares words, not punctuation or case.
      const words = (phrase.toLowerCase().match(/[a-z0-9]+/g) ?? []).join(' ')
      const matches = w.abilityMatches[words] ?? w.abilityMatches[words.replace(/ /g, '-')] ?? null
      return ok(JSON.stringify({ phrase, matches }), matches ? 0 : 1)
    }
    if (line === 'doctor --json' && w.doctors.project) return ok(JSON.stringify(w.doctors.project))
    if (line === 'doctor --global --json' && w.doctors.global) return ok(JSON.stringify(w.doctors.global))
    if (line === 'ability doctor --json' && w.doctors.ability) return ok(JSON.stringify(w.doctors.ability))
    return ok('', 1)
  })

  // The libraries the doctors look for exist unless a test removes them.
  w.files[`${GLOBAL}/rules/index.md`] ??= '# index\n'
  w.files[`${GLOBAL}/abilities/index.md`] ??= '# index\n'
  return w
}

// The bottom of the tool chain: the tool ran and succeeded.
export function toolsSucceed(on: On): void {
  on('tool.call', () => ({ result: { ok: true } as never, text: 'done' }))
}
