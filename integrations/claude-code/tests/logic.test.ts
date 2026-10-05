import { describe, expect, test } from 'claude-code/testing'

import {
  abilityBlock,
  commandTouchesSecrets,
  diffFailures,
  failuresOf,
  remedyFor,
  isRuleFile,
  isSecretPath,
  isUnder,
  matchTokenBound,
  missingLibrary,
  remedyOf,
  mentionsHydra,
  newestFirst,
  parseDollarAbility,
  parseJSON,
  ruleModeOf,
  splitLibraries,
  statusLine,
  stripFrontmatter,
  worthMatching,
  type Failure,
} from '../hooks/logic'

describe('ability routing helpers', () => {
  test('$ability parses a name, a task, or nothing', () => {
    expect(parseDollarAbility('$ability explain-code')).toEqual({ kind: 'named', name: 'explain-code', rest: '' })
    expect(parseDollarAbility('  $ability Explain-Code walk me through auth.go')).toEqual({
      kind: 'named',
      name: 'explain-code',
      rest: 'walk me through auth.go',
    })
    expect(parseDollarAbility('$ability')).toEqual({ kind: 'bare' })
    expect(parseDollarAbility('$ability   ')).toEqual({ kind: 'bare' })
    expect(parseDollarAbility('use $ability explain-code')).toEqual({ kind: 'none' })
    expect(parseDollarAbility('$abilityx')).toEqual({ kind: 'none' })
  })

  test('only a prompt short enough to be a whole trigger is worth a spawn', () => {
    const bound = matchTokenBound([
      { name: 'prepare-for-production', description: '', triggers: ['make it production ready'], path: '' },
    ])
    expect(bound).toBe(4 + 8)
    expect(worthMatching('Primetime!', bound)).toBe(true)
    expect(worthMatching('could you make it production ready for me please', bound)).toBe(true)
    expect(worthMatching('a b c d e f g h i j k l m', bound)).toBe(false)
    expect(worthMatching('two\nlines', bound)).toBe(false)
    expect(worthMatching('   ', bound)).toBe(false)
    expect(worthMatching('anything short', undefined)).toBe(true)
  })

  test('an oversized body is delivered as a pointer', () => {
    const m = { ability: 'a', kind: 'trigger' as const, trigger: 't', path: '/x/ABILITY.md' }
    expect(abilityBlock(m, 'short', 100)).toContain('counts as read')
    expect(abilityBlock(m, 'x'.repeat(101), 100)).toContain('Read that ABILITY.md completely')
    expect(abilityBlock(m, undefined, 100)).toContain('Read that ABILITY.md completely')
  })

  test('frontmatter is stripped, the body kept', () => {
    expect(stripFrontmatter('---\nname: a\n---\n\n# A\nbody\n')).toBe('# A\nbody\n')
    expect(stripFrontmatter('# No frontmatter\n')).toBe('# No frontmatter\n')
  })

  test('a report that does not parse is no opinion', () => {
    expect(parseJSON('{"a":1}')).toEqual({ a: 1 })
    expect(parseJSON('Error: unknown command "match"')).toBeUndefined()
    expect(parseJSON('null')).toBeUndefined()
  })
})

describe('rule helpers', () => {
  test('secret paths are the ones warden denies', () => {
    for (const p of ['/r/.env', '/r/.env.testing', '/r/sub/.env.local', '/home/u/.secrets']) {
      expect(isSecretPath(p), p).toBe(true)
    }
    for (const p of ['/r/.environment/x', '/r/env.ts', '/r/.envrc.md/x', '/r/.secrets.bak']) {
      expect(isSecretPath(p), p).toBe(false)
    }
  })

  test('a command touching a secret file is never held', () => {
    expect(commandTouchesSecrets('cat .env')).toBe(true)
    expect(commandTouchesSecrets('grep KEY ./.env.local')).toBe(true)
    expect(commandTouchesSecrets('source ~/.secrets && x')).toBe(true)
    expect(commandTouchesSecrets('echo "x" > .env')).toBe(true)
    expect(commandTouchesSecrets('npm run env')).toBe(false)
    expect(commandTouchesSecrets('cat .environment')).toBe(false)
  })

  test('a hydra command marks the library as possibly changed', () => {
    expect(mentionsHydra('hydra sync --global')).toBe(true)
    expect(mentionsHydra('cd x && ~/.local/bin/hydra add --glob a')).toBe(true)
    expect(mentionsHydra('ls hydra-old')).toBe(false)
    expect(mentionsHydra('echo hydras')).toBe(false)
  })

  test('a rule file is an .md directly in a library, not its index or README', () => {
    const libs = [{ kind: 'extra', dir: '/d/rules' }]
    expect(isRuleFile('/d/rules/warden.md', libs)).toBe(true)
    expect(isRuleFile('/d/rules/index.md', libs)).toBe(false)
    expect(isRuleFile('/d/rules/README.md', libs)).toBe(false)
    expect(isRuleFile('/d/rules/sub/x.md', libs)).toBe(false)
    expect(isRuleFile('/d/other/x.md', libs)).toBe(false)
  })

  test('library lists expand ~ and drop blanks', () => {
    expect(splitLibraries('~/AI/dotfiles/rules: /abs/rules ::', '/home/u')).toEqual(['/home/u/AI/dotfiles/rules', '/abs/rules'])
    expect(splitLibraries('', '/home/u')).toEqual([])
  })

  test('under means inside, not a sibling with the same prefix', () => {
    expect(isUnder('/h/.hydra/rules/a.md', '/h/.hydra')).toBe(true)
    expect(isUnder('/h/.hydra-old/a.md', '/h/.hydra')).toBe(false)
  })

  test('an unknown mode falls back to enforce', () => {
    expect(ruleModeOf('status')).toBe('status')
    expect(ruleModeOf('loud')).toBe('enforce')
    expect(ruleModeOf(undefined)).toBe('enforce')
  })

  test('fired names stay unique, newest first', () => {
    expect(newestFirst(['a', 'b'], ['c', 'a'])).toEqual(['c', 'a', 'b'])
  })
})

describe('doctor helpers', () => {
  const failing = (scope: string, fix?: string[], detail?: string) => ({
    scope,
    home: '/h',
    ok: true,
    checks: [{ name: 'index.md is current', ok: false, severity: 'warning' as const, fix, detail }],
  })

  test('the fix comes from argv when hydra states it', () => {
    expect(failuresOf([failing('global', ['hydra', 'sync', '--global'])])[0]?.fix).toBe('hydra sync --global')
  })

  test("an older hydra's global prose is corrected to --global", () => {
    const check = { name: 'x', ok: false, severity: 'warning' as const, detail: "run 'hydra sync'" }
    expect(remedyFor('global', check)).toEqual({ fix: 'hydra sync --global' })
    expect(remedyFor('project', check)).toEqual({ fix: 'hydra sync' })
    expect(remedyFor('global abilities', { ...check, detail: "run 'hydra ability sync'" })).toEqual({ fix: 'hydra ability sync' })
  })

  test('advice that is no command is shown as written', () => {
    const detail = "it names /lib: set HYDRA_HOME=/lib, or run 'hydra sync --global --force' to point it at /h/.hydra"
    const conflict = { name: 'block names this library in /h/.claude/CLAUDE.md', ok: false, severity: 'error' as const, detail }
    // The run '...' inside is the --force alternative, not the fix: no argv means the advice stands as written.
    const [f] = failuresOf([{ scope: 'global', home: '/h/.hydra', ok: false, checks: [conflict] }])
    expect(f?.severity).toBe('error')
    expect(remedyOf(f!)).toContain('set HYDRA_HOME=/lib')
  })

  test('only a change is news', () => {
    const now = failuresOf([failing('global', ['hydra', 'sync', '--global'])])
    expect(diffFailures(undefined, now).appeared.length).toBe(1)
    expect(diffFailures(['global|index.md is current'], now).appeared).toEqual([])
    expect(diffFailures(['global|index.md is current'], [])).toEqual({ appeared: [], cleared: true })
    expect(diffFailures([], [])).toEqual({ appeared: [], cleared: false })
  })

  test('a missing library says what to point where, never hydra init', () => {
    expect(missingLibrary(undefined, 'default').hint).toBe('set HYDRA_HOME to your hydra library')
    expect(missingLibrary('/x', 'HYDRA_HOME').hint).toBe('point HYDRA_HOME at your hydra library (/x does not exist)')
    expect(missingLibrary('/x', 'hydraHome option').hint).toContain('hydraHome option')
    expect(missingLibrary('/x', 'default').name).toBe('global library not found at /x')
  })

  test('the status line composes rules and drift, and clears when quiet', () => {
    const warn: Failure = { key: 'k', scope: 'global', name: 'n', severity: 'warning', fix: 'hydra sync --global' }
    expect(statusLine([], [])).toBeUndefined()
    expect(statusLine(['a', 'b', 'c', 'd'], [])).toBe('rules matched: a, b, c +1')
    expect(statusLine(['a'], [warn])).toBe('rule matched: a · ! run hydra sync --global')
    expect(statusLine([], [warn, { ...warn, severity: 'error' }])).toBe('✗ hydra doctor')
  })
})
