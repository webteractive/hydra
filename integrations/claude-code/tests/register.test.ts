import type { On, PromptSubmitInput, TurnStepResult } from 'claude-code'
import { describe, expect, test } from 'claude-code/testing'
import type { Engine } from 'claude-code/testing'

import { DOTFILES, GLOBAL, HOME, ROOT, healthy, rule, stale, toolsSucceed, worldOf, writes } from './fixtures'

const SESSION = { cwd: ROOT, surface: 'terminal' as const, isInteractive: true }

// A prompt typed at the terminal.
const typed = (text: string): PromptSubmitInput => ({ text, wait: false, origin: { kind: 'composer' } })

const PREPARE = {
  ability: 'prepare-for-production',
  kind: 'trigger' as const,
  trigger: 'primetime',
  description: 'Harden a change for production.',
  path: `${GLOBAL}/abilities/prepare-for-production/ABILITY.md`,
}

describe('ability routing', () => {
  test('a trigger match attaches the ability as hidden context and toasts', async ($, on) => {
    const w = worldOf(on, {
      abilities: [{ name: 'prepare-for-production', description: 'x', triggers: ['primetime'], path: PREPARE.path }],
      abilityMatches: { primetime: [PREPARE] },
      files: { [PREPARE.path]: '---\nname: prepare-for-production\n---\n\n# Prepare\n\nStep one.\n' },
    })
    let seen: { text: string; context?: readonly string[] } | undefined
    on('prompt.submit', ($, e) => {
      seen = e
      return { text: e.text, context: e.context }
    })

    await $.session.start(SESSION)
    await $.prompt.submit(typed('Primetime!'))

    expect(seen?.text, 'the user message is untouched').toBe('Primetime!')
    expect(seen?.context?.length).toBe(1)
    const block = seen?.context?.[0] ?? ''
    expect(block).toContain('<hydra-ability name="prepare-for-production" matched-by="trigger: primetime"')
    expect(block).toContain('Step one.')
    expect(block).not.toContain('name: prepare-for-production')
    expect(w.toasts).toContain('⚡ ability: prepare-for-production')
    expect(w.runs.some(r => r[0] === 'ability' && r[1] === 'match' && r[2] === 'Primetime!')).toBe(true)
  })

  test('a long or multi-line prompt never spawns a match', async ($, on) => {
    const w = worldOf(on, { abilities: [{ name: 'explain-code', description: 'x', triggers: ['explain this'], path: '/a' }] })
    on('prompt.submit', ($, e) => ({ text: e.text, context: e.context }))

    await $.session.start(SESSION)
    await $.prompt.submit(typed('please refactor the whole billing module so that invoices are generated lazily and cached'))
    await $.prompt.submit(typed('explain this\nand that'))

    expect(w.runs.filter(r => r[1] === 'match')).toEqual([])
  })

  test('several candidates are listed without bodies', async ($, on) => {
    const other = { ...PREPARE, ability: 'ship-it', path: `${GLOBAL}/abilities/ship-it/ABILITY.md` }
    worldOf(on, { abilityMatches: { primetime: [PREPARE, other] } })
    let context: readonly string[] | undefined
    on('prompt.submit', ($, e) => {
      context = e.context
      return { text: e.text, context: e.context }
    })

    await $.session.start(SESSION)
    await $.prompt.submit(typed('primetime'))

    expect(context?.[0]).toContain('<hydra-abilities>')
    expect(context?.[0]).toContain('- ship-it')
    expect(context?.[0]).not.toContain('Step one')
  })

  test('$ability <name> routes by exact name and keeps the task text', async ($, on) => {
    const named = { ...PREPARE, kind: 'name' as const, trigger: undefined }
    const w = worldOf(on, {
      abilityMatches: { 'prepare-for-production': [named] },
      files: { [PREPARE.path]: '# Prepare\n' },
    })
    let seen: { text: string; context?: readonly string[] } | undefined
    on('prompt.submit', ($, e) => {
      seen = e
      return { text: e.text, context: e.context }
    })

    await $.session.start(SESSION)
    await $.prompt.submit(typed('$ability prepare-for-production for the billing change'))

    expect(seen?.text).toBe('$ability prepare-for-production for the billing change')
    expect(seen?.context?.[0]).toContain('matched-by="exact ability name"')
    expect(w.runs).toContainEqual(['ability', 'match', 'prepare-for-production', '--json'])
  })

  test('$ability with an unknown name toasts and lists the names', async ($, on) => {
    const w = worldOf(on, { abilities: [{ name: 'explain-code', description: 'x', triggers: [], path: '/a' }] })
    let context: readonly string[] | undefined
    on('prompt.submit', ($, e) => {
      context = e.context
      return { text: e.text, context: e.context }
    })

    await $.session.start(SESSION)
    await $.prompt.submit(typed('$ability nope'))

    expect(w.toasts).toContain('hydra: no ability named nope')
    expect(context?.[0]).toContain('- explain-code')
  })

  test('a bare $ability is left to the router skill', async ($, on) => {
    const w = worldOf(on)
    let context: readonly string[] | undefined
    on('prompt.submit', ($, e) => {
      context = e.context
      return { text: e.text, context: e.context }
    })

    await $.session.start(SESSION)
    await $.prompt.submit(typed('$ability'))

    expect(context).toBeUndefined()
    expect(w.runs.filter(r => r[1] === 'match')).toEqual([])
  })

  test('with hydra missing every prompt passes through', async ($, on) => {
    const w = worldOf(on, { hydraMissing: true, abilityMatches: { primetime: [PREPARE] } })
    let context: readonly string[] | undefined
    on('prompt.submit', ($, e) => {
      context = e.context
      return { text: e.text, context: e.context }
    })

    await $.session.start(SESSION)
    await $.prompt.submit(typed('primetime'))

    expect(context).toBeUndefined()
    expect(w.toasts).toEqual([])
    expect(w.logs.some(l => l.includes('no hydra binary starts'))).toBe(true)
  })

  test('routing can be turned off', { options: { abilityRouting: false } }, async ($, on) => {
    const w = worldOf(on, { abilityMatches: { primetime: [PREPARE] } })
    on('prompt.submit', ($, e) => ({ text: e.text, context: e.context }))

    await $.session.start(SESSION)
    await $.prompt.submit(typed('primetime'))

    expect(w.runs.filter(r => r[1] === 'match')).toEqual([])
  })
})

const EDIT = (file_path: string) => ({ tool: 'Edit' as const, file_path, old_string: 'a', new_string: 'b' })
const READ = (file_path: string) => ({ tool: 'Read' as const, file_path })
const BASH = (command: string) => ({ tool: 'Bash' as const, command })

function cliWorld(on: On) {
  const cli = rule('cli-version', 'extra', '**/main.go', `${ROOT}/main.go`)
  const releases = rule('releases', 'global', 'gh release', 'gh release create v1', 'command')
  const w = worldOf(on, {
    rules: [
      { match: cli, paths: [`${ROOT}/main.go`, `${ROOT}/cmd/main.go`] },
      { match: releases, commands: ['gh release'] },
    ],
    files: {
      [cli.file]: '---\ntitle: Every CLI reports its own version\n---\n\n- Ship --version.\n',
      [releases.file]: '# Releases\n\nInstall after release.\n',
    },
  })
  toolsSucceed(on)
  return { w, cli, releases }
}

describe('rules', () => {
  test('status mode names the rule and never attaches or holds', { options: { ruleMode: 'status' } }, async ($, on) => {
    const { w } = cliWorld(on)
    await $.session.start(SESSION)

    const ran = await $.tool.call(EDIT(`${ROOT}/main.go`))

    expect(ran.deny).toBeUndefined()
    expect(ran.context).toBeUndefined()
    expect(w.statuses.at(-1)).toBe('rule matched: cli-version')
  })

  test('inject mode attaches the body to the first result only', { options: { ruleMode: 'inject' } }, async ($, on) => {
    cliWorld(on)
    await $.session.start(SESSION)

    const first = await $.tool.call(READ(`${ROOT}/main.go`))
    expect(first.context?.[0]).toContain('<hydra-rule name="cli-version"')
    expect(first.context?.[0]).toContain('- Ship --version.')
    expect(first.context?.[0]).not.toContain('title: Every CLI')

    const again = await $.tool.call(EDIT(`${ROOT}/cmd/main.go`))
    expect(again.deny).toBeUndefined()
    expect(again.context, 'a rule is delivered once per loop').toBeUndefined()
  })

  test('enforce holds the first edit once, with the rule as the reason', async ($, on) => {
    const { w } = cliWorld(on)
    await $.session.start(SESSION)

    const held = await $.tool.call(EDIT(`${ROOT}/main.go`))
    expect(held.deny).toContain('<hydra-rule name="cli-version"')
    expect(held.deny).toContain('held once, not refused')
    expect(w.toasts, 'a hold is status-only').toEqual([])

    const retried = await $.tool.call(EDIT(`${ROOT}/main.go`))
    expect(retried.deny).toBeUndefined()
    expect(retried.context).toBeUndefined()
  })

  test('enforce holds a command a rule covers', async ($, on) => {
    cliWorld(on)
    await $.session.start(SESSION)

    const held = await $.tool.call(BASH('gh release create v1'))
    expect(held.deny).toContain('<hydra-rule name="releases"')
    const retried = await $.tool.call(BASH('gh release create v1'))
    expect(retried.deny).toBeUndefined()
  })

  test('enforce attaches on a read instead of holding it', async ($, on) => {
    cliWorld(on)
    await $.session.start(SESSION)

    const read = await $.tool.call(READ(`${ROOT}/main.go`))
    expect(read.deny).toBeUndefined()
    expect(read.context?.[0]).toContain('<hydra-rule name="cli-version"')

    const edit = await $.tool.call(EDIT(`${ROOT}/main.go`))
    expect(edit.deny, 'already delivered by the read').toBeUndefined()
  })

  test('a model reading the rule file itself counts as delivered', { options: { ruleLibraries: DOTFILES } }, async ($, on) => {
    const { cli } = cliWorld(on)
    await $.session.start(SESSION)

    await $.tool.call(READ(cli.file))
    const edit = await $.tool.call(EDIT(`${ROOT}/main.go`))
    expect(edit.deny).toBeUndefined()
  })

  test('enforce never holds a .env or .secrets call: warden denies those', async ($, on) => {
    const warden = rule('warden', 'extra', '**/.env*', `${ROOT}/.env`)
    worldOf(on, {
      rules: [
        { match: warden, paths: [`${ROOT}/.env`, `${HOME}/.secrets`], commands: ['cat .env'] },
      ],
      files: { [warden.file]: '- Use warden.\n' },
    })
    toolsSucceed(on)
    await $.session.start(SESSION)

    expect((await $.tool.call(EDIT(`${ROOT}/.env`))).deny).toBeUndefined()
    expect((await $.tool.call(BASH('cat .env'))).deny).toBeUndefined()
  })

  test('a subagent loop gets its own delivery', async ($, on) => {
    cliWorld(on)
    await $.session.start(SESSION)

    await $.tool.call(READ(`${ROOT}/main.go`))
    const fromSub = await $.tool.call({ ...EDIT(`${ROOT}/main.go`), agentId: 'sub-1' } as never)
    expect((fromSub as { deny?: string }).deny).toContain('cli-version')
  })

  test('untrusted project rules are named but never delivered', { options: { trustProjectRules: false } }, async ($, on) => {
    const local = rule('queue', 'project', 'app/Jobs/**', `${ROOT}/app/Jobs/A.php`)
    const w = worldOf(on, { rules: [{ match: local, paths: [`${ROOT}/app/Jobs/A.php`] }], files: { [local.file]: 'x' } })
    toolsSucceed(on)
    await $.session.start(SESSION)

    const edit = await $.tool.call(EDIT(`${ROOT}/app/Jobs/A.php`))
    expect(edit.deny).toBeUndefined()
    expect(w.statuses.at(-1)).toBe('rule matched: queue')
  })

  test('a hydra without match turns rule delivery off', async ($, on) => {
    const { w } = cliWorld(on)
    w.hasMatch = false
    await $.session.start(SESSION)

    expect((await $.tool.call(EDIT(`${ROOT}/main.go`))).deny).toBeUndefined()
    expect(w.logs.some(l => l.includes("no 'hydra match'"))).toBe(true)
  })

  test('the fired list resets on each prompt', async ($, on) => {
    const { w } = cliWorld(on)
    on('prompt.submit', ($, e) => ({ text: e.text, context: e.context }))
    await $.session.start(SESSION)

    await $.tool.call(READ(`${ROOT}/main.go`))
    expect(w.statuses.at(-1)).toBe('rule matched: cli-version')
    await $.prompt.submit(typed('next thing'))
    expect(w.statuses.at(-1)).toBeUndefined()
  })

  test('extra libraries are passed to hydra match', { options: { ruleLibraries: '~/AI/dotfiles/rules' } }, async ($, on) => {
    const { w } = cliWorld(on)
    await $.session.start(SESSION)
    await $.tool.call(READ(`${ROOT}/main.go`))

    const match = w.runs.find(r => r[0] === 'match' && r[1] === `--path=${ROOT}/main.go`)
    expect(match).toEqual(['match', `--path=${ROOT}/main.go`, '--library', DOTFILES, '--json'])
  })
})

describe('doctor', () => {
  test('drift at session start toasts once with the scoped fix', async ($, on) => {
    const w = worldOf(on, { doctors: { global: stale('global', GLOBAL, ['hydra', 'sync', '--global']), ability: healthy('global abilities', GLOBAL) } })
    await $.session.start(SESSION)
    await w.clock.settle()

    expect(w.toasts).toEqual(['hydra: global: index.md is current — run hydra sync --global'])
    expect(w.statuses.at(-1)).toBe('! run hydra sync --global')
    expect(w.runs.some(r => r.join(' ') === 'doctor --json'), 'no project library, no project doctor').toBe(false)
  })

  test('an old hydra without fix argv still gets --global', async ($, on) => {
    const report = stale('global', GLOBAL, [])
    delete report.checks[0]!.fix
    report.checks[0]!.detail = "run 'hydra sync'"
    const w = worldOf(on, { doctors: { global: report } })
    await $.session.start(SESSION)
    await w.clock.settle()

    expect(w.toasts[0]).toContain('run hydra sync --global')
  })

  test('a repeated failure stays quiet and a recovery says so', async ($, on) => {
    const w = worldOf(on, { doctors: { global: stale('global', GLOBAL, ['hydra', 'sync', '--global']) } })
    toolsSucceed(on)
    await $.session.start(SESSION)
    await w.clock.settle()
    expect(w.toasts.length).toBe(1)

    // An edit under the global library schedules another run.
    await $.tool.call(EDIT(`${GLOBAL}/rules/releases.md`))
    await w.clock.advance(1500)
    await w.clock.settle()
    expect(w.toasts.length, 'same failure, no new toast').toBe(1)

    w.doctors.global = healthy('global', GLOBAL)
    await $.tool.call(BASH('hydra sync --global'))
    await w.clock.advance(1500)
    await w.clock.settle()
    expect(w.toasts.at(-1)).toBe('hydra: in sync')
    expect(w.statuses.at(-1)).toBeUndefined()
  })

  test('the project doctor runs where a project library exists', async ($, on) => {
    const w = worldOf(on, {
      files: { [`${ROOT}/.hydra/rules/index.md`]: '#' },
      doctors: { project: stale('project', `${ROOT}/.hydra`, ['hydra', 'sync']), global: healthy('global', GLOBAL) },
    })
    await $.session.start(SESSION)
    await w.clock.settle()

    expect(w.runs.some(r => r.join(' ') === 'doctor --json')).toBe(true)
    expect(w.toasts[0]).toContain('run hydra sync')
  })

  test('/hydra doctor reports the checks and their fixes', async ($, on) => {
    const w = worldOf(on, { doctors: { global: stale('global', GLOBAL, ['hydra', 'sync', '--global']) } })
    await $.session.start(SESSION)
    await w.clock.settle()

    const out = await $.command.run({ command: 'hydra', args: 'doctor' } as never)
    expect(out.text).toContain('global: index.md is current — run hydra sync --global')
    expect(out.text).toContain('hydra 0.3.0-test')
  })
})

const COMPOSE = { model: 'm', promptModel: 'm', surfaces: [], tools: [], outputStyle: null, traits: [] }

describe('system prompt', () => {
  test('rule delivery adds one static section', async ($, on) => {
    worldOf(on)
    on('prompt.compose', () => ({ sections: [{ id: 'intro', text: 'hi', scope: 'shared' as const }] }))
    await $.session.start(SESSION)
    const composed = await $.prompt.compose(COMPOSE)
    expect(composed.sections.map(s => s.id)).toEqual(['intro', 'hydra:rules-delivery'])
  })

  test('status mode adds nothing', { options: { ruleMode: 'status' } }, async ($, on) => {
    worldOf(on)
    on('prompt.compose', () => ({ sections: [{ id: 'intro', text: 'hi', scope: 'shared' as const }] }))
    await $.session.start(SESSION)
    const composed = await $.prompt.compose(COMPOSE)
    expect(composed.sections.map(s => s.id)).toEqual(['intro'])
  })
})

describe('holds across steps', () => {
  async function step($: Engine, index: number, agentId?: string) {
    for await (const _ of $.turn.step({ turnId: 't1', index, model: 'm', messageCount: 1, ...(agentId ? { agentId } : {}) })) {
      // drain
    }
  }

  test('parallel calls in the held step are held too, then pass in the next step', async ($, on) => {
    cliWorld(on)
    on('turn.step', async function* (_$, e) {
      const done: TurnStepResult = { turnId: e.turnId, index: e.index, answer: '', toolUses: [], stopReason: 'tool_use', usage: null }
      return done
    })
    await $.session.start(SESSION)

    await step($, 0)
    const first = await $.tool.call(EDIT(`${ROOT}/main.go`))
    const sibling = await $.tool.call(EDIT(`${ROOT}/cmd/main.go`))
    expect(first.deny).toContain('<hydra-rule name="cli-version"')
    expect(sibling.deny).toContain('its text is on a sibling')
    expect(sibling.deny).not.toContain('<hydra-rule')

    await step($, 1)
    expect((await $.tool.call(EDIT(`${ROOT}/main.go`))).deny).toBeUndefined()
    expect((await $.tool.call(EDIT(`${ROOT}/cmd/main.go`))).deny).toBeUndefined()
  })

  test('compaction delivers rules again', async ($, on) => {
    cliWorld(on)
    on('session.compact', () => ({ skip: 'test' }))
    await $.session.start(SESSION)

    await $.tool.call(READ(`${ROOT}/main.go`))
    await $.session.compact({ trigger: 'manual', messages: [{ role: 'user', text: 'earlier', toolUses: [] }] })
    const again = await $.tool.call(READ(`${ROOT}/main.go`))
    expect(again.context?.[0]).toContain('<hydra-rule name="cli-version"')
  })
})

describe('doctor staleness', () => {
  test('a prompt re-runs a doctor older than the interval', { options: { doctorIntervalSec: 60 } }, async ($, on) => {
    const w = worldOf(on)
    on('prompt.submit', ($, e) => ({ text: e.text, context: e.context }))
    await $.session.start(SESSION)
    await w.clock.settle()
    const runs = () => w.runs.filter(r => r.join(' ') === 'doctor --global --json').length
    expect(runs()).toBe(1)

    await $.prompt.submit(typed('soon'))
    await w.clock.settle()
    expect(runs(), 'fresh: no rerun').toBe(1)

    await w.clock.advance(61_000)
    await $.prompt.submit(typed('later'))
    await w.clock.settle()
    expect(runs()).toBe(2)
  })
})

describe('the global library', () => {
  test('a missing library is reported, and its doctors are not run', async ($, on) => {
    const w = worldOf(on)
    delete w.files[`${GLOBAL}/rules/index.md`]
    delete w.files[`${GLOBAL}/abilities/index.md`]
    await $.session.start(SESSION)
    await w.clock.settle()

    expect(w.toasts).toEqual([`hydra: global: global library not found at ${GLOBAL} — set HYDRA_HOME to your hydra library`])
    expect(w.statuses.at(-1)).toBe('! set HYDRA_HOME to your hydra library')
    expect(w.runs.some(r => r.includes('--global') && r[0] === 'doctor'), 'its advice would be hydra init').toBe(false)
    expect(w.runs.some(r => r[0] === 'ability' && r[1] === 'doctor')).toBe(false)
  })

  test('HYDRA_HOME is followed, and a missing one is named as such', async ($, on) => {
    const w = worldOf(on, { envHydraHome: '/home/u/AI/dotfiles/hydra' })
    await $.session.start(SESSION)
    await w.clock.settle()

    expect(w.toasts[0]).toContain('global library not found at /home/u/AI/dotfiles/hydra')
    expect(w.toasts[0]).toContain('point HYDRA_HOME at your hydra library')
    const out = await $.command.run({ command: 'hydra', args: '' } as never)
    expect(out.text).toContain('global library: /home/u/AI/dotfiles/hydra (from HYDRA_HOME)')
  })

  test('the hydraHome option is handed to every hydra process', { options: { hydraHome: '~/AI/dotfiles/hydra' } }, async ($, on) => {
    const lib = `${HOME}/AI/dotfiles/hydra`
    const w = worldOf(on, { files: { [`${lib}/rules/index.md`]: '#', [`${lib}/abilities/index.md`]: '#' } })
    await $.session.start(SESSION)
    await w.clock.settle()

    expect(w.runEnv.length).toBeGreaterThan(0)
    expect(w.runEnv.every(v => v === lib)).toBe(true)
    expect(w.toasts).toEqual([])
    const out = await $.command.run({ command: 'hydra', args: '' } as never)
    expect(out.text).toContain(`global library: ${lib} (from hydraHome option)`)
  })
})

describe('read-only', () => {
  test('a whole session runs only read-only hydra commands', { options: { ruleLibraries: DOTFILES } }, async ($, on) => {
    const { w } = cliWorld(on)
    w.abilityMatches = { primetime: [PREPARE] }
    w.files[PREPARE.path] = '# Prepare\n'
    w.doctors.global = stale('global', GLOBAL, ['hydra', 'sync', '--global'])
    on('prompt.submit', ($, e) => ({ text: e.text, context: e.context }))
    await $.session.start(SESSION)
    await w.clock.settle()

    await $.prompt.submit(typed('primetime'))
    await $.tool.call(READ(`${ROOT}/main.go`))
    await $.tool.call(EDIT(`${GLOBAL}/rules/releases.md`))
    await $.tool.call(BASH('hydra sync --global'))
    await w.clock.advance(1500)
    await w.clock.settle()
    await $.command.run({ command: 'hydra', args: 'doctor' } as never)
    await $.command.run({ command: 'hydra', args: `match ${ROOT}/main.go` } as never)

    expect(w.runs.length).toBeGreaterThan(5)
    expect(w.runs.filter(writes)).toEqual([])
    expect([...new Set(w.runs.map(r => (r[0] === 'ability' ? `ability ${r[1]}` : r[0])))].sort()).toEqual([
      '--version',
      'ability doctor',
      'ability list',
      'ability match',
      'doctor',
      'match',
    ])
  })
})
